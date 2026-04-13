// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// ProfileReconciler reconciles a Profile object.
// It evaluates the Profile's TargetSelector against all Targets in the namespace
// and creates/deletes ReleaseBindings accordingly.
type ProfileReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	// WatchNamespace restricts reconciliation to this namespace.
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=profiles,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=profiles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releasebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile evaluates the Profile's TargetSelector and ensures matching ReleaseBindings exist.
func (r *ProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("Profile is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	// Fetch Profile
	profile := &solarv1alpha1.Profile{}
	if err := r.Get(ctx, req.NamespacedName, profile); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Profile")
	}

	// Evaluate TargetSelector against all Targets
	selector, err := metav1.LabelSelectorAsSelector(&profile.Spec.TargetSelector)
	if err != nil {
		log.Error(err, "invalid targetSelector in Profile")

		return ctrl.Result{}, nil
	}

	targetList := &solarv1alpha1.TargetList{}
	if err := r.List(ctx, targetList,
		client.InNamespace(profile.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to list Targets")
	}

	// Build set of desired ReleaseBindings (one per matching target)
	desiredTargets := map[string]bool{}
	for _, target := range targetList.Items {
		desiredTargets[target.Name] = true
	}

	// List existing ReleaseBindings owned by this Profile
	existingBindings := &solarv1alpha1.ReleaseBindingList{}
	if err := r.List(ctx, existingBindings,
		client.InNamespace(profile.Namespace),
		client.MatchingFields{"metadata.ownerReferences.name": profile.Name},
	); err != nil {
		// Field index may not be available; fall back to listing all and filtering
		allBindings := &solarv1alpha1.ReleaseBindingList{}
		if err := r.List(ctx, allBindings, client.InNamespace(profile.Namespace)); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to list ReleaseBindings")
		}

		existingBindings = &solarv1alpha1.ReleaseBindingList{}

		for i := range allBindings.Items {
			if metav1.IsControlledBy(&allBindings.Items[i], profile) {
				existingBindings.Items = append(existingBindings.Items, allBindings.Items[i])
			}
		}
	}

	// Delete ReleaseBindings for targets that no longer match
	existingTargets := map[string]*solarv1alpha1.ReleaseBinding{}
	for i := range existingBindings.Items {
		rb := &existingBindings.Items[i]
		existingTargets[rb.Spec.TargetRef.Name] = rb

		if !desiredTargets[rb.Spec.TargetRef.Name] {
			log.V(1).Info("Deleting ReleaseBinding for unmatched target", "target", rb.Spec.TargetRef.Name)
			if err := r.Delete(ctx, rb); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete ReleaseBinding")
			}

			r.Recorder.Eventf(profile, nil, corev1.EventTypeNormal, "Deleted", "Delete",
				"Deleted ReleaseBinding for target %s", rb.Spec.TargetRef.Name)
		}
	}

	// Create ReleaseBindings for new matching targets
	for _, target := range targetList.Items {
		if _, exists := existingTargets[target.Name]; exists {
			continue
		}

		rb := &solarv1alpha1.ReleaseBinding{
			ObjectMeta: metav1.ObjectMeta{
				// We need to truncated the name: 57 (input) + 1 (-) + 5 (appended by generated) = 63 (max chars allowed)
				GenerateName: truncateName(fmt.Sprintf("%s-%s", profile.Name, target.Name), 57) + "-",
				Namespace:    profile.Namespace,
			},
			Spec: solarv1alpha1.ReleaseBindingSpec{
				TargetRef:  corev1.LocalObjectReference{Name: target.Name},
				ReleaseRef: profile.Spec.ReleaseRef,
			},
		}
		if err := ctrl.SetControllerReference(profile, rb, r.Scheme); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to set controller reference on ReleaseBinding")
		}

		if err := r.Create(ctx, rb); err != nil {
			if apierrors.IsAlreadyExists(err) {
				continue
			}

			return ctrl.Result{}, errLogAndWrap(log, err, "failed to create ReleaseBinding")
		}

		log.V(1).Info("Created ReleaseBinding for target", "target", target.Name)
		r.Recorder.Eventf(profile, nil, corev1.EventTypeNormal, "Created", "Create",
			"Created ReleaseBinding for target %s", target.Name)
	}

	// Update status
	original := profile.DeepCopy()
	profile.Status.MatchedTargets = len(targetList.Items)
	if profile.Status.MatchedTargets != original.Status.MatchedTargets {
		if err := r.Status().Update(ctx, profile); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to update Profile status")
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Profile{}).
		Owns(&solarv1alpha1.ReleaseBinding{}).
		Watches(
			&solarv1alpha1.Target{},
			handler.EnqueueRequestsFromMapFunc(r.mapTargetToProfiles),
		).
		Complete(r)
}

// mapTargetToProfiles maps a Target to all Profiles in the same namespace that might match it.
func (r *ProfileReconciler) mapTargetToProfiles(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	target, ok := obj.(*solarv1alpha1.Target)
	if !ok {
		return nil
	}

	profileList := &solarv1alpha1.ProfileList{}
	if err := r.List(ctx, profileList, client.InNamespace(target.Namespace)); err != nil {
		log.Error(err, "failed to list Profiles for Target mapping")

		return nil
	}

	targetLabels := labels.Set(target.Labels)
	var requests []reconcile.Request

	for _, profile := range profileList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&profile.Spec.TargetSelector)
		if err != nil {
			continue
		}

		if selector.Matches(targetLabels) {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&profile),
			})
		}
	}

	return requests
}
