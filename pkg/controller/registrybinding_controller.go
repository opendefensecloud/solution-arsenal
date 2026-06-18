// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// RegistryBindingReconciler manages the deletion-protection finalizer on the Registry referenced
// by each RegistryBinding, preventing Registry deletion while RegistryBindings exist.
type RegistryBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registrybindings,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registrybindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registries,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registries/finalizers,verbs=update

func (r *RegistryBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("RegistryBinding is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	rb := &solarv1alpha1.RegistryBinding{}
	if err := r.Get(ctx, req.NamespacedName, rb); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get RegistryBinding")
	}

	// Handle deletion: remove registryRefFinalizer from Registry if no other active referencer exists.
	if !rb.DeletionTimestamp.IsZero() {
		if rb.Spec.RegistryRef.Name != "" {
			registry := &solarv1alpha1.Registry{}
			if err := r.Get(ctx, types.NamespacedName{Name: rb.Spec.RegistryRef.Name, Namespace: rb.Namespace}, registry); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Registry for finalizer cleanup")
				}
			} else if err := r.removeRegistryRefFinalizer(ctx, rb, registry); err != nil {
				return ctrl.Result{}, err
			}
		}

		if slices.Contains(rb.Finalizers, registryBindingFinalizer) {
			latest := &solarv1alpha1.RegistryBinding{}
			if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest RegistryBinding for finalizer removal")
			}
			original := latest.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool { return s == registryBindingFinalizer })
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to remove finalizer from RegistryBinding")
			}
		}

		return ctrl.Result{}, nil
	}

	// Ensure self-finalizer exists.
	if !slices.Contains(rb.Finalizers, registryBindingFinalizer) {
		latest := &solarv1alpha1.RegistryBinding{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest RegistryBinding for finalizer addition")
		}
		if !slices.Contains(latest.Finalizers, registryBindingFinalizer) {
			original := latest.DeepCopy()
			latest.Finalizers = append(latest.Finalizers, registryBindingFinalizer)
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to add finalizer to RegistryBinding")
			}
		}
	}

	// Protect the referenced Registry from deletion.
	if rb.Spec.RegistryRef.Name != "" {
		registry := &solarv1alpha1.Registry{}
		if err := r.Get(ctx, types.NamespacedName{Name: rb.Spec.RegistryRef.Name, Namespace: rb.Namespace}, registry); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Registry for protection finalizer")
			}
		} else if !slices.Contains(registry.Finalizers, registryRefFinalizer) {
			latest := registry.DeepCopy()
			latest.Finalizers = append(latest.Finalizers, registryRefFinalizer)
			if err := r.Patch(ctx, latest, client.MergeFrom(registry)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to add protection finalizer to Registry")
			}
		}
	}

	return ctrl.Result{}, nil
}

// removeRegistryRefFinalizer removes registryRefFinalizer from registry when no other active
// Target or RegistryBinding (excluding the deleting RegistryBinding) still references it.
func (r *RegistryBindingReconciler) removeRegistryRefFinalizer(ctx context.Context, deletingRB *solarv1alpha1.RegistryBinding, registry *solarv1alpha1.Registry) error {
	return removeRegistryRefFinalizer(ctx, r.Client, nil, deletingRB, registry)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RegistryBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.RegistryBinding{}).
		Complete(r)
}
