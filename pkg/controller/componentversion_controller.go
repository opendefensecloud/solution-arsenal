// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// ComponentVersionReconciler manages the componentVersionFinalizer on each
// ComponentVersion so deletion is observable by other controllers. The
// componentRefFinalizer on the parent Component is owned by ComponentReconciler.
type ComponentVersionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions/finalizers,verbs=update

func (r *ComponentVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("ComponentVersion is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	cv := &solarv1alpha1.ComponentVersion{}
	if err := r.Get(ctx, req.NamespacedName, cv); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get ComponentVersion: %w", err)
	}

	// Handle deletion: remove the self-finalizer; the parent Component's
	// protection finalizer and GC are handled by ComponentReconciler.
	if !cv.DeletionTimestamp.IsZero() {
		if slices.Contains(cv.Finalizers, componentVersionFinalizer) {
			latest := &solarv1alpha1.ComponentVersion{}
			if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get latest ComponentVersion for finalizer removal: %w", err)
			}
			original := latest.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool { return s == componentVersionFinalizer })
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from ComponentVersion: %w", err)
			}
		}

		return ctrl.Result{}, nil
	}

	// Ensure self-finalizer exists.
	if !slices.Contains(cv.Finalizers, componentVersionFinalizer) {
		latest := &solarv1alpha1.ComponentVersion{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get latest ComponentVersion for finalizer addition: %w", err)
		}
		if !slices.Contains(latest.Finalizers, componentVersionFinalizer) {
			original := latest.DeepCopy()
			latest.Finalizers = append(latest.Finalizers, componentVersionFinalizer)
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to add finalizer to ComponentVersion: %w", err)
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.ComponentVersion{}).
		Complete(r)
}
