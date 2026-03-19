// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	ConditionTypeComponentVersionResolved = "ComponentVersionResolved"
)

// ReleaseReconciler reconciles a Release object.
// It validates that the referenced ComponentVersion exists and sets status conditions.
// Rendering is handled by the Target controller.
type ReleaseReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	// See: https://book.kubebuilder.io/reference/envtest#testing-considerations
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile validates the Release by resolving its ComponentVersion reference.
func (r *ReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("Release is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrlResult, nil
	}

	// Fetch the Release instance
	res := &solarv1alpha1.Release{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrlResult, nil
		}

		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Resolve ComponentVersion
	cvRef := types.NamespacedName{
		Name:      res.Spec.ComponentVersionRef.Name,
		Namespace: res.Namespace,
	}
	cv := &solarv1alpha1.ComponentVersion{}
	if err := r.Get(ctx, cvRef, cv); err != nil {
		if apierrors.IsNotFound(err) {
			changed := apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeComponentVersionResolved,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: res.Generation,
				Reason:             "NotFound",
				Message:            "ComponentVersion not found: " + res.Spec.ComponentVersionRef.Name,
			})
			if changed {
				if err := r.Status().Update(ctx, res); err != nil {
					return ctrlResult, errLogAndWrap(log, err, "failed to update status")
				}
			}

			return ctrlResult, nil
		}

		return ctrlResult, errLogAndWrap(log, err, "failed to get ComponentVersion")
	}

	// ComponentVersion found — set resolved condition
	changed := apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeComponentVersionResolved,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: res.Generation,
		Reason:             "Resolved",
		Message:            "ComponentVersion resolved: " + cv.Name,
	})
	if changed {
		if err := r.Status().Update(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to update status")
		}
	}

	return ctrlResult, nil
}

func (r *ReleaseReconciler) updateStatusConditionsFromRenderTask(ctx context.Context, res *solarv1alpha1.Release, rt *solarv1alpha1.RenderTask) (changed bool) {
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
		r.Recorder.Eventf(res, rt, corev1.EventTypeWarning, "TaskFailed", "RunTask", "RenderTask failed")

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

		if res.Status.ChartURL != rt.Status.ChartURL {
			res.Status.ChartURL = rt.Status.ChartURL
			changed = true
		}

		log.V(1).Info("RenderTask succeeded", "name", rt.Name)
		r.Recorder.Eventf(res, rt, corev1.EventTypeWarning, "TaskCompleted", "RunTask", "RenderTask completed successfully")

		return changed
	}

	log.V(1).Info("RenderTask has no final condtions yet", "name", rt.Name)

	return false
}

func (r *ReleaseReconciler) createRenderTask(ctx context.Context, res *solarv1alpha1.Release) error {
	log := ctrl.LoggerFrom(ctx)

	// Check if we need to cleanup an old task
	if res.Status.RenderTaskRef != nil && res.Status.RenderTaskRef.Name != "" {
		if err := r.deleteRenderTask(ctx, res); err != nil {
			return errLogAndWrap(log, err, "failed to cleanup old task")
		}
	}

	spec, err := r.computeRenderTaskSpec(ctx, res)
	if err != nil {
		return err
	}
	rt := &solarv1alpha1.RenderTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: renderTaskName(res),
		},
		Spec: spec,
	}
	rt.Spec.OwnerName = res.Name
	rt.Spec.OwnerNamespace = res.Namespace
	rt.Spec.OwnerKind = "Release"

	if err := r.Create(ctx, rt); err != nil {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "CreationFailed", "Create", "Failed to create RenderTask", err)
		return errLogAndWrap(log, err, "failed to create RenderTask")
	}

	// Set Reference in Status
	res.Status.RenderTaskRef = &corev1.ObjectReference{
		APIVersion: solarv1alpha1.SchemeGroupVersion.String(),
		Kind:       "RenderTask",
		Name:       rt.Name,
	}

	if err := r.Status().Update(ctx, res); err != nil {
		return errLogAndWrap(log, err, "failed to update status")
	}

	return nil
}

func (r *ReleaseReconciler) deleteRenderTask(ctx context.Context, res *solarv1alpha1.Release) error {
	if res.Status.RenderTaskRef == nil {
		return nil
	}

	rt := &solarv1alpha1.RenderTask{}
	if err := r.Get(ctx, client.ObjectKey{Name: res.Status.RenderTaskRef.Name}, rt); client.IgnoreNotFound(err) != nil {
		return err
	} else if err == nil {
		return r.Delete(ctx, rt, client.PropagationPolicy(metav1.DeletePropagationBackground))
	}

	return nil
}

func (r *ReleaseReconciler) computeRenderTaskSpec(ctx context.Context, res *solarv1alpha1.Release) (solarv1alpha1.RenderTaskSpec, error) {
	spec := solarv1alpha1.RenderTaskSpec{}

	cvRef := types.NamespacedName{
		Name:      res.Spec.ComponentVersionRef.Name,
		Namespace: res.Namespace,
	}

	cv := &solarv1alpha1.ComponentVersion{}
	if err := r.Get(ctx, cvRef, cv); err != nil {
		return spec, err
	}

	chartName := fmt.Sprintf("release-%s", res.Name)
	repo, err := url.JoinPath(res.Namespace, chartName)
	if err != nil {
		return spec, err
	}

	tag := fmt.Sprintf("v0.0.%d", res.GetGeneration())

	spec.RendererConfig = solarv1alpha1.RendererConfig{
		Type: solarv1alpha1.RendererConfigTypeRelease,
		ReleaseConfig: solarv1alpha1.ReleaseConfig{
			Chart: solarv1alpha1.ChartConfig{
				Name:        chartName,
				Description: fmt.Sprintf("Release of %s", res.Spec.ComponentVersionRef.Name),
				Version:     tag,
				AppVersion:  tag,
			},
			Input: solarv1alpha1.ReleaseInput{
				Component:  solarv1alpha1.ReleaseComponent{Name: cv.Spec.ComponentRef.Name},
				Resources:  cv.Spec.Resources,
				Entrypoint: cv.Spec.Entrypoint,
			},
			TargetNamespace: res.Spec.TargetNamespace,
			Values:          res.Spec.Values,
		},
	}
	spec.Repository = repo
	spec.Tag = tag
	spec.FailedJobTTL = res.Spec.FailedJobTTL

	return spec, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Release{}).
		Complete(r)
}
