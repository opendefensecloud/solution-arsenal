// Copyright 2026 BWI GmbH and Artefact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"slices"

	"go.opendefense.cloud/solar/api/solar/v1alpha1"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	discoveryFinalizer = "arc.opendefense.cloud/discovery-finalizer"
)

// DiscoveryReconciler reconciles a Discovery object
type DiscoveryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=arc.opendefense.cloud,resources=discoveries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=arc.opendefense.cloud,resources=discoveries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=arc.opendefense.cloud,resources=discoveries/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile moves the current state of the cluster closer to the desired state
func (r *DiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	// Fetch the Order instance
	res := &v1alpha1.Discovery{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrlResult, nil
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Handle deletion: cleanup artifact workflows, then remove finalizer
	if !res.DeletionTimestamp.IsZero() {
		log.V(1).Info("Discovery is being deleted")
		r.Recorder.Event(res, corev1.EventTypeWarning, "Deleting", "Discovery is being deleted, cleaning up worker")

		// Cleanup worker, if exists
		if err := r.Delete(ctx, &corev1.Pod{ObjectMeta: v1.ObjectMeta{Namespace: res.Namespace, Name: res.Name}}); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "pod deletion failed")
		}

		// Remove finalizer
		if slices.Contains(res.Finalizers, discoveryFinalizer) {
			log.V(1).Info("Removing finalizer from resource")
			res.Finalizers = slices.DeleteFunc(res.Finalizers, func(f string) bool {
				return f == discoveryFinalizer
			})
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer")
			}
		}
	}

	// Add finalizer if not present and not deleting
	if res.DeletionTimestamp.IsZero() {
		if !slices.Contains(res.Finalizers, discoveryFinalizer) {
			log.V(1).Info("Adding finalizer to resource")
			res.Finalizers = append(res.Finalizers, discoveryFinalizer)
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer")
			}
			// Return without requeue; the Update event will trigger reconciliation again
			return ctrlResult, nil
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Discovery{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
