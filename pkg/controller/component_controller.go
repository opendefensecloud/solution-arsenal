// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"slices"
	"time"

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

	// Re-read the Component straight from the API server. This serves two
	// purposes: it is the authoritative check for componentRefFinalizer
	// having been removed out-of-band (e.g. a manual kubectl edit) since the
	// cached Get above, so we don't attempt to delete a Component that no
	// longer carries our finalizer; and it gives the fresh UID that feeds
	// the delete precondition below, so the delete targets the exact object
	// version this reconcile evaluated.
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
	//
	// A CV created between the checks above and the strip below briefly loses
	// its parent: the Component is removed under a live reference and the next
	// discovery event re-creates it via the apiwriter's ensureComponent. That
	// residual is inherent to poll-then-act without cross-resource
	// transactions and is accepted. Re-checking after the delete would not
	// help: at that point the Component is already Terminating, so keeping the
	// finalizer strands it until its last CV disappears, which does not
	// self-heal (finalizers cannot be added to or protect a terminating
	// object from an already-issued delete).
	if comp.DeletionTimestamp.IsZero() {
		if err := client.IgnoreNotFound(r.Delete(ctx, comp, client.Preconditions{UID: &comp.UID})); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete unreferenced Component")
		}
		log.V(1).Info("Deleted unreferenced Component", "component", comp.Name)
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
			return ctrl.Result{RequeueAfter: time.Second}, nil
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

// directListPageSize bounds each page of the uncached ComponentVersion list on
// the GC path; custom field indexes are cache-only, so the filter runs in code.
const directListPageSize = 500

// countLiveCVsDirect counts non-terminating ComponentVersions referencing comp
// by listing straight from the API server, page by page.
func (r *ComponentReconciler) countLiveCVsDirect(ctx context.Context, comp *solarv1alpha1.Component) (int, error) {
	live := 0
	continueToken := ""
	for {
		cvList := &solarv1alpha1.ComponentVersionList{}
		if err := r.APIReader.List(ctx, cvList,
			client.InNamespace(comp.Namespace),
			client.Limit(directListPageSize),
			client.Continue(continueToken),
		); err != nil {
			return 0, errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to list ComponentVersions from API server")
		}

		for _, cv := range cvList.Items {
			if cv.Spec.ComponentRef.Name == comp.Name && cv.DeletionTimestamp.IsZero() {
				live++
			}
		}

		continueToken = cvList.Continue
		if continueToken == "" {
			return live, nil
		}
	}
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
