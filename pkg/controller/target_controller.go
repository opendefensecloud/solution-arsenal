// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	targetFinalizer = "solar.opendefense.cloud/target-finalizer"
)

type TargetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets,verbs=get;list;watch;create;update;patch;delete

// Reconcile moves the current state of the cluster closer to the desired state
func (r *TargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("Target is being reconciled", "req", req)

	// Fetch target
	target := &solarv1alpha1.Target{}
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrlResult, nil
		}

		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Handle deletion
	if !target.DeletionTimestamp.IsZero() {
		log.V(1).Info("Target is being deleted")
		r.Recorder.Eventf(target, nil, corev1.EventTypeWarning, "Deleting", "Reconcile", "Target is being deleted, cleaning up HydratedTarget")

		// Delete HydratedTarget
		if err := r.Delete(ctx, &solarv1alpha1.HydratedTarget{ObjectMeta: metav1.ObjectMeta{Namespace: target.Namespace, Name: target.Name}}); err != nil && !apierrors.IsNotFound(err) {
			return ctrlResult, errLogAndWrap(log, err, "failed to delete HydratedTarget")
		}

		// Remove finalizer
		if slices.Contains(target.Finalizers, targetFinalizer) {
			// Re-fetch latest version to avoid conflicts
			latest := &solarv1alpha1.Target{}
			if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to get latest Target for finalizer removal")
			}
			log.V(1).Info("Removing finalizer from Target")
			original := latest.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool {
				return s == targetFinalizer
			})

			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer from Target")
			}
		}

		return ctrlResult, nil
	}

	// Set finalizer if not set already and not currently deleting
	if target.DeletionTimestamp.IsZero() && !slices.Contains(target.Finalizers, targetFinalizer) {
		log.V(1).Info("Target does not have finalizer set, adding finalizer")
		latest := &solarv1alpha1.Target{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to get latest Target for finalizer addition")
		}
		original := latest.DeepCopy()
		latest.Finalizers = append(latest.Finalizers, targetFinalizer)
		if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer to Target")
		}

		return ctrlResult, nil
	}

	// Check if hydrated target exists, if not create and make sure to SetControllerReference...
	hydratedTarget := &solarv1alpha1.HydratedTarget{}
	err := r.Get(ctx, req.NamespacedName, hydratedTarget)

	if err != nil && !apierrors.IsNotFound(err) {
		return ctrlResult, errLogAndWrap(log, err, "failed to get HydratedTarget")
	}

	// Create HydratedTarget if not exists or update/override spec
	if apierrors.IsNotFound(err) {
		log.V(1).Info("Creating HydratedTarget for Target", "target", req.NamespacedName)
		// FIXME: Just copy over releases and userdata (for now)
		hydratedTarget = &solarv1alpha1.HydratedTarget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      target.Name,
				Namespace: target.Namespace,
			},
			Spec: solarv1alpha1.HydratedTargetSpec{
				Releases: target.Spec.Releases,
				Userdata: target.Spec.Userdata,
			},
		}
		if err := ctrl.SetControllerReference(target, hydratedTarget, r.Scheme); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to set controller reference on HydratedTarget")
		}
		if err := r.Create(ctx, hydratedTarget); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to create HydratedTarget")
		}
		r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Created", "Create", "Created HydratedTarget %s/%s", hydratedTarget.Namespace, hydratedTarget.Name)

		return ctrlResult, nil
	}

	// Update if out of sync
	// re-fetch target and hydratedTarget to avoid conflicts
	hydratedTarget = &solarv1alpha1.HydratedTarget{}
	if err := r.Get(ctx, req.NamespacedName, hydratedTarget); err != nil {
		return ctrlResult, errLogAndWrap(log, err, "failed to re-fetch HydratedTarget for update check")
	}
	target = &solarv1alpha1.Target{}
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		return ctrlResult, errLogAndWrap(log, err, "failed to re-fetch Target for update check")
	}

	original := hydratedTarget.DeepCopy()

	if !apiequality.Semantic.DeepEqual(hydratedTarget.Spec, target.Spec) {
		hydratedTarget.Spec.Releases = target.Spec.Releases
		hydratedTarget.Spec.Userdata = target.Spec.Userdata
		log.V(1).Info("Updating HydratedTarget for Target", "target", req.NamespacedName)
		if err := r.Patch(ctx, hydratedTarget, client.MergeFrom(original)); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to update HydratedTarget")
		}
		r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Updated", "Update", "Updated HydratedTarget %s/%s", hydratedTarget.Namespace, hydratedTarget.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Target{}).
		Owns(&solarv1alpha1.HydratedTarget{}).
		Complete(r)
}
