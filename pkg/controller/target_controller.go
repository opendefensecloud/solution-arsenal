// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	ociname "github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	targetFinalizer = "solar.opendefense.cloud/target-finalizer"

	ConditionTypeRegistryResolved = "RegistryResolved"
	ConditionTypeReleasesRendered = "ReleasesRendered"
	ConditionTypeBootstrapReady   = "BootstrapReady"
)

var ErrReleaseNotRenderedYet = errors.New("release is not rendered yet")

type releaseInfo struct {
	name     string
	release  *solarv1alpha1.Release
	cv       *solarv1alpha1.ComponentVersion
	rtName   string
	chartURL string
}

type TargetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registries,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releasebindings,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile collects ReleaseBindings, resolves the render registry, creates per-release
// RenderTasks (with dedup), and creates a per-target bootstrap RenderTask.
func (r *TargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("Target is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	// Fetch target
	target := &solarv1alpha1.Target{}
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get object")
	}

	// Handle deletion
	if !target.DeletionTimestamp.IsZero() {
		log.V(1).Info("Target is being deleted")
		r.Recorder.Eventf(target, nil, corev1.EventTypeWarning, "Deleting", "Reconcile", "Target is being deleted, cleaning up RenderTasks")

		// Delete owned RenderTasks
		if err := r.deleteOwnedRenderTasks(ctx, target); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete owned RenderTasks")
		}

		// Remove finalizer
		if slices.Contains(target.Finalizers, targetFinalizer) {
			latest := &solarv1alpha1.Target{}
			if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest Target for finalizer removal")
			}

			original := latest.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool {
				return s == targetFinalizer
			})
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to remove finalizer from Target")
			}
		}

		return ctrl.Result{}, nil
	}

	// Set finalizer if not set
	if !slices.Contains(target.Finalizers, targetFinalizer) {
		latest := &solarv1alpha1.Target{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest Target for finalizer addition")
		}

		original := latest.DeepCopy()
		latest.Finalizers = append(latest.Finalizers, targetFinalizer)
		if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to add finalizer to Target")
		}

		return ctrl.Result{}, nil
	}

	// Resolve render registry
	registry := &solarv1alpha1.Registry{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      target.Spec.RenderRegistryRef.Name,
		Namespace: target.Namespace,
	}, registry); err != nil {
		if apierrors.IsNotFound(err) {
			r.setCondition(ctx, target, ConditionTypeRegistryResolved, metav1.ConditionFalse, "NotFound",
				"Registry not found: "+target.Spec.RenderRegistryRef.Name)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Registry")
	}

	if registry.Spec.SolarSecretRef == nil {
		r.setCondition(ctx, target, ConditionTypeRegistryResolved, metav1.ConditionFalse, "MissingSolarSecretRef",
			"Registry does not have SolarSecretRef set, required for rendering")

		return ctrl.Result{}, nil
	}

	r.setCondition(ctx, target, ConditionTypeRegistryResolved, metav1.ConditionTrue, "Resolved",
		"Registry resolved: "+registry.Name)

	// Collect ReleaseBindings for this target
	bindingList := &solarv1alpha1.ReleaseBindingList{}
	if err := r.List(ctx, bindingList,
		client.InNamespace(target.Namespace),
		client.MatchingFields{indexReleaseBindingTargetName: target.Name},
	); err != nil {
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to list ReleaseBindings")
	}

	if len(bindingList.Items) == 0 {
		log.V(1).Info("No ReleaseBindings found for target")
		r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "NoBindings",
			"No ReleaseBindings found for this target")

		return ctrl.Result{}, nil
	}

	// For each bound release, ensure a per-release RenderTask exists
	var releases []releaseInfo

	for _, binding := range bindingList.Items {
		rel := &solarv1alpha1.Release{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      binding.Spec.ReleaseRef.Name,
			Namespace: target.Namespace,
		}, rel); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("Release not found", "release", binding.Spec.ReleaseRef.Name)

				continue
			}

			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Release")
		}

		cv := &solarv1alpha1.ComponentVersion{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      rel.Spec.ComponentVersionRef.Name,
			Namespace: target.Namespace,
		}, cv); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("ComponentVersion not found", "cv", rel.Spec.ComponentVersionRef.Name)

				continue
			}

			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get ComponentVersion")
		}

		rtName := releaseRenderTaskName(rel.Name, registry.Name)
		releases = append(releases, releaseInfo{
			name:    rel.Name,
			release: rel,
			cv:      cv,
			rtName:  rtName,
		})
	}

	// Create per-release RenderTasks (dedup: if another target shares same registry, reuses same RenderTask)
	allRendered := true

	for i, ri := range releases {
		rt := &solarv1alpha1.RenderTask{}
		err := r.Get(ctx, client.ObjectKey{Name: ri.rtName, Namespace: target.Namespace}, rt)

		if apierrors.IsNotFound(err) {
			spec := r.computeReleaseRenderTaskSpec(ri.release, ri.cv, registry, target)
			rt = &solarv1alpha1.RenderTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ri.rtName,
					Namespace: target.Namespace,
				},
				Spec: spec,
			}

			if err := r.Create(ctx, rt); err != nil {
				if apierrors.IsAlreadyExists(err) {
					// Another target with the same registry already created it; fetch it
					if err := r.Get(ctx, client.ObjectKey{Name: ri.rtName, Namespace: target.Namespace}, rt); err != nil {
						return ctrl.Result{}, errLogAndWrap(log, err, "failed to get existing RenderTask")
					}
				} else {
					return ctrl.Result{}, errLogAndWrap(log, err, "failed to create release RenderTask")
				}
			} else {
				log.V(1).Info("Created release RenderTask", "release", ri.name, "renderTask", ri.rtName)
				r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Created", "Create",
					"Created release RenderTask %s for release %s", ri.rtName, ri.name)
			}
		} else if err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get release RenderTask")
		}

		// Check if release RenderTask is complete
		if apimeta.IsStatusConditionTrue(rt.Status.Conditions, ConditionTypeJobFailed) {
			r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "ReleaseFailed",
				fmt.Sprintf("Release %s rendering failed", ri.name))

			return ctrl.Result{}, nil
		}

		if apimeta.IsStatusConditionTrue(rt.Status.Conditions, ConditionTypeJobSucceeded) && rt.Status.ChartURL != "" {
			releases[i].chartURL = rt.Status.ChartURL
		} else {
			allRendered = false
		}
	}

	if !allRendered {
		r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "Pending",
			"Waiting for release RenderTasks to complete")

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionTrue, "AllRendered",
		"All releases rendered successfully")

	// Determine if a new bootstrap render is needed by checking whether the
	// current bootstrapVersion's RenderTask still matches the desired release set.
	bootstrapVersion := target.Status.BootstrapVersion
	bootstrapRTName := targetRenderTaskName(target.Name, bootstrapVersion)
	bootstrapRT := &solarv1alpha1.RenderTask{}
	err := r.Get(ctx, client.ObjectKey{Name: bootstrapRTName, Namespace: target.Namespace}, bootstrapRT)

	needsNewBootstrap := false

	switch {
	case apierrors.IsNotFound(err):
		// No RenderTask for the current version yet — create one
		needsNewBootstrap = true
	case err != nil:
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get bootstrap RenderTask")
	default:
		// RenderTask exists — check if the release set changed
		releaseNames := make([]string, 0, len(releases))
		for _, ri := range releases {
			releaseNames = append(releaseNames, ri.name)
		}

		sort.Strings(releaseNames)

		existingNames := make([]string, 0, len(bootstrapRT.Spec.RendererConfig.BootstrapConfig.Input.Releases))
		for name := range bootstrapRT.Spec.RendererConfig.BootstrapConfig.Input.Releases {
			existingNames = append(existingNames, name)
		}

		sort.Strings(existingNames)

		if !slices.Equal(releaseNames, existingNames) {
			// Release set changed — bump version and create a new RenderTask
			bootstrapVersion++
			needsNewBootstrap = true
		}
	}

	if needsNewBootstrap {
		spec, specErr := r.computeBootstrapRenderTaskSpec(target, releases, registry, bootstrapVersion)
		if specErr != nil {
			return ctrl.Result{}, errLogAndWrap(log, specErr, "failed to compute bootstrap RenderTask spec")
		}

		bootstrapRTName = targetRenderTaskName(target.Name, bootstrapVersion)
		bootstrapRT = &solarv1alpha1.RenderTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bootstrapRTName,
				Namespace: target.Namespace,
			},
			Spec: spec,
		}

		if err := r.Create(ctx, bootstrapRT); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to create bootstrap RenderTask")
			}

			if err := r.Get(ctx, client.ObjectKey{Name: bootstrapRTName, Namespace: target.Namespace}, bootstrapRT); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to get existing bootstrap RenderTask")
			}
		} else {
			log.V(1).Info("Created bootstrap RenderTask", "renderTask", bootstrapRTName, "bootstrapVersion", bootstrapVersion)
			r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Created", "Create",
				"Created bootstrap RenderTask %s (version %d)", bootstrapRTName, bootstrapVersion)
		}

		// Persist the new bootstrapVersion in status
		if bootstrapVersion != target.Status.BootstrapVersion {
			target.Status.BootstrapVersion = bootstrapVersion
			if err := r.Status().Update(ctx, target); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to update Target bootstrapVersion")
			}
		}
	}

	// Update target status from bootstrap RenderTask
	if apimeta.IsStatusConditionTrue(bootstrapRT.Status.Conditions, ConditionTypeJobFailed) {
		r.setCondition(ctx, target, ConditionTypeBootstrapReady, metav1.ConditionFalse, "Failed",
			"Bootstrap rendering failed")

		return ctrl.Result{}, nil
	}

	if apimeta.IsStatusConditionTrue(bootstrapRT.Status.Conditions, ConditionTypeJobSucceeded) {
		r.setCondition(ctx, target, ConditionTypeBootstrapReady, metav1.ConditionTrue, "Ready",
			"Bootstrap rendered successfully: "+bootstrapRT.Status.ChartURL)

		// Clean up stale RenderTasks owned by this target (old bootstrap versions)
		if err := r.deleteStaleRenderTasks(ctx, target, bootstrapRTName); err != nil {
			log.Error(err, "failed to clean up stale RenderTasks")
		}

		return ctrl.Result{}, nil
	}

	// Still running
	return ctrl.Result{}, nil
}

func (r *TargetReconciler) setCondition(ctx context.Context, target *solarv1alpha1.Target, condType string, status metav1.ConditionStatus, reason, message string) {
	changed := apimeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: target.Generation,
		Reason:             reason,
		Message:            message,
	})
	if changed {
		if err := r.Status().Update(ctx, target); err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "failed to update Target status condition", "type", condType)
		}
	}
}

// deleteStaleRenderTasks removes RenderTasks owned by this target that are no
// longer needed. It keeps the current bootstrap RenderTask and all release
// RenderTasks (which may be shared across targets). Only old bootstrap
// RenderTasks from previous versions are deleted.
func (r *TargetReconciler) deleteStaleRenderTasks(ctx context.Context, target *solarv1alpha1.Target, currentBootstrapRT string) error {
	log := ctrl.LoggerFrom(ctx)

	rtList := &solarv1alpha1.RenderTaskList{}
	if err := r.List(ctx, rtList,
		client.InNamespace(target.Namespace),
		client.MatchingFields{indexOwnerKind: "Target"},
	); err != nil {
		return err
	}

	for i := range rtList.Items {
		rt := &rtList.Items[i]
		if rt.Spec.OwnerName != target.Name || rt.Spec.OwnerNamespace != target.Namespace {
			continue
		}

		if rt.Name == currentBootstrapRT {
			continue
		}

		// Only clean up bootstrap RenderTasks (render-tgt-*), not release ones (render-rel-*)
		if rt.Spec.RendererConfig.Type != solarv1alpha1.RendererConfigTypeBootstrap {
			continue
		}

		log.V(1).Info("Deleting stale bootstrap RenderTask", "renderTask", rt.Name)
		if err := r.Delete(ctx, rt, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			return err
		}

		r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Deleted", "Delete",
			"Deleted stale bootstrap RenderTask %s", rt.Name)
	}

	return nil
}

func (r *TargetReconciler) deleteOwnedRenderTasks(ctx context.Context, target *solarv1alpha1.Target) error {
	rtList := &solarv1alpha1.RenderTaskList{}
	if err := r.List(ctx, rtList,
		client.InNamespace(target.Namespace),
		client.MatchingFields{indexOwnerKind: "Target"},
	); err != nil {
		return err
	}

	for i := range rtList.Items {
		rt := &rtList.Items[i]
		if rt.Spec.OwnerName == target.Name && rt.Spec.OwnerNamespace == target.Namespace {
			if err := r.Delete(ctx, rt, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}

	return nil
}

func (r *TargetReconciler) computeReleaseRenderTaskSpec(rel *solarv1alpha1.Release, cv *solarv1alpha1.ComponentVersion, registry *solarv1alpha1.Registry, target *solarv1alpha1.Target) solarv1alpha1.RenderTaskSpec {
	chartName := fmt.Sprintf("release-%s", rel.Name)
	repo := fmt.Sprintf("%s/%s", target.Namespace, chartName)
	tag := fmt.Sprintf("v0.0.%d", rel.GetGeneration())

	return solarv1alpha1.RenderTaskSpec{
		RendererConfig: solarv1alpha1.RendererConfig{
			Type: solarv1alpha1.RendererConfigTypeRelease,
			ReleaseConfig: solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        chartName,
					Description: fmt.Sprintf("Release of %s", rel.Spec.ComponentVersionRef.Name),
					Version:     tag,
					AppVersion:  tag,
				},
				Input: solarv1alpha1.ReleaseInput{
					Component:  solarv1alpha1.ReleaseComponent{Name: cv.Spec.ComponentRef.Name},
					Resources:  cv.Spec.Resources,
					Entrypoint: cv.Spec.Entrypoint,
				},
				Values: rel.Spec.Values,
			},
		},
		Repository:     repo,
		Tag:            tag,
		BaseURL:        registry.Spec.Hostname,
		PushSecretRef:  registry.Spec.SolarSecretRef,
		FailedJobTTL:   rel.Spec.FailedJobTTL,
		OwnerName:      target.Name,
		OwnerNamespace: target.Namespace,
		OwnerKind:      "Target",
	}
}

func (r *TargetReconciler) computeBootstrapRenderTaskSpec(target *solarv1alpha1.Target, releases []releaseInfo, registry *solarv1alpha1.Registry, bootstrapVersion int64) (solarv1alpha1.RenderTaskSpec, error) {
	resolvedReleases := map[string]solarv1alpha1.ResourceAccess{}
	resolvedReleaseNames := make([]string, 0, len(releases))

	for _, ri := range releases {
		ref, err := ociname.ParseReference(ri.chartURL)
		if err != nil {
			return solarv1alpha1.RenderTaskSpec{}, fmt.Errorf("failed to parse chartURL %s: %w", ri.chartURL, err)
		}

		repo, err := url.JoinPath(ref.Context().RegistryStr(), ref.Context().RepositoryStr())
		if err != nil {
			return solarv1alpha1.RenderTaskSpec{}, err
		}

		resolvedReleases[ri.name] = solarv1alpha1.ResourceAccess{
			Repository: strings.TrimPrefix(repo, "oci://"),
			Tag:        ref.Identifier(),
		}
		resolvedReleaseNames = append(resolvedReleaseNames, ri.name)
	}

	sort.Strings(resolvedReleaseNames)

	chartName := fmt.Sprintf("bootstrap-%s", target.Name)
	repo := fmt.Sprintf("%s/%s", target.Namespace, chartName)
	tag := fmt.Sprintf("v0.0.%d", bootstrapVersion)

	return solarv1alpha1.RenderTaskSpec{
		RendererConfig: solarv1alpha1.RendererConfig{
			Type: solarv1alpha1.RendererConfigTypeBootstrap,
			BootstrapConfig: solarv1alpha1.BootstrapConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        chartName,
					Description: fmt.Sprintf("Bootstrap of %v", resolvedReleaseNames),
					Version:     tag,
					AppVersion:  tag,
				},
				Input: solarv1alpha1.BootstrapInput{
					Releases: resolvedReleases,
					Userdata: target.Spec.Userdata,
				},
			},
		},
		Repository:     repo,
		Tag:            tag,
		BaseURL:        registry.Spec.Hostname,
		PushSecretRef:  registry.Spec.SolarSecretRef,
		OwnerName:      target.Name,
		OwnerNamespace: target.Namespace,
		OwnerKind:      "Target",
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Target{}).
		Watches(
			&solarv1alpha1.ReleaseBinding{},
			handler.EnqueueRequestsFromMapFunc(r.mapReleaseBindingToTarget),
		).
		Watches(
			&solarv1alpha1.RenderTask{},
			handler.EnqueueRequestsFromMapFunc(mapRenderTaskToOwner("Target")),
			builder.WithPredicates(renderTaskStatusChangePredicate()),
		).
		Complete(r)
}

func (r *TargetReconciler) mapReleaseBindingToTarget(_ context.Context, obj client.Object) []reconcile.Request {
	rb, ok := obj.(*solarv1alpha1.ReleaseBinding)
	if !ok || rb.Spec.TargetRef.Name == "" {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      rb.Spec.TargetRef.Name,
				Namespace: rb.Namespace,
			},
		},
	}
}
