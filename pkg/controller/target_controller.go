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
	apiequality "k8s.io/apimachinery/pkg/api/equality"
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
	name              string
	release           *solarv1alpha1.Release
	cv                *solarv1alpha1.ComponentVersion
	resolvedResources map[string]solarv1alpha1.ResourceAccess
	rtName            string
	chartURL          string
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
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registrybindings,verbs=get;list;watch
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
			if condErr := r.setCondition(ctx, target, ConditionTypeRegistryResolved, metav1.ConditionFalse, "NotFound",
				"Registry not found: "+target.Spec.RenderRegistryRef.Name); condErr != nil {
				return ctrl.Result{}, condErr
			}

			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Registry")
	}

	if registry.Spec.SolarSecretRef == nil {
		if condErr := r.setCondition(ctx, target, ConditionTypeRegistryResolved, metav1.ConditionFalse, "MissingSolarSecretRef",
			"Registry does not have SolarSecretRef set, required for rendering"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{}, nil
	}

	if condErr := r.setCondition(ctx, target, ConditionTypeRegistryResolved, metav1.ConditionTrue, "Resolved",
		"Registry resolved: "+registry.Name); condErr != nil {
		return ctrl.Result{}, condErr
	}

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
		if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "NoBindings",
			"No ReleaseBindings found for this target"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{}, nil
	}

	// Collect RegistryBindings for this target
	regBindingList := &solarv1alpha1.RegistryBindingList{}
	if err := r.List(ctx, regBindingList,
		client.InNamespace(target.Namespace),
		client.MatchingFields{indexRegistryBindingTargetName: target.Name},
	); err != nil {
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to list RegistryBindings")
	}

	// Resolve each RegistryBinding to its Registry
	var regBindings []registryBindingInfo

	for i := range regBindingList.Items {
		rb := &regBindingList.Items[i]
		reg := &solarv1alpha1.Registry{}

		if err := r.Get(ctx, client.ObjectKey{
			Name:      rb.Spec.RegistryRef.Name,
			Namespace: target.Namespace,
		}, reg); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("Registry for RegistryBinding not found", "registry", rb.Spec.RegistryRef.Name)

				continue
			}

			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Registry for RegistryBinding")
		}

		regBindings = append(regBindings, registryBindingInfo{binding: rb, registry: reg})
	}

	// For each bound release, ensure a per-release RenderTask exists
	var releases []releaseInfo

	pendingDeps := false

	for _, binding := range bindingList.Items {
		rel := &solarv1alpha1.Release{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      binding.Spec.ReleaseRef.Name,
			Namespace: target.Namespace,
		}, rel); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("Release not found", "release", binding.Spec.ReleaseRef.Name)
				pendingDeps = true

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
				pendingDeps = true

				continue
			}

			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get ComponentVersion")
		}

		// Resolve resources through RegistryBindings
		resolvedRes, resolveErr := resolveResources(cv.Spec.Resources, regBindings)
		if resolveErr != nil {
			log.V(1).Info("Failed to resolve resources for release", "release", rel.Name, "error", resolveErr)
			pendingDeps = true

			continue
		}

		rtName := releaseRenderTaskName(rel.Name, target.Name, rel.GetGeneration())
		releases = append(releases, releaseInfo{
			name:              rel.Name,
			release:           rel,
			cv:                cv,
			resolvedResources: resolvedRes,
			rtName:            rtName,
		})
	}

	// Create per-release RenderTasks (one per target+release pair).
	// The renderer job handles dedup by skipping if the chart already exists in the registry.
	allRendered := true

	for i, ri := range releases {
		rt := &solarv1alpha1.RenderTask{}
		err := r.Get(ctx, client.ObjectKey{Name: ri.rtName, Namespace: target.Namespace}, rt)

		if apierrors.IsNotFound(err) {
			spec := r.computeReleaseRenderTaskSpec(ri, registry, target)
			rt = &solarv1alpha1.RenderTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ri.rtName,
					Namespace: target.Namespace,
				},
				Spec: spec,
			}

			if err := r.Create(ctx, rt); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to create release RenderTask")
			}

			log.V(1).Info("Created release RenderTask", "release", ri.name, "renderTask", ri.rtName)
			r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Created", "Create",
				"Created release RenderTask %s for release %s", ri.rtName, ri.name)
		} else if err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get release RenderTask")
		}

		// Check if release RenderTask is complete
		if apimeta.IsStatusConditionTrue(rt.Status.Conditions, ConditionTypeJobFailed) {
			if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "ReleaseFailed",
				fmt.Sprintf("Release %s rendering failed", ri.name)); condErr != nil {
				return ctrl.Result{}, condErr
			}

			return ctrl.Result{}, nil
		}

		if apimeta.IsStatusConditionTrue(rt.Status.Conditions, ConditionTypeJobSucceeded) && rt.Status.ChartURL != "" {
			releases[i].chartURL = rt.Status.ChartURL
		} else {
			allRendered = false
		}
	}

	if pendingDeps {
		if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "MissingDependencies",
			"One or more bound Releases or ComponentVersions not found"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if !allRendered {
		if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "Pending",
			"Waiting for release RenderTasks to complete"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionTrue, "AllRendered",
		"All releases rendered successfully"); condErr != nil {
		return ctrl.Result{}, condErr
	}

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
		// RenderTask exists — check if the desired bootstrap input changed
		// (release set, resolved refs/tags, or userdata)
		desiredInput, inputErr := buildBootstrapInput(target, releases, registry, regBindings)
		if inputErr != nil {
			return ctrl.Result{}, errLogAndWrap(log, inputErr, "failed to build desired bootstrap input for comparison")
		}

		existingInput := bootstrapRT.Spec.RendererConfig.BootstrapConfig.Input
		if !apiequality.Semantic.DeepEqual(desiredInput, existingInput) {
			bootstrapVersion++
			needsNewBootstrap = true
		}
	}

	if needsNewBootstrap {
		spec, specErr := r.computeBootstrapRenderTaskSpec(target, releases, registry, regBindings, bootstrapVersion)
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
		if condErr := r.setCondition(ctx, target, ConditionTypeBootstrapReady, metav1.ConditionFalse, "Failed",
			"Bootstrap rendering failed"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{}, nil
	}

	if apimeta.IsStatusConditionTrue(bootstrapRT.Status.Conditions, ConditionTypeJobSucceeded) {
		if condErr := r.setCondition(ctx, target, ConditionTypeBootstrapReady, metav1.ConditionTrue, "Ready",
			"Bootstrap rendered successfully: "+bootstrapRT.Status.ChartURL); condErr != nil {
			return ctrl.Result{}, condErr
		}

		// Clean up stale RenderTasks owned by this target (old versions)
		currentRTNames := map[string]struct{}{bootstrapRTName: {}}
		for _, ri := range releases {
			currentRTNames[ri.rtName] = struct{}{}
		}
		if err := r.deleteStaleRenderTasks(ctx, target, currentRTNames); err != nil {
			log.Error(err, "failed to clean up stale RenderTasks")
		}

		return ctrl.Result{}, nil
	}

	// Still running
	return ctrl.Result{}, nil
}

func (r *TargetReconciler) setCondition(ctx context.Context, target *solarv1alpha1.Target, condType string, status metav1.ConditionStatus, reason, message string) error {
	changed := apimeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: target.Generation,
		Reason:             reason,
		Message:            message,
	})
	if changed {
		if err := r.Status().Update(ctx, target); err != nil {
			return fmt.Errorf("failed to update Target status condition %s: %w", condType, err)
		}
	}

	return nil
}

// deleteStaleRenderTasks removes RenderTasks owned by this target that are no
// longer needed. Any owned RenderTask whose name is not in currentRTNames is
// deleted. This covers both old bootstrap versions and old release generations.
func (r *TargetReconciler) deleteStaleRenderTasks(ctx context.Context, target *solarv1alpha1.Target, currentRTNames map[string]struct{}) error {
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

		if _, current := currentRTNames[rt.Name]; current {
			continue
		}

		log.V(1).Info("Deleting stale RenderTask", "renderTask", rt.Name)
		if err := r.Delete(ctx, rt, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			return err
		}

		r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Deleted", "Delete",
			"Deleted stale RenderTask %s", rt.Name)
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

func (r *TargetReconciler) computeReleaseRenderTaskSpec(ri releaseInfo, registry *solarv1alpha1.Registry, target *solarv1alpha1.Target) solarv1alpha1.RenderTaskSpec {
	chartName := fmt.Sprintf("release-%s", ri.release.Name)
	repo := fmt.Sprintf("%s/%s", target.Namespace, chartName)
	tag := fmt.Sprintf("v0.0.%d", ri.release.GetGeneration())

	var targetNamespace string
	if ri.release.Spec.TargetNamespace != nil {
		targetNamespace = *ri.release.Spec.TargetNamespace
	}

	return solarv1alpha1.RenderTaskSpec{
		RendererConfig: solarv1alpha1.RendererConfig{
			Type: solarv1alpha1.RendererConfigTypeRelease,
			ReleaseConfig: solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        chartName,
					Description: fmt.Sprintf("Release of %s", ri.release.Spec.ComponentVersionRef.Name),
					Version:     tag,
					AppVersion:  tag,
				},
				Input: solarv1alpha1.ReleaseInput{
					Component:  solarv1alpha1.ReleaseComponent{Name: ri.cv.Spec.ComponentRef.Name},
					Resources:  ri.resolvedResources,
					Entrypoint: ri.cv.Spec.Entrypoint,
				},
				Values:          ri.release.Spec.Values,
				TargetNamespace: targetNamespace,
			},
		},
		Repository:     repo,
		Tag:            tag,
		BaseURL:        registry.Spec.Hostname,
		Insecure:       registry.Spec.PlainHTTP,
		SecretRef:      registry.Spec.SolarSecretRef,
		FailedJobTTL:   ri.release.Spec.FailedJobTTL,
		OwnerName:      target.Name,
		OwnerNamespace: target.Namespace,
		OwnerKind:      "Target",
	}
}

// buildBootstrapInput constructs the desired BootstrapInput from the current
// target and resolved releases. Used for both comparison and spec construction.
func buildBootstrapInput(target *solarv1alpha1.Target, releases []releaseInfo, registry *solarv1alpha1.Registry, regBindings []registryBindingInfo) (solarv1alpha1.BootstrapInput, error) {
	resolvedReleases := map[string]solarv1alpha1.ResourceAccess{}

	for _, ri := range releases {
		ref, err := ociname.ParseReference(ri.chartURL)
		if err != nil {
			return solarv1alpha1.BootstrapInput{}, fmt.Errorf("failed to parse chartURL %s: %w", ri.chartURL, err)
		}

		repo, err := url.JoinPath(ref.Context().RegistryStr(), ref.Context().RepositoryStr())
		if err != nil {
			return solarv1alpha1.BootstrapInput{}, err
		}

		resolvedReleases[ri.name] = solarv1alpha1.ResourceAccess{
			Repository:     strings.TrimPrefix(repo, "oci://"),
			Tag:            ref.Identifier(),
			PullSecretName: registry.Spec.TargetPullSecretName,
		}
	}

	// Apply RegistryBinding rewrites to bootstrap releases
	resolvedReleases = resolveBootstrapReleases(resolvedReleases, regBindings)

	return solarv1alpha1.BootstrapInput{
		Releases: resolvedReleases,
		Userdata: target.Spec.Userdata,
	}, nil
}

func (r *TargetReconciler) computeBootstrapRenderTaskSpec(target *solarv1alpha1.Target, releases []releaseInfo, registry *solarv1alpha1.Registry, regBindings []registryBindingInfo, bootstrapVersion int64) (solarv1alpha1.RenderTaskSpec, error) {
	input, err := buildBootstrapInput(target, releases, registry, regBindings)
	if err != nil {
		return solarv1alpha1.RenderTaskSpec{}, err
	}

	releaseNames := make([]string, 0, len(releases))
	for _, ri := range releases {
		releaseNames = append(releaseNames, ri.name)
	}

	sort.Strings(releaseNames)

	chartName := fmt.Sprintf("bootstrap-%s", target.Name)
	repo := fmt.Sprintf("%s/%s", target.Namespace, chartName)
	tag := fmt.Sprintf("v0.0.%d", bootstrapVersion)

	return solarv1alpha1.RenderTaskSpec{
		RendererConfig: solarv1alpha1.RendererConfig{
			Type: solarv1alpha1.RendererConfigTypeBootstrap,
			BootstrapConfig: solarv1alpha1.BootstrapConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        chartName,
					Description: fmt.Sprintf("Bootstrap of %v", releaseNames),
					Version:     tag,
					AppVersion:  tag,
				},
				Input: input,
			},
		},
		Repository:     repo,
		Tag:            tag,
		BaseURL:        registry.Spec.Hostname,
		Insecure:       registry.Spec.PlainHTTP,
		SecretRef:      registry.Spec.SolarSecretRef,
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
			&solarv1alpha1.RegistryBinding{},
			handler.EnqueueRequestsFromMapFunc(r.mapRegistryBindingToTarget),
		).
		Watches(
			&solarv1alpha1.RenderTask{},
			handler.EnqueueRequestsFromMapFunc(mapRenderTaskToOwner("Target")),
			builder.WithPredicates(renderTaskStatusChangePredicate()),
		).
		Watches(
			&solarv1alpha1.Registry{},
			handler.EnqueueRequestsFromMapFunc(r.mapRegistryToTargets),
		).
		Watches(
			&solarv1alpha1.Release{},
			handler.EnqueueRequestsFromMapFunc(r.mapReleaseToTargets),
		).
		Complete(r)
}

// mapRegistryToTargets maps a Registry event to reconcile requests for all
// Targets in the same namespace that reference it via renderRegistryRef.
func (r *TargetReconciler) mapRegistryToTargets(ctx context.Context, obj client.Object) []reconcile.Request {
	reg, ok := obj.(*solarv1alpha1.Registry)
	if !ok {
		return nil
	}

	targetList := &solarv1alpha1.TargetList{}
	if err := r.List(ctx, targetList, client.InNamespace(reg.Namespace)); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to list Targets for Registry", "registry", reg.Name)

		return nil
	}

	var requests []reconcile.Request
	for _, t := range targetList.Items {
		if t.Spec.RenderRegistryRef.Name == reg.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      t.Name,
					Namespace: t.Namespace,
				},
			})
		}
	}

	return requests
}

// mapReleaseToTargets maps a Release event to reconcile requests for all
// Targets that are bound to the release via ReleaseBindings.
func (r *TargetReconciler) mapReleaseToTargets(ctx context.Context, obj client.Object) []reconcile.Request {
	rel, ok := obj.(*solarv1alpha1.Release)
	if !ok {
		return nil
	}

	bindingList := &solarv1alpha1.ReleaseBindingList{}
	if err := r.List(ctx, bindingList,
		client.InNamespace(rel.Namespace),
		client.MatchingFields{indexReleaseBindingReleaseName: rel.Name},
	); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to list ReleaseBindings for Release", "release", rel.Name)

		return nil
	}

	seen := map[string]struct{}{}
	var requests []reconcile.Request

	for _, rb := range bindingList.Items {
		targetName := rb.Spec.TargetRef.Name
		if _, ok := seen[targetName]; ok {
			continue
		}

		seen[targetName] = struct{}{}
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      targetName,
				Namespace: rb.Namespace,
			},
		})
	}

	return requests
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

func (r *TargetReconciler) mapRegistryBindingToTarget(_ context.Context, obj client.Object) []reconcile.Request {
	rb, ok := obj.(*solarv1alpha1.RegistryBinding)
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
