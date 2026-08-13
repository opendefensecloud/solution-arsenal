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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// ComponentReconciler owns the Component lifecycle: it keeps the
// componentRefFinalizer in sync with live ComponentVersions (deletion
// protection) and garbage-collects a Component once its last live
// ComponentVersion is gone. Reconciles are serialized per Component, so the
// count-then-act decision cannot interleave with itself for the same object.
type ComponentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// APIReader reads directly from the API server, bypassing the informer
	// cache. Used as the authoritative zero-live-CV check before deleting,
	// because the cached index can lag behind a just-created CV.
	APIReader client.Reader
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=components,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=components/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch

func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("Component is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	comp := &solarv1alpha1.Component{}
	if err := r.Get(ctx, req.NamespacedName, comp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Component")
	}

	live, err := r.countLiveCVsCached(ctx, comp)
	if err != nil {
		return ctrl.Result{}, err
	}

	if live > 0 {
		return ctrl.Result{}, r.ensureProtectionFinalizer(ctx, comp)
	}

	// live == 0: only GC Components that carry the protection finalizer. The
	// finalizer means a live CV was observed at some point; without it the
	// Component was either just created by discovery (its first CV not yet
	// visible) or created manually, and neither must be deleted here.
	if !slices.Contains(comp.Finalizers, componentRefFinalizer) {
		return ctrl.Result{}, nil
	}

	// Re-read the Component straight from the API server so a Component
	// whose finalizer was already removed elsewhere (the coexisting
	// ComponentVersionReconciler also manages componentRefFinalizer removal
	// in this one-commit window) is left alone instead of attempting to
	// delete it here too. This does not by itself close the delete-then-
	// recreate race further down: the authoritative live-CV check below,
	// and its post-delete repeat, are what actually protect a Component
	// against a CV that shows up while GC is in flight.
	fresh := &solarv1alpha1.Component{}
	if err := r.APIReader.Get(ctx, req.NamespacedName, fresh); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Component from API server")
	}
	if !slices.Contains(fresh.Finalizers, componentRefFinalizer) {
		return ctrl.Result{}, nil
	}
	comp = fresh

	// Authoritative re-check straight from the API server; custom field
	// indexes are cache-only, so filter in code.
	liveDirect, err := r.countLiveCVsDirect(ctx, comp)
	if err != nil {
		return ctrl.Result{}, err
	}
	if liveDirect > 0 {
		// Cache lagged behind a just-created CV; its watch event re-enqueues us.
		return ctrl.Result{}, nil
	}

	// Delete first (the finalizer holds the object in Terminating), then strip
	// the finalizer. If we crash in between, a later reconcile finds
	// live == 0 with the finalizer present and finishes the removal.
	if comp.DeletionTimestamp.IsZero() {
		if err := client.IgnoreNotFound(r.Delete(ctx, comp, client.Preconditions{UID: &comp.UID})); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete unreferenced Component")
		}
		log.V(1).Info("Deleted unreferenced Component", "component", comp.Name)
	}

	// Re-check once more before stripping the finalizer: a CV can be created
	// in the window between the delete call above and this point. If one now
	// exists, leave the finalizer in place — the Component sits Terminating-
	// and-protected rather than being fully removed out from under a live
	// reference, and the CV's watch event re-enqueues us to finish the
	// removal once it's genuinely gone.
	liveAfterDelete, err := r.countLiveCVsDirect(ctx, comp)
	if err != nil {
		return ctrl.Result{}, err
	}
	if liveAfterDelete > 0 {
		return ctrl.Result{}, nil
	}

	latest := &solarv1alpha1.Component{}
	if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest Component for finalizer removal")
	}
	original := latest.DeepCopy()
	latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool { return s == componentRefFinalizer })
	if err := r.Patch(ctx, latest, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		if apierrors.IsConflict(err) {
			// Something changed concurrently; re-evaluate from scratch.
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to remove protection finalizer from Component")
	}

	return ctrl.Result{}, nil
}

// ensureProtectionFinalizer adds componentRefFinalizer to comp unless it is
// already present or the Component is terminating (the API server rejects
// adding finalizers to terminating objects).
func (r *ComponentReconciler) ensureProtectionFinalizer(ctx context.Context, comp *solarv1alpha1.Component) error {
	if !comp.DeletionTimestamp.IsZero() || slices.Contains(comp.Finalizers, componentRefFinalizer) {
		return nil
	}

	original := comp.DeepCopy()
	comp.Finalizers = append(comp.Finalizers, componentRefFinalizer)
	if err := r.Patch(ctx, comp, client.MergeFrom(original)); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to add protection finalizer to Component")
	}

	return nil
}

// countLiveCVsCached counts non-terminating ComponentVersions referencing comp
// using the indexed informer cache.
func (r *ComponentReconciler) countLiveCVsCached(ctx context.Context, comp *solarv1alpha1.Component) (int, error) {
	cvList := &solarv1alpha1.ComponentVersionList{}
	if err := r.List(ctx, cvList,
		client.InNamespace(comp.Namespace),
		client.MatchingFields{indexCVByComponentName: comp.Name},
	); err != nil {
		return 0, errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to list ComponentVersions for Component")
	}

	live := 0
	for _, cv := range cvList.Items {
		if cv.DeletionTimestamp.IsZero() {
			live++
		}
	}

	return live, nil
}

// countLiveCVsDirect counts non-terminating ComponentVersions referencing comp
// by listing straight from the API server.
func (r *ComponentReconciler) countLiveCVsDirect(ctx context.Context, comp *solarv1alpha1.Component) (int, error) {
	cvList := &solarv1alpha1.ComponentVersionList{}
	if err := r.APIReader.List(ctx, cvList, client.InNamespace(comp.Namespace)); err != nil {
		return 0, errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to list ComponentVersions from API server")
	}

	live := 0
	for _, cv := range cvList.Items {
		if cv.Spec.ComponentRef.Name == comp.Name && cv.DeletionTimestamp.IsZero() {
			live++
		}
	}

	return live, nil
}

// mapCVToComponent maps ComponentVersion events to a reconcile request for the
// parent Component (same namespace, spec.componentRef.name).
func mapCVToComponent(_ context.Context, obj client.Object) []reconcile.Request {
	cv, ok := obj.(*solarv1alpha1.ComponentVersion)
	if !ok || cv.Spec.ComponentRef.Name == "" {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      cv.Spec.ComponentRef.Name,
				Namespace: cv.Namespace,
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Component{}).
		Watches(&solarv1alpha1.ComponentVersion{}, handler.EnqueueRequestsFromMapFunc(mapCVToComponent)).
		Complete(r)
}
