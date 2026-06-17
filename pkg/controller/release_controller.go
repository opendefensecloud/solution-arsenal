// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=referencegrants,verbs=get;list;watch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile validates the Release by resolving its ComponentVersion reference and
// manages deletion-protection finalizers on that ComponentVersion.
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

	// Handle deletion: remove componentVersionRefFinalizer from CV if no other Release references it.
	if !res.DeletionTimestamp.IsZero() {
		cvNamespace := res.Namespace
		if res.Spec.ComponentVersionNamespace != "" {
			cvNamespace = res.Spec.ComponentVersionNamespace
		}

		if res.Spec.ComponentVersionRef.Name != "" {
			cv := &solarv1alpha1.ComponentVersion{}
			if err := r.Get(ctx, types.NamespacedName{Name: res.Spec.ComponentVersionRef.Name, Namespace: cvNamespace}, cv); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrlResult, errLogAndWrap(log, err, "failed to get ComponentVersion for finalizer cleanup")
				}
			} else if err := r.removeComponentVersionRefFinalizer(ctx, res, cv); err != nil {
				return ctrlResult, err
			}
		}

		if slices.Contains(res.Finalizers, releaseFinalizer) {
			latest := &solarv1alpha1.Release{}
			if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to get latest Release for finalizer removal")
			}
			original := latest.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool { return s == releaseFinalizer })
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer from Release")
			}
		}

		return ctrlResult, nil
	}

	// Ensure self-finalizer exists before any other work.
	if !slices.Contains(res.Finalizers, releaseFinalizer) {
		latest := &solarv1alpha1.Release{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to get latest Release for finalizer addition")
		}
		original := latest.DeepCopy()
		latest.Finalizers = append(latest.Finalizers, releaseFinalizer)
		if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer to Release")
		}
	}

	cvNamespace := res.Namespace
	if res.Spec.ComponentVersionNamespace != "" {
		cvNamespace = res.Spec.ComponentVersionNamespace
	}

	// For cross-namespace references, verify a ReferenceGrant permits it.
	if cvNamespace != res.Namespace {
		granted, err := r.componentVersionGranted(ctx, res, cvNamespace)
		if err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to check ReferenceGrant for cross-namespace ComponentVersion")
		}
		if !granted {
			changed := apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeComponentVersionResolved,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: res.Generation,
				Reason:             "NotGranted",
				Message:            "no ReferenceGrant permits access to ComponentVersion in namespace " + cvNamespace,
			})
			if changed {
				if err := r.Status().Update(ctx, res); err != nil {
					return ctrlResult, errLogAndWrap(log, err, "failed to update status")
				}
			}

			return ctrlResult, nil
		}
	}

	// Resolve ComponentVersion
	cvRef := types.NamespacedName{
		Name:      res.Spec.ComponentVersionRef.Name,
		Namespace: cvNamespace,
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

	// Protect ComponentVersion from deletion while this Release references it.
	if !slices.Contains(cv.Finalizers, componentVersionRefFinalizer) {
		latest := cv.DeepCopy()
		latest.Finalizers = append(latest.Finalizers, componentVersionRefFinalizer)
		if err := r.Patch(ctx, latest, client.MergeFrom(cv)); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to add protection finalizer to ComponentVersion")
		}
	}

	// ComponentVersion found — set resolved condition and effective unique name.
	uname := effectiveUniqueName(res, cv)

	condChanged := apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeComponentVersionResolved,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: res.Generation,
		Reason:             "Resolved",
		Message:            "ComponentVersion resolved: " + cv.Name,
	})
	nameChanged := res.Status.EffectiveUniqueName != uname
	if condChanged || nameChanged {
		res.Status.EffectiveUniqueName = uname
		if err := r.Status().Update(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to update status")
		}
	}

	return ctrlResult, nil
}

// removeComponentVersionRefFinalizer removes componentVersionRefFinalizer from cv when no other
// active Release still references it (excluding the Release that is currently being deleted).
func (r *ReleaseReconciler) removeComponentVersionRefFinalizer(ctx context.Context, deletingRelease *solarv1alpha1.Release, cv *solarv1alpha1.ComponentVersion) error {
	if !slices.Contains(cv.Finalizers, componentVersionRefFinalizer) {
		return nil
	}

	refKey := cv.Namespace + "/" + cv.Name
	releaseList := &solarv1alpha1.ReleaseList{}
	if err := r.List(ctx, releaseList, client.MatchingFields{indexReleaseByCVRef: refKey}); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to list Releases for ComponentVersion finalizer check")
	}

	for _, rel := range releaseList.Items {
		if rel.Name == deletingRelease.Name && rel.Namespace == deletingRelease.Namespace {
			continue
		}
		if !rel.DeletionTimestamp.IsZero() {
			continue
		}

		return nil // another active Release still references this ComponentVersion
	}

	freshCV := &solarv1alpha1.ComponentVersion{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(cv), freshCV); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to get latest ComponentVersion for finalizer removal")
	}
	original := freshCV.DeepCopy()
	freshCV.Finalizers = slices.DeleteFunc(freshCV.Finalizers, func(s string) bool { return s == componentVersionRefFinalizer })
	if err := r.Patch(ctx, freshCV, client.MergeFrom(original)); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to remove protection finalizer from ComponentVersion")
	}

	ctrl.LoggerFrom(ctx).V(1).Info("Removed protection finalizer from ComponentVersion", "componentversion", cv.Name)

	return nil
}

// componentVersionGranted returns true if a ReferenceGrant in cvNamespace permits
// the given Release to reference a ComponentVersion there.
func (r *ReleaseReconciler) componentVersionGranted(ctx context.Context, release *solarv1alpha1.Release, cvNamespace string) (bool, error) {
	grantList := &solarv1alpha1.ReferenceGrantList{}
	if err := r.List(ctx, grantList, client.InNamespace(cvNamespace)); err != nil {
		return false, err
	}
	for i := range grantList.Items {
		if grantPermitsComponentVersionAccess(&grantList.Items[i], release.Namespace) {
			return true, nil
		}
	}

	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Release{}).
		Watches(
			&solarv1alpha1.ComponentVersion{},
			handler.EnqueueRequestsFromMapFunc(r.mapComponentVersionToReleases),
		).
		Watches(
			&solarv1alpha1.ReferenceGrant{},
			handler.EnqueueRequestsFromMapFunc(r.mapReferenceGrantToReleases),
		).
		Complete(r)
}

// mapComponentVersionToReleases enqueues all Releases that reference this ComponentVersion.
func (r *ReleaseReconciler) mapComponentVersionToReleases(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	refKey := obj.GetNamespace() + "/" + obj.GetName()
	releaseList := &solarv1alpha1.ReleaseList{}
	if err := r.List(ctx, releaseList, client.MatchingFields{indexReleaseByCVRef: refKey}); err != nil {
		log.Error(err, "failed to list Releases for ComponentVersion mapping")

		return nil
	}

	requests := make([]reconcile.Request, len(releaseList.Items))
	for i := range releaseList.Items {
		requests[i] = reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&releaseList.Items[i])}
	}

	return requests
}

// mapReferenceGrantToReleases enqueues Releases whose cross-namespace ComponentVersion
// reference is covered by the changed ReferenceGrant.
func (r *ReleaseReconciler) mapReferenceGrantToReleases(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	grant, ok := obj.(*solarv1alpha1.ReferenceGrant)
	if !ok {
		return nil
	}

	if !grantsComponentVersionResource(grant) {
		return nil
	}

	var requests []reconcile.Request

	for _, from := range grant.Spec.From {
		if from.Kind != "Release" || from.Group != solarGroup {
			continue
		}
		releaseList := &solarv1alpha1.ReleaseList{}
		if err := r.List(ctx, releaseList, client.InNamespace(from.Namespace)); err != nil {
			log.Error(err, "failed to list Releases for ReferenceGrant mapping", "namespace", from.Namespace)

			continue
		}
		for _, rel := range releaseList.Items {
			if rel.Spec.ComponentVersionNamespace == grant.Namespace {
				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&rel),
				})
			}
		}
	}

	return requests
}
