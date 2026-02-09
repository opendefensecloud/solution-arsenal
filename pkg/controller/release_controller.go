// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"slices"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/renderer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	releaseFinalizer = "solar.opendefense.cloud/release-finalizer"
)

// ReleaseReconciler reconciles a Release object
type ReleaseReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	RendererImage   string
	RendererCommand string
	RendererArgs    []string
	PushOptions     renderer.PushOptions
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile moves the current state of the cluster closer to the desired state
//
// Reconciliation Flow:
//
//	Release created
//	    ↓
//	Add finalizer
//	    ↓
//	Check if already succeeded → YES → Return (no-op)
//	    ↓ NO
//	Create/update config secret
//	    ↓
//	Get or create RenderTask
//	    ↓
//	Update release status from RenderTask
//	    ↓
//	RenderTask completed with success?
//	    ├→ YES → Cleanup resources → Return
//	    └→ NO → Still running? → Requeue in 5s

func (r *ReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("Release is being reconciled", "req", req)

	// Fetch the Release instance
	res := &solarv1alpha1.Release{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrlResult, nil
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Handle deletion: cleanup job and secret, then remove finalizer
	if !res.DeletionTimestamp.IsZero() {
		log.V(1).Info("Release is being deleted")
		r.Recorder.Event(res, corev1.EventTypeWarning, "Deleting", "Release is being deleted, cleaning up resources")

		// TODO: delete RenderTask

		return ctrlResult, nil
	}

	// Add finalizer if not present and not deleting
	if res.DeletionTimestamp.IsZero() {
		if !slices.Contains(res.Finalizers, releaseFinalizer) {
			log.V(1).Info("Adding finalizer to resource")
			res.Finalizers = append(res.Finalizers, releaseFinalizer)
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer")
			}
			// Return without requeue; the Update event will trigger reconciliation again
			return ctrlResult, nil
		}
	}

	// Check if release has already completed successfully
	if apimeta.IsStatusConditionTrue(res.Status.Conditions, ConditionTypeJobSucceeded) {
		log.V(1).Info("Release has already completed successfully, no further action needed")
		return ctrlResult, nil
	}

	// TODO: reconcile RenderTask

	return ctrlResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Release{}).
		Owns(&solarv1alpha1.RenderTask{}).
		Complete(r)
}
