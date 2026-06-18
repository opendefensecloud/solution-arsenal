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

// ComponentVersionReconciler manages the deletion-protection finalizer on the Component
// referenced by each ComponentVersion, preventing Component deletion while ComponentVersions exist.
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
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=components,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=components/finalizers,verbs=update

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

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get ComponentVersion")
	}

	// Handle deletion: remove componentRefFinalizer from Component if no other CV references it.
	if !cv.DeletionTimestamp.IsZero() {
		if cv.Spec.ComponentRef.Name != "" {
			comp := &solarv1alpha1.Component{}
			if err := r.Get(ctx, types.NamespacedName{Name: cv.Spec.ComponentRef.Name, Namespace: cv.Namespace}, comp); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Component for finalizer cleanup")
				}
			} else if err := r.removeComponentRefFinalizer(ctx, cv, comp); err != nil {
				return ctrl.Result{}, err
			}
		}

		if slices.Contains(cv.Finalizers, componentVersionFinalizer) {
			latest := &solarv1alpha1.ComponentVersion{}
			if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest ComponentVersion for finalizer removal")
			}
			original := latest.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool { return s == componentVersionFinalizer })
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to remove finalizer from ComponentVersion")
			}
		}

		return ctrl.Result{}, nil
	}

	// Ensure self-finalizer exists.
	if !slices.Contains(cv.Finalizers, componentVersionFinalizer) {
		latest := &solarv1alpha1.ComponentVersion{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest ComponentVersion for finalizer addition")
		}
		if !slices.Contains(latest.Finalizers, componentVersionFinalizer) {
			original := latest.DeepCopy()
			latest.Finalizers = append(latest.Finalizers, componentVersionFinalizer)
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to add finalizer to ComponentVersion")
			}
		}
	}

	// Protect the referenced Component from deletion.
	if cv.Spec.ComponentRef.Name != "" {
		comp := &solarv1alpha1.Component{}
		if err := r.Get(ctx, types.NamespacedName{Name: cv.Spec.ComponentRef.Name, Namespace: cv.Namespace}, comp); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Component for protection finalizer")
			}
		} else if !slices.Contains(comp.Finalizers, componentRefFinalizer) {
			latest := comp.DeepCopy()
			latest.Finalizers = append(latest.Finalizers, componentRefFinalizer)
			if err := r.Patch(ctx, latest, client.MergeFrom(comp)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to add protection finalizer to Component")
			}
		}
	}

	return ctrl.Result{}, nil
}

// removeComponentRefFinalizer removes componentRefFinalizer from comp when no other active
// ComponentVersion still references it (excluding the CV currently being deleted).
func (r *ComponentVersionReconciler) removeComponentRefFinalizer(ctx context.Context, deletingCV *solarv1alpha1.ComponentVersion, comp *solarv1alpha1.Component) error {
	if !slices.Contains(comp.Finalizers, componentRefFinalizer) {
		return nil
	}

	cvList := &solarv1alpha1.ComponentVersionList{}
	if err := r.List(ctx, cvList,
		client.InNamespace(comp.Namespace),
		client.MatchingFields{indexCVByComponentName: comp.Name},
	); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to list ComponentVersions for Component finalizer check")
	}

	for _, cv := range cvList.Items {
		if cv.Name == deletingCV.Name {
			continue
		}
		if !cv.DeletionTimestamp.IsZero() {
			continue
		}

		return nil // another active ComponentVersion still references this Component
	}

	freshComp := &solarv1alpha1.Component{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(comp), freshComp); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to get latest Component for finalizer removal")
	}
	original := freshComp.DeepCopy()
	freshComp.Finalizers = slices.DeleteFunc(freshComp.Finalizers, func(s string) bool { return s == componentRefFinalizer })
	if err := r.Patch(ctx, freshComp, client.MergeFrom(original)); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to remove protection finalizer from Component")
	}

	ctrl.LoggerFrom(ctx).V(1).Info("Removed protection finalizer from Component", "component", comp.Name)

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.ComponentVersion{}).
		Complete(r)
}
