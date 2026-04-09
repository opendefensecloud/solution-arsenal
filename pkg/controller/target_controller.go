// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	targetFinalizer = "solar.opendefense.cloud/target-finalizer"
)

type TargetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	// See: https://book.kubebuilder.io/reference/envtest#testing-considerations
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=bootstraps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=profiles,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile moves the current state of the cluster closer to the desired state
func (r *TargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("Target is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

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
		r.Recorder.Eventf(target, nil, corev1.EventTypeWarning, "Deleting", "Reconcile", "Target is being deleted, cleaning up Bootstrap")

		// Delete Bootstrap
		if err := r.Delete(ctx, &solarv1alpha1.Bootstrap{ObjectMeta: metav1.ObjectMeta{Namespace: target.Namespace, Name: target.Name}}); err != nil && !apierrors.IsNotFound(err) {
			return ctrlResult, errLogAndWrap(log, err, "failed to delete Bootstrap")
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

	// Get matching profiles
	profileList := &solarv1alpha1.ProfileList{}
	if err := r.List(ctx, profileList, client.InNamespace(target.Namespace)); err != nil {
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to list Profiles")
	}

	matchingProfiles := make(map[string]corev1.LocalObjectReference)
	targetLabels := labels.Set(target.Labels)

	for _, profile := range profileList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&profile.Spec.TargetSelector)
		if err != nil {
			log.Error(err, "invalid targetSelector in Profile; skipping")
			continue
		}

		if selector.Matches(targetLabels) {
			matchingProfiles[profile.Name] = corev1.LocalObjectReference{Name: profile.Name}
		}
	}

	// Check if bootstrap exists, if not create and make sure to SetControllerReference...
	bootstrap := &solarv1alpha1.Bootstrap{}
	err := r.Get(ctx, req.NamespacedName, bootstrap)

	if err != nil && !apierrors.IsNotFound(err) {
		return ctrlResult, errLogAndWrap(log, err, "failed to get Bootstrap")
	}

	// Create Bootstrap if not exists or update/override spec
	if apierrors.IsNotFound(err) {
		log.V(1).Info("Creating Bootstrap for Target", "target", req.NamespacedName)
		bootstrap = &solarv1alpha1.Bootstrap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      target.Name,
				Namespace: target.Namespace,
			},
			Spec: solarv1alpha1.BootstrapSpec{
				Releases: target.Spec.Releases,
				Profiles: matchingProfiles,
				Userdata: target.Spec.Userdata,
			},
		}
		if err := ctrl.SetControllerReference(target, bootstrap, r.Scheme); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to set controller reference on Bootstrap")
		}
		if err := r.Create(ctx, bootstrap); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return ctrlResult, errLogAndWrap(log, err, "failed to create Bootstrap")
			}
			log.V(1).Info("Bootstrap already exists, will update", "bootstrap", req.NamespacedName)
		} else {
			r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Created", "Create", "Created Bootstrap %s/%s", bootstrap.Namespace, bootstrap.Name)
			return ctrlResult, nil
		}
	}

	// Update if out of sync
	// re-fetch target and bootstrap to avoid conflicts
	bootstrap = &solarv1alpha1.Bootstrap{}
	if err := r.Get(ctx, req.NamespacedName, bootstrap); err != nil {
		return ctrlResult, errLogAndWrap(log, err, "failed to re-fetch Bootstrap for update check")
	}
	target = &solarv1alpha1.Target{}
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		return ctrlResult, errLogAndWrap(log, err, "failed to re-fetch Target for update check")
	}

	original := bootstrap.DeepCopy()

	bootstrap.Spec.Releases = target.Spec.Releases
	bootstrap.Spec.Profiles = matchingProfiles
	bootstrap.Spec.Userdata = target.Spec.Userdata

	if !apiequality.Semantic.DeepEqual(original.Spec, bootstrap.Spec) {
		log.V(1).Info("Updating Bootstrap for Target", "target", req.NamespacedName)
		if err := r.Patch(ctx, bootstrap, client.MergeFrom(original)); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to update Bootstrap")
		}
		r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Updated", "Update", "Updated Bootstrap %s/%s", bootstrap.Namespace, bootstrap.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Target{}).
		Owns(&solarv1alpha1.Bootstrap{}).
		Watches(
			&solarv1alpha1.Profile{},
			handler.EnqueueRequestsFromMapFunc(r.mapProfileToTargets),
			builder.WithPredicates(profileSelectionPredicate()),
		).
		Complete(r)
}

// profileSelectionPredicate filters events to only trigger reconciles when the target selector of a profile changes.
func profileSelectionPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj, ok1 := e.ObjectOld.(*solarv1alpha1.Profile)
			newObj, ok2 := e.ObjectNew.(*solarv1alpha1.Profile)
			if !ok1 || !ok2 {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldObj.Spec.TargetSelector, newObj.Spec.TargetSelector)
		},
	}
}

// mapProfileToTargets maps a Profile to a list of Target reconcile requests.
func (r *TargetReconciler) mapProfileToTargets(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	profile, ok := obj.(*solarv1alpha1.Profile)
	if !ok {
		log.Error(nil, "Object is not a Profile", "type", fmt.Sprintf("%T", obj))
		return nil
	}

	selector, err := metav1.LabelSelectorAsSelector(&profile.Spec.TargetSelector)
	if err != nil {
		log.Error(err, "Invalid targetSelector in Profile", "profile", profile.Name, "targetSelector", profile.Spec.TargetSelector.String())
		return nil
	}

	targetList := &solarv1alpha1.TargetList{}
	err = r.List(ctx, targetList,
		client.InNamespace(profile.GetNamespace()),
		client.MatchingLabelsSelector{Selector: selector},
	)
	if err != nil {
		log.V(1).Error(err, "Failed to list Targets for Profile", "profile", profile.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(targetList.Items))
	for _, target := range targetList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      target.Name,
				Namespace: target.Namespace,
			},
		})
	}

	return requests
}
