// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
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

		return ctrl.Result{}, fmt.Errorf("failed to get ReleaseBinding: %w", err)
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
						return ctrl.Result{}, fmt.Errorf("failed to check owner Profile during ReleaseBinding deletion: %w", err)
					}
				} else if slices.Contains(ownerProfile.Finalizers, profileFinalizer) {
					profileOwnerManaging = true
				}
			}
			if !profileOwnerManaging {
				release := &solarv1alpha1.Release{}
				if err := r.Get(ctx, types.NamespacedName{Name: rb.Spec.ReleaseRef.Name, Namespace: rb.Namespace}, release); err != nil {
					if !apierrors.IsNotFound(err) {
						return ctrl.Result{}, fmt.Errorf("failed to get Release for finalizer cleanup: %w", err)
					}
				} else if err := r.removeReleaseRefFinalizer(ctx, rb, release); err != nil {
					return ctrl.Result{}, err
				}
			}
		}

		if slices.Contains(rb.Finalizers, releaseBindingFinalizer) {
			latest := &solarv1alpha1.ReleaseBinding{}
			if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get latest ReleaseBinding for finalizer removal: %w", err)
			}
			original := latest.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool { return s == releaseBindingFinalizer })
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from ReleaseBinding: %w", err)
			}
		}

		return ctrl.Result{}, nil
	}

	// Ensure self-finalizer exists.
	if !slices.Contains(rb.Finalizers, releaseBindingFinalizer) {
		latest := &solarv1alpha1.ReleaseBinding{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get latest ReleaseBinding for finalizer addition: %w", err)
		}
		if !slices.Contains(latest.Finalizers, releaseBindingFinalizer) {
			original := latest.DeepCopy()
			latest.Finalizers = append(latest.Finalizers, releaseBindingFinalizer)
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to add finalizer to ReleaseBinding: %w", err)
			}
		}
	}

	// Protect the referenced Release from deletion.
	if rb.Spec.ReleaseRef.Name != "" {
		release := &solarv1alpha1.Release{}
		if err := r.Get(ctx, types.NamespacedName{Name: rb.Spec.ReleaseRef.Name, Namespace: rb.Namespace}, release); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("failed to get Release for protection finalizer: %w", err)
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
					return ctrl.Result{}, fmt.Errorf("failed to check owner Profile for deletion status: %w", err)
				}
			}
			freshRelease := &solarv1alpha1.Release{}
			if err := r.Get(ctx, types.NamespacedName{Name: rb.Spec.ReleaseRef.Name, Namespace: rb.Namespace}, freshRelease); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, fmt.Errorf("failed to get latest Release for finalizer addition: %w", err)
				}

				return ctrl.Result{}, nil
			}
			if !slices.Contains(freshRelease.Finalizers, releaseRefFinalizer) {
				original := freshRelease.DeepCopy()
				freshRelease.Finalizers = append(freshRelease.Finalizers, releaseRefFinalizer)
				if err := r.Patch(ctx, freshRelease, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to add protection finalizer to Release: %w", err)
				}
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
	referenced, err := releaseReferenced(ctx, r.Client, release, nil, deletingRB)
	if err != nil || referenced {
		return err
	}

	return dropFinalizer(ctx, r.Client, release, releaseRefFinalizer)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReleaseBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.ReleaseBinding{}).
		Complete(r)
}
