// Copyright 2026 BWI GmbH and Artefact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TargetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets,verbs=get;list;watch;create;update;patch;delete

// Reconcile moves the current state of the cluster closer to the desired state
func (r *TargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// log := ctrl.LoggerFrom(ctx)

	// Fetch target

	// Check, if resource is being deleted:
	//   If no finalizer return
	//   Remove created resources and finalizer and return.
	//   (always return)

	// Set finalizer on target, when not set already

	// Check if hydrated target exists, if not create and make sure to SetControllerReference...

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Target{}).
		Owns(&solarv1alpha1.HydratedTarget{}).
		Complete(r)
}
