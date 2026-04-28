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

// bindingTargetKey returns a stable namespace/name key for a ReleaseBinding's target reference.
func bindingTargetKey(rb *solarv1alpha1.ReleaseBinding) string {
	ns := rb.Spec.TargetNamespace
	if ns == "" {
		ns = rb.Namespace
	}

	return ns + "/" + rb.Spec.TargetRef.Name
}

// targetKey returns the namespace/name key for a Target.
func targetKey(t *solarv1alpha1.Target) string {
	return t.Namespace + "/" + t.Name
}

// solarGroup is the API group for all solar resources.
const solarGroup = "solar.opendefense.cloud"

// grantPermits returns true if the ReferenceGrant allows a resource identified by
// (fromGroup, fromKind, fromNamespace) to reference a resource of (toGroup, toKind)
// in the grant's own namespace.
func grantPermits(grant *solarv1alpha1.ReferenceGrant, fromGroup, fromKind, fromNamespace, toGroup, toKind string) bool {
	hasFrom := false
	for _, f := range grant.Spec.From {
		if f.Namespace == fromNamespace && f.Kind == fromKind && f.Group == fromGroup {
			hasFrom = true
			break
		}
	}
	if !hasFrom {
		return false
	}
	for _, t := range grant.Spec.To {
		if t.Kind == toKind && t.Group == toGroup {
			return true
		}
	}

	return false
}

// grantPermitsTargetAccess returns true if the ReferenceGrant allows a Profile in
// fromNamespace to reference Target resources in the grant's namespace.
func grantPermitsTargetAccess(grant *solarv1alpha1.ReferenceGrant, fromNamespace string) bool {
	return grantPermits(grant, solarGroup, "Profile", fromNamespace, solarGroup, "Target")
}

// grantsTargetResource returns true if the ReferenceGrant includes Target in its To list.
func grantsTargetResource(grant *solarv1alpha1.ReferenceGrant) bool {
	for _, t := range grant.Spec.To {
		if t.Kind == "Target" && t.Group == solarGroup {
			return true
		}
	}

	return false
}

// grantPermitsComponentVersionAccess returns true if the ReferenceGrant allows a Release
// in fromNamespace to reference ComponentVersion resources in the grant's namespace.
func grantPermitsComponentVersionAccess(grant *solarv1alpha1.ReferenceGrant, fromNamespace string) bool {
	return grantPermits(grant, solarGroup, "Release", fromNamespace, solarGroup, "ComponentVersion")
}

// grantsComponentVersionResource returns true if the ReferenceGrant includes ComponentVersion in its To list.
func grantsComponentVersionResource(grant *solarv1alpha1.ReferenceGrant) bool {
	for _, t := range grant.Spec.To {
		if t.Kind == "ComponentVersion" && t.Group == solarGroup {
			return true
		}
	}

	return false
}

// ProfileReconciler reconciles a Profile object.
// It evaluates the Profile's TargetSelector against all Targets in the namespace
// and creates/deletes ReleaseBindings accordingly.
// Cross-namespace Targets are included when a ReferenceGrant in the target's namespace
// grants the Profile's namespace access to "targets".
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
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=referencegrants,verbs=get;list;watch
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

	// Collect matching targets from the profile's own namespace
	sameNsTargets := &solarv1alpha1.TargetList{}
	if err := r.List(ctx, sameNsTargets,
		client.InNamespace(profile.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to list Targets")
	}

	allTargets := make([]solarv1alpha1.Target, 0, len(sameNsTargets.Items))
	allTargets = append(allTargets, sameNsTargets.Items...)

	// Collect cross-namespace targets via ReferenceGrants.
	// A ReferenceGrant in namespace B listing the profile's namespace in From and
	// "targets" in To allows this Profile to select Targets from namespace B.
	grantList := &solarv1alpha1.ReferenceGrantList{}
	if err := r.List(ctx, grantList); err != nil {
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to list ReferenceGrants")
	}

	for i := range grantList.Items {
		grant := &grantList.Items[i]
		if grant.Namespace == profile.Namespace {
			// same-namespace targets already covered above
			continue
		}
		if !grantPermitsTargetAccess(grant, profile.Namespace) {
			continue
		}
		crossNsTargets := &solarv1alpha1.TargetList{}
		if err := r.List(ctx, crossNsTargets,
			client.InNamespace(grant.Namespace),
			client.MatchingLabelsSelector{Selector: selector},
		); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to list cross-namespace Targets in "+grant.Namespace)
		}
		allTargets = append(allTargets, crossNsTargets.Items...)
	}

	// Build set of desired ReleaseBindings (one per matching target, keyed by namespace/name)
	desiredTargets := map[string]solarv1alpha1.Target{}
	for _, t := range allTargets {
		desiredTargets[targetKey(&t)] = t
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
	existingByKey := map[string]*solarv1alpha1.ReleaseBinding{}
	for i := range existingBindings.Items {
		rb := &existingBindings.Items[i]
		key := bindingTargetKey(rb)
		existingByKey[key] = rb

		if _, desired := desiredTargets[key]; !desired {
			log.V(1).Info("Deleting ReleaseBinding for unmatched target", "key", key)
			if err := r.Delete(ctx, rb); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete ReleaseBinding")
			}

			r.Recorder.Eventf(profile, nil, corev1.EventTypeNormal, "Deleted", "Delete",
				"Deleted ReleaseBinding for target %s", key)
		}
	}

	// Create ReleaseBindings for new matching targets
	for key, target := range desiredTargets {
		if _, exists := existingByKey[key]; exists {
			continue
		}

		crossNs := ""
		if target.Namespace != profile.Namespace {
			crossNs = target.Namespace
		}

		rb := &solarv1alpha1.ReleaseBinding{
			ObjectMeta: metav1.ObjectMeta{
				// We need to truncated the name: 57 (input) + 1 (-) + 5 (appended by generated) = 63 (max chars allowed)
				GenerateName: truncateName(fmt.Sprintf("%s-%s", profile.Name, target.Name), 57) + "-",
				Namespace:    profile.Namespace,
			},
			Spec: solarv1alpha1.ReleaseBindingSpec{
				TargetRef:       corev1.LocalObjectReference{Name: target.Name},
				TargetNamespace: crossNs,
				ReleaseRef:      profile.Spec.ReleaseRef,
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

		log.V(1).Info("Created ReleaseBinding for target", "key", key)
		r.Recorder.Eventf(profile, nil, corev1.EventTypeNormal, "Created", "Create",
			"Created ReleaseBinding for target %s", key)
	}

	// Update status
	original := profile.DeepCopy()
	profile.Status.MatchedTargets = len(desiredTargets)
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
		Watches(
			&solarv1alpha1.ReferenceGrant{},
			handler.EnqueueRequestsFromMapFunc(r.mapReferenceGrantToProfiles),
		).
		Complete(r)
}

// mapTargetToProfiles maps a Target to all Profiles that might match it,
// including Profiles in namespaces that have been granted access via ReferenceGrant.
func (r *ProfileReconciler) mapTargetToProfiles(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	target, ok := obj.(*solarv1alpha1.Target)
	if !ok {
		return nil
	}

	targetLabels := labels.Set(target.Labels)
	var requests []reconcile.Request

	// Enqueue profiles in the target's own namespace
	profileList := &solarv1alpha1.ProfileList{}
	if err := r.List(ctx, profileList, client.InNamespace(target.Namespace)); err != nil {
		log.Error(err, "failed to list Profiles for Target mapping")

		return nil
	}

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

	// Enqueue profiles in namespaces that have been granted access to targets in
	// this target's namespace via a ReferenceGrant.
	grantList := &solarv1alpha1.ReferenceGrantList{}
	if err := r.List(ctx, grantList, client.InNamespace(target.Namespace)); err != nil {
		log.Error(err, "failed to list ReferenceGrants for cross-namespace Target mapping")

		return requests
	}

	for i := range grantList.Items {
		grant := &grantList.Items[i]
		if !grantsTargetResource(grant) {
			continue
		}
		for _, from := range grant.Spec.From {
			if from.Namespace == target.Namespace {
				continue
			}
			fromProfiles := &solarv1alpha1.ProfileList{}
			if err := r.List(ctx, fromProfiles, client.InNamespace(from.Namespace)); err != nil {
				log.Error(err, "failed to list Profiles in granted namespace", "namespace", from.Namespace)
				continue
			}
			for _, p := range fromProfiles.Items {
				selector, err := metav1.LabelSelectorAsSelector(&p.Spec.TargetSelector)
				if err != nil {
					continue
				}
				if selector.Matches(targetLabels) {
					requests = append(requests, reconcile.Request{
						NamespacedName: client.ObjectKeyFromObject(&p),
					})
				}
			}
		}
	}

	return requests
}

// mapReferenceGrantToProfiles enqueues all Profiles in the namespaces listed in
// a ReferenceGrant's From field, allowing them to re-evaluate cross-namespace matches.
func (r *ProfileReconciler) mapReferenceGrantToProfiles(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	grant, ok := obj.(*solarv1alpha1.ReferenceGrant)
	if !ok {
		return nil
	}

	if !grantsTargetResource(grant) {
		return nil
	}

	var requests []reconcile.Request
	for _, from := range grant.Spec.From {
		profiles := &solarv1alpha1.ProfileList{}
		if err := r.List(ctx, profiles, client.InNamespace(from.Namespace)); err != nil {
			log.Error(err, "failed to list Profiles for ReferenceGrant mapping", "namespace", from.Namespace)
			continue
		}
		for _, p := range profiles.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&p),
			})
		}
	}

	return requests
}
