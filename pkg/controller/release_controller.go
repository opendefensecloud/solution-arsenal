// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

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

// SetupWithManager sets up the controller with the Manager.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Release{}).
		Complete(r)
}
