// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	ociname "github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	hydratedTargetFinalizer = "solar.opendefense.cloud/hydrated-target-finalizer"
)

// HydratedTargetReconciler reconciles a HydratedTarget object
type HydratedTargetReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Recorder    record.EventRecorder
	PushOptions solarv1alpha1.PushOptions
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets/finalizers,verbs=update
// FIXME: Switch out releases for profiles                      👇
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile moves the current state of the cluster closer to the desired state
//
// Reconciliation Flow:
//
//	HydratedTarget created
//	    ↓
//	Add finalizer
//	    ↓
//	Check if already succeeded → YES → Return (no-op)
//	    ↓ NO
//	Get or create RenderTask
//	    ↓
//	Update status from RenderTask

func (r *HydratedTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("HydratedTarget is being reconciled", "req", req)

	// Fetch the HydratedTarget instance
	res := &solarv1alpha1.HydratedTarget{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrlResult, nil
		}

		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Handle deletion: cleanup rendertask, then remove finalizer
	if !res.DeletionTimestamp.IsZero() {
		log.V(1).Info("HydratedTarget is being deleted")
		r.Recorder.Event(res, corev1.EventTypeWarning, "Deleting", "HydratedTarget is being deleted, cleaning up resources")

		if err := r.deleteRenderTask(ctx, res); client.IgnoreNotFound(err) != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to delete render task")
		}

		// Remove finalizer
		if slices.Contains(res.Finalizers, hydratedTargetFinalizer) {
			log.V(1).Info("Removing finalizer from resource")
			res.Finalizers = slices.DeleteFunc(res.Finalizers, func(f string) bool {
				return f == hydratedTargetFinalizer
			})
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer")
			}
		}

		return ctrlResult, nil
	}

	// Add finalizer if not present and not deleting
	if res.DeletionTimestamp.IsZero() {
		if !slices.Contains(res.Finalizers, hydratedTargetFinalizer) {
			log.V(1).Info("Adding finalizer to resource")
			res.Finalizers = append(res.Finalizers, hydratedTargetFinalizer)
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer")
			}
			// Return without requeue; the Update event will trigger reconciliation again
			return ctrlResult, nil
		}
	}

	// Check if rendertask has already completed successfully
	sc := apimeta.FindStatusCondition(res.Status.Conditions, ConditionTypeTaskCompleted)
	if sc != nil && sc.ObservedGeneration == res.Generation && sc.Status == metav1.ConditionTrue {
		log.V(1).Info("RenderTask has already completed successfully, no further action needed")
		return ctrlResult, nil
	}

	// Check if rendertask has already failed
	fc := apimeta.FindStatusCondition(res.Status.Conditions, ConditionTypeTaskFailed)
	if fc != nil && fc.ObservedGeneration == res.Generation && fc.Status == metav1.ConditionTrue {
		log.V(1).Info("RenderTask has already failed, no further action needed")
		return ctrlResult, nil
	}

	// Reconcile RenderTask
	rt := &solarv1alpha1.RenderTask{}
	err := r.Get(ctx, client.ObjectKey{Name: generationName(res), Namespace: res.Namespace}, rt)
	if client.IgnoreNotFound(err) != nil {
		log.V(1).Info("Failed to get render task", "res", res)
		return ctrlResult, errLogAndWrap(log, err, "failed to get RenderTask")
	}

	if apierrors.IsNotFound(err) {
		if err := r.createRenderTask(ctx, res); err != nil {
			log.V(1).Info("Failed to create RenderTask", "res", res)
			r.Recorder.Event(res, corev1.EventTypeWarning, "CreationFailed", "failed to create RenderTask")

			return ctrl.Result{RequeueAfter: 30 * time.Second}, errLogAndWrap(log, err, "failed to create RenderTask")
		}
		log.V(1).Info("Created RenderTask", "res", res)
		r.Recorder.Event(res, corev1.EventTypeNormal, "Created", "RenderTask was created")
	}

	if changed := r.updateStatusConditionsFromRenderTask(ctx, res, rt); changed {
		if err := r.Status().Update(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to update status")
		}
	}

	// RenderTask still running, requeue
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *HydratedTargetReconciler) updateStatusConditionsFromRenderTask(ctx context.Context, res *solarv1alpha1.HydratedTarget, rt *solarv1alpha1.RenderTask) (changed bool) {
	if rt == nil || res == nil {
		return false
	}

	log := ctrl.LoggerFrom(ctx)

	if apimeta.IsStatusConditionTrue(rt.Status.Conditions, ConditionTypeJobFailed) {
		changed = apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeTaskFailed,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: res.Generation,
			Reason:             "TaskFailed",
			Message:            "RenderTask failed",
		})

		log.V(1).Info("RenderTask failed", "name", rt.Name)
		r.Recorder.Event(res, corev1.EventTypeWarning, "TaskFailed", "RendererTask failed")

		return changed
	}

	if apimeta.IsStatusConditionTrue(rt.Status.Conditions, ConditionTypeJobSucceeded) {
		changed = apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeTaskCompleted,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: res.Generation,
			Reason:             "TaskCompleted",
			Message:            "RenderTask completed",
		})

		log.V(1).Info("RenderTask failed", "name", rt.Name)
		r.Recorder.Event(res, corev1.EventTypeWarning, "TaskCompleted", "RendererTask completed successfully")

		return changed
	}

	log.V(1).Info("RenderTask has no final condtions yet", "name", rt.Name)

	return false
}

func (r *HydratedTargetReconciler) createRenderTask(ctx context.Context, res *solarv1alpha1.HydratedTarget) error {
	log := ctrl.LoggerFrom(ctx)

	// Check if we need to cleanup an old task
	if res.Status.RenderTaskRef != nil && res.Status.RenderTaskRef.Name != "" {
		if err := r.deleteRenderTask(ctx, res); err != nil {
			return errLogAndWrap(log, err, "failed to cleanup old task")
		}
	}

	cfg, err := r.computeRendererConfig(ctx, res)
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("unexpected nil RendererConfig")
	}

	rt := &solarv1alpha1.RenderTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generationName(res),
			Namespace: res.Namespace,
		},
		Spec: solarv1alpha1.RenderTaskSpec{
			RendererConfig: *cfg,
		},
	}

	if err := r.Create(ctx, rt); err != nil {
		r.Recorder.Eventf(res, corev1.EventTypeWarning, "CreationFailed", "Failed to create RenderTask", err)
		return errLogAndWrap(log, err, "secret creation failed")
	}

	// Set owner references
	if err := controllerutil.SetControllerReference(res, rt, r.Scheme); err != nil {
		return errLogAndWrap(log, err, "failed to set controller reference")
	}

	// Set Reference in Status
	if err := r.Get(ctx, client.ObjectKey{Name: rt.Name, Namespace: rt.Namespace}, rt); err != nil {
		return errLogAndWrap(log, err, "could not fetch RenderTask")
	}

	res.Status.RenderTaskRef = &corev1.ObjectReference{
		APIVersion: rt.APIVersion,
		Kind:       rt.Kind,
		Namespace:  rt.Namespace,
		Name:       rt.Name,
	}

	if err := r.Status().Update(ctx, res); err != nil {
		return errLogAndWrap(log, err, "failed to update status")
	}

	return nil
}

func (r *HydratedTargetReconciler) deleteRenderTask(ctx context.Context, res *solarv1alpha1.HydratedTarget) error {
	if res.Status.RenderTaskRef == nil {
		return nil
	}

	rt := &solarv1alpha1.RenderTask{}
	if err := r.Get(ctx, client.ObjectKey{Name: res.Status.RenderTaskRef.Name, Namespace: res.Status.RenderTaskRef.Namespace}, rt); client.IgnoreNotFound(err) != nil {
		return err
	} else if err == nil {
		return r.Delete(ctx, rt, client.PropagationPolicy(metav1.DeletePropagationBackground))
	}

	return nil
}

func (r *HydratedTargetReconciler) computeRendererConfig(ctx context.Context, res *solarv1alpha1.HydratedTarget) (*solarv1alpha1.RendererConfig, error) {
	resolvedReleases := map[string]solarv1alpha1.ResourceAccess{}
	for k, v := range res.Spec.Releases {
		rel := &solarv1alpha1.Release{}
		if err := r.Get(ctx, client.ObjectKey{Name: v.Name, Namespace: res.Namespace}, rel); err != nil {
			return nil, err
		}

		ref, err := ociname.ParseReference(rel.Status.ChartURL)
		if err != nil {
			return nil, err
		}

		repo, err := url.JoinPath(ref.Context().RegistryStr(), ref.Context().RepositoryStr())
		if err != nil {
			return nil, err
		}

		resolvedReleases[k] = solarv1alpha1.ResourceAccess{
			Repository: strings.TrimPrefix(repo, "oci://"),
			Tag:        ref.Identifier(),
		}
	}

	resolvedReleaseNames := []string{}
	for k := range resolvedReleases {
		resolvedReleaseNames = append(resolvedReleaseNames, k)
	}

	po := r.PushOptions
	url, err := url.JoinPath(po.ReferenceURL, res.Namespace, fmt.Sprintf("ht-%s", res.Name))
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(url, "oci://") {
		url = fmt.Sprintf("oci://%s", url)
	}

	version := fmt.Sprintf("v0.0.%d", res.GetGeneration())
	url = fmt.Sprintf("%s:%s", url, version)

	po.ReferenceURL = url

	return &solarv1alpha1.RendererConfig{
		Type: solarv1alpha1.RendererConfigTypeHydratedTarget,
		HydratedTargetConfig: solarv1alpha1.HydratedTargetConfig{
			Chart: solarv1alpha1.ChartConfig{
				Name:        res.Name,
				Description: fmt.Sprintf("HydratedTarget of %v", resolvedReleaseNames),
				Version:     version,
				AppVersion:  version,
			},
			Input: solarv1alpha1.HydratedTargetInput{
				Releases: resolvedReleases,
				Userdata: res.Spec.Userdata,
			},
		},
		PushOptions: po,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HydratedTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.HydratedTarget{}).
		Owns(&solarv1alpha1.RenderTask{}).
		Complete(r)
}
