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
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=referencegrants,verbs=get;list;watch
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

	releaseList := &solarv1alpha1.ReleaseList{}
	if err := r.List(ctx, releaseList); err != nil {
		log.Error(err, "failed to list Releases for ComponentVersion mapping")

		return nil
	}

	var requests []reconcile.Request

	for _, rel := range releaseList.Items {
		cvNs := rel.Namespace
		if rel.Spec.ComponentVersionNamespace != "" {
			cvNs = rel.Spec.ComponentVersionNamespace
		}
		if rel.Spec.ComponentVersionRef.Name == obj.GetName() && cvNs == obj.GetNamespace() {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&rel),
			})
		}
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
