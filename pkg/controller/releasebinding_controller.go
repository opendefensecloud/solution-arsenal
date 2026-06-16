// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// ReleaseBindingReconciler manages the deletion-protection finalizer on the Release referenced
// by each ReleaseBinding. This covers both Profile-created and manually created ReleaseBindings.
type ReleaseBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releasebindings,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releasebindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=profiles,verbs=get;list;watch

func (r *ReleaseBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("ReleaseBinding is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	rb := &solarv1alpha1.ReleaseBinding{}
	if err := r.Get(ctx, req.NamespacedName, rb); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get ReleaseBinding")
	}

	// Handle deletion: remove releaseRefFinalizer from Release if no other active referencer exists.
	if !rb.DeletionTimestamp.IsZero() {
		if rb.Spec.ReleaseRef.Name != "" {
			// If owned by a Profile that is still managing cleanup (profileFinalizer present),
			// defer release-ref removal to the Profile controller.
			profileOwnerManaging := false
			if ownerRef := metav1.GetControllerOf(rb); ownerRef != nil && ownerRef.Kind == "Profile" && ownerRef.APIVersion == solarv1alpha1.SchemeGroupVersion.String() {
				ownerProfile := &solarv1alpha1.Profile{}
				if err := r.Get(ctx, types.NamespacedName{Name: ownerRef.Name, Namespace: rb.Namespace}, ownerProfile); err != nil {
					if !apierrors.IsNotFound(err) {
						return ctrl.Result{}, errLogAndWrap(log, err, "failed to check owner Profile during ReleaseBinding deletion")
					}
				} else if slices.Contains(ownerProfile.Finalizers, profileFinalizer) {
					profileOwnerManaging = true
				}
			}
			if !profileOwnerManaging {
				release := &solarv1alpha1.Release{}
				if err := r.Get(ctx, types.NamespacedName{Name: rb.Spec.ReleaseRef.Name, Namespace: rb.Namespace}, release); err != nil {
					if !apierrors.IsNotFound(err) {
						return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Release for finalizer cleanup")
					}
				} else if err := r.removeReleaseRefFinalizer(ctx, rb, release); err != nil {
					return ctrl.Result{}, err
				}
			}
		}

		if slices.Contains(rb.Finalizers, releaseBindingFinalizer) {
			latest := &solarv1alpha1.ReleaseBinding{}
			if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest ReleaseBinding for finalizer removal")
			}
			original := latest.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool { return s == releaseBindingFinalizer })
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to remove finalizer from ReleaseBinding")
			}
		}

		return ctrl.Result{}, nil
	}

	// Ensure self-finalizer exists.
	if !slices.Contains(rb.Finalizers, releaseBindingFinalizer) {
		latest := &solarv1alpha1.ReleaseBinding{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest ReleaseBinding for finalizer addition")
		}
		original := latest.DeepCopy()
		latest.Finalizers = append(latest.Finalizers, releaseBindingFinalizer)
		if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to add finalizer to ReleaseBinding")
		}

		return ctrl.Result{}, nil
	}

	// Protect the referenced Release from deletion.
	if rb.Spec.ReleaseRef.Name != "" {
		release := &solarv1alpha1.Release{}
		if err := r.Get(ctx, types.NamespacedName{Name: rb.Spec.ReleaseRef.Name, Namespace: rb.Namespace}, release); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Release for protection finalizer")
			}
		} else if !slices.Contains(release.Finalizers, releaseRefFinalizer) {
			// If this ReleaseBinding is owned by a Profile that is gone or being deleted, skip
			// adding the protection finalizer — the binding will be GC'd alongside the Profile,
			// and the Profile controller already handled removal of releaseRefFinalizer.
			if ownerRef := metav1.GetControllerOf(rb); ownerRef != nil && ownerRef.Kind == "Profile" && ownerRef.APIVersion == solarv1alpha1.SchemeGroupVersion.String() {
				ownerProfile := &solarv1alpha1.Profile{}
				err := r.Get(ctx, types.NamespacedName{Name: ownerRef.Name, Namespace: rb.Namespace}, ownerProfile)
				if apierrors.IsNotFound(err) || (err == nil && !ownerProfile.DeletionTimestamp.IsZero()) {
					return ctrl.Result{}, nil
				}
				if err != nil {
					return ctrl.Result{}, errLogAndWrap(log, err, "failed to check owner Profile for deletion status")
				}
			}
			latest := release.DeepCopy()
			latest.Finalizers = append(latest.Finalizers, releaseRefFinalizer)
			if err := r.Patch(ctx, latest, client.MergeFrom(release)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to add protection finalizer to Release")
			}
		}
	}

	return ctrl.Result{}, nil
}

// removeReleaseRefFinalizer removes releaseRefFinalizer from release when no other active
// Profile or ReleaseBinding (excluding the deleting ReleaseBinding) still references it.
func (r *ReleaseBindingReconciler) removeReleaseRefFinalizer(ctx context.Context, deletingRB *solarv1alpha1.ReleaseBinding, release *solarv1alpha1.Release) error {
	if !slices.Contains(release.Finalizers, releaseRefFinalizer) {
		return nil
	}

	// Count active Profiles in the same namespace referencing this Release.
	profileList := &solarv1alpha1.ProfileList{}
	if err := r.List(ctx, profileList,
		client.InNamespace(release.Namespace),
		client.MatchingFields{indexProfileByReleaseName: release.Name},
	); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to list Profiles for Release finalizer check")
	}

	for _, p := range profileList.Items {
		if !p.DeletionTimestamp.IsZero() {
			// Profile is deleting. If profile-finalizer is still present, the Profile controller
			// is managing cleanup and will remove release-ref once all owned bindings are gone.
			if slices.Contains(p.Finalizers, profileFinalizer) {
				return nil
			}

			continue
		}

		return nil
	}

	// Count active ReleaseBindings (excluding self and those owned by deleting Profiles) referencing this Release.
	bindingList := &solarv1alpha1.ReleaseBindingList{}
	if err := r.List(ctx, bindingList,
		client.InNamespace(release.Namespace),
		client.MatchingFields{indexReleaseBindingReleaseName: release.Name},
	); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to list ReleaseBindings for Release finalizer check")
	}

	// Cache owner Profile lookups to avoid repeated Gets when bindings share the same owner.
	ownerProfileCache := map[string]*solarv1alpha1.Profile{} // nil = gone or deleting
	for _, rb := range bindingList.Items {
		if rb.Name == deletingRB.Name || !rb.DeletionTimestamp.IsZero() {
			continue
		}

		ownerRef := metav1.GetControllerOf(&rb)
		if ownerRef == nil || ownerRef.Kind != "Profile" || ownerRef.APIVersion != solarv1alpha1.SchemeGroupVersion.String() {
			return nil // active binding without a Profile owner → Release still referenced
		}

		cacheKey := rb.Namespace + "/" + ownerRef.Name
		if _, seen := ownerProfileCache[cacheKey]; !seen {
			op := &solarv1alpha1.Profile{}
			err := r.Get(ctx, types.NamespacedName{Name: ownerRef.Name, Namespace: rb.Namespace}, op)
			switch {
			case apierrors.IsNotFound(err) || (err == nil && !op.DeletionTimestamp.IsZero()):
				ownerProfileCache[cacheKey] = nil // gone or deleting
			case err != nil:
				return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to check owner Profile for Release finalizer check")
			default:
				ownerProfileCache[cacheKey] = op
			}
		}

		if ownerProfileCache[cacheKey] != nil {
			return nil // binding is actively owned → Release still referenced
		}
	}

	freshRelease := &solarv1alpha1.Release{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(release), freshRelease); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to get latest Release for finalizer removal")
	}
	original := freshRelease.DeepCopy()
	freshRelease.Finalizers = slices.DeleteFunc(freshRelease.Finalizers, func(s string) bool { return s == releaseRefFinalizer })
	if err := r.Patch(ctx, freshRelease, client.MergeFrom(original)); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to remove protection finalizer from Release")
	}

	ctrl.LoggerFrom(ctx).V(1).Info("Removed protection finalizer from Release", "release", release.Name)

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReleaseBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.ReleaseBinding{}).
		Complete(r)
}
