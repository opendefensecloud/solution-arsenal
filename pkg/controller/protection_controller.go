// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// ProtectionReconciler releases protection finalizers that no referrer accounts for.
//
// Referrers add a protection finalizer to what they reference, and release it when they
// are deleted. A referrer that switches to another object cannot release the one it left:
// its reconcile only sees the new reference. These controllers run on the protected side
// instead. They watch the referrers, enqueue both the old and the new reference of an
// update, and drop the finalizer from any object that no active referrer still references.
// This also releases finalizers leaked before they existed.
type ProtectionReconciler struct {
	client.Client
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions;releases;registries,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=profiles;releasebindings;targets;registrybindings,verbs=get;list;watch

// protection describes one protection finalizer: the kind it protects and who refers to it.
type protection struct {
	name       string
	finalizer  string
	newObject  func() client.Object
	referenced func(ctx context.Context, c client.Client, obj client.Object) (bool, error)
	referrers  []referrer
}

// referrer is a kind that holds a protection, with the objects one of them references.
type referrer struct {
	object client.Object
	refs   func(obj client.Object) []types.NamespacedName
}

func protections() []protection {
	return []protection{
		{
			name:      "componentversion-protection",
			finalizer: componentVersionRefFinalizer,
			newObject: func() client.Object { return &solarv1alpha1.ComponentVersion{} },
			referenced: func(ctx context.Context, c client.Client, obj client.Object) (bool, error) {
				return componentVersionReferenced(ctx, c, obj.(*solarv1alpha1.ComponentVersion), nil)
			},
			referrers: []referrer{{
				object: &solarv1alpha1.Release{},
				refs: func(obj client.Object) []types.NamespacedName {
					rel := obj.(*solarv1alpha1.Release)
					return nonEmpty(cmp.Or(rel.Spec.ComponentVersionRef.Namespace, rel.Namespace), rel.Spec.ComponentVersionRef.Name)
				},
			}},
		},
		{
			name:      "release-protection",
			finalizer: releaseRefFinalizer,
			newObject: func() client.Object { return &solarv1alpha1.Release{} },
			referenced: func(ctx context.Context, c client.Client, obj client.Object) (bool, error) {
				return releaseReferenced(ctx, c, obj.(*solarv1alpha1.Release), nil, nil)
			},
			referrers: []referrer{
				{
					object: &solarv1alpha1.Profile{},
					refs: func(obj client.Object) []types.NamespacedName {
						p := obj.(*solarv1alpha1.Profile)
						return nonEmpty(p.Namespace, p.Spec.ReleaseRef.Name)
					},
				},
				{
					object: &solarv1alpha1.ReleaseBinding{},
					refs: func(obj client.Object) []types.NamespacedName {
						rb := obj.(*solarv1alpha1.ReleaseBinding)
						return nonEmpty(rb.Namespace, rb.Spec.ReleaseRef.Name)
					},
				},
			},
		},
		{
			name:      "registry-protection",
			finalizer: registryRefFinalizer,
			newObject: func() client.Object { return &solarv1alpha1.Registry{} },
			referenced: func(ctx context.Context, c client.Client, obj client.Object) (bool, error) {
				return registryReferenced(ctx, c, obj.(*solarv1alpha1.Registry), nil, nil)
			},
			referrers: []referrer{
				{
					object: &solarv1alpha1.Target{},
					refs: func(obj client.Object) []types.NamespacedName {
						t := obj.(*solarv1alpha1.Target)
						return nonEmpty(cmp.Or(t.Spec.RenderRegistryRef.Namespace, t.Namespace), t.Spec.RenderRegistryRef.Name)
					},
				},
				{
					object: &solarv1alpha1.RegistryBinding{},
					refs: func(obj client.Object) []types.NamespacedName {
						rb := obj.(*solarv1alpha1.RegistryBinding)
						return nonEmpty(rb.Namespace, rb.Spec.RegistryRef.Name)
					},
				},
			},
		},
	}
}

func nonEmpty(namespace, name string) []types.NamespacedName {
	if name == "" {
		return nil
	}

	return []types.NamespacedName{{Namespace: namespace, Name: name}}
}

// SetupWithManager sets up one controller per protection finalizer.
func (r *ProtectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	for _, p := range protections() {
		b := ctrl.NewControllerManagedBy(mgr).Named(p.name).For(p.newObject())
		for _, ref := range p.referrers {
			b = b.Watches(ref.object, enqueueReferenced(ref.refs))
		}
		if err := b.Complete(reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
			return reconcile.Result{}, r.release(ctx, p, req.NamespacedName)
		})); err != nil {
			return fmt.Errorf("failed to set up %s controller: %w", p.name, err)
		}
	}

	return nil
}

// release drops p's finalizer from the object at key unless an active referrer still
// references it.
func (r *ProtectionReconciler) release(ctx context.Context, p protection, key types.NamespacedName) error {
	obj := p.newObject()
	if err := r.Get(ctx, key, obj); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !slices.Contains(obj.GetFinalizers(), p.finalizer) {
		return nil
	}
	referenced, err := p.referenced(ctx, r.Client, obj)
	if err != nil || referenced {
		return err
	}

	return dropFinalizer(ctx, r.Client, obj, p.finalizer)
}

// enqueueReferenced enqueues what a referrer references. An update enqueues the old
// reference too: that is the object a switch leaves behind.
func enqueueReferenced(refs func(client.Object) []types.NamespacedName) handler.EventHandler {
	add := func(q workqueue.TypedRateLimitingInterface[reconcile.Request], objs ...client.Object) {
		for _, obj := range objs {
			for _, key := range refs(obj) {
				q.Add(reconcile.Request{NamespacedName: key})
			}
		}
	}

	return handler.Funcs{
		UpdateFunc: func(_ context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			add(q, e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(_ context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			add(q, e.Object)
		},
	}
}

// dropFinalizer removes finalizer from the latest version of obj. The optimistic lock
// makes a referrer adding it back concurrently win over the removal.
func dropFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) error {
	fresh, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("cannot copy %T", obj)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), fresh); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !slices.Contains(fresh.GetFinalizers(), finalizer) {
		return nil
	}
	original, ok := fresh.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("cannot copy %T", fresh)
	}
	fresh.SetFinalizers(slices.DeleteFunc(fresh.GetFinalizers(), func(s string) bool { return s == finalizer }))
	if err := c.Patch(ctx, fresh, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})); err != nil {
		return fmt.Errorf("failed to remove finalizer %s from %s: %w", finalizer, client.ObjectKeyFromObject(obj), err)
	}

	ctrl.LoggerFrom(ctx).V(1).Info("Removed protection finalizer", "finalizer", finalizer, "object", client.ObjectKeyFromObject(obj))

	return nil
}

// componentVersionReferenced reports whether an active Release other than skip references cv.
func componentVersionReferenced(ctx context.Context, c client.Client, cv *solarv1alpha1.ComponentVersion, skip *solarv1alpha1.Release) (bool, error) {
	releases := &solarv1alpha1.ReleaseList{}
	if err := c.List(ctx, releases, client.MatchingFields{indexReleaseByCVRef: cv.Namespace + "/" + cv.Name}); err != nil {
		return false, fmt.Errorf("failed to list Releases for ComponentVersion finalizer check: %w", err)
	}
	for _, rel := range releases.Items {
		if skip != nil && rel.Name == skip.Name && rel.Namespace == skip.Namespace {
			continue
		}
		if rel.DeletionTimestamp.IsZero() {
			return true, nil
		}
	}

	return false, nil
}

// releaseReferenced reports whether a Profile other than skipProfile, or a ReleaseBinding
// other than skipBinding, still references release.
//
// A deleting Profile that still holds profileFinalizer counts: the Profile controller
// releases the Release itself once its bindings are gone. A binding counts unless it is
// deleting or its owning Profile is gone or deleting (garbage collection removes it).
func releaseReferenced(ctx context.Context, c client.Client, release *solarv1alpha1.Release, skipProfile *solarv1alpha1.Profile, skipBinding *solarv1alpha1.ReleaseBinding) (bool, error) {
	profiles := &solarv1alpha1.ProfileList{}
	if err := c.List(ctx, profiles,
		client.InNamespace(release.Namespace),
		client.MatchingFields{indexProfileByReleaseName: release.Name},
	); err != nil {
		return false, fmt.Errorf("failed to list Profiles for Release finalizer check: %w", err)
	}
	for _, p := range profiles.Items {
		if skipProfile != nil && p.Name == skipProfile.Name {
			continue
		}
		if p.DeletionTimestamp.IsZero() || slices.Contains(p.Finalizers, profileFinalizer) {
			return true, nil
		}
	}

	bindings := &solarv1alpha1.ReleaseBindingList{}
	if err := c.List(ctx, bindings,
		client.InNamespace(release.Namespace),
		client.MatchingFields{indexReleaseBindingReleaseName: release.Name},
	); err != nil {
		return false, fmt.Errorf("failed to list ReleaseBindings for Release finalizer check: %w", err)
	}
	// Owner Profiles by name; nil means gone or deleting.
	owners := map[string]*solarv1alpha1.Profile{}
	for _, rb := range bindings.Items {
		if skipBinding != nil && rb.Name == skipBinding.Name {
			continue
		}
		if !rb.DeletionTimestamp.IsZero() {
			continue
		}
		owner := metav1.GetControllerOf(&rb)
		if owner == nil || owner.Kind != "Profile" || owner.APIVersion != solarv1alpha1.SchemeGroupVersion.String() {
			return true, nil
		}
		if _, seen := owners[owner.Name]; !seen {
			p := &solarv1alpha1.Profile{}
			err := c.Get(ctx, types.NamespacedName{Name: owner.Name, Namespace: rb.Namespace}, p)
			switch {
			case apierrors.IsNotFound(err) || (err == nil && !p.DeletionTimestamp.IsZero()):
				owners[owner.Name] = nil
			case err != nil:
				return false, fmt.Errorf("failed to check owner Profile for Release finalizer check: %w", err)
			default:
				owners[owner.Name] = p
			}
		}
		if owners[owner.Name] != nil {
			return true, nil
		}
	}

	return false, nil
}

// registryReferenced reports whether an active Target other than skipTarget, or an active
// RegistryBinding other than skipBinding, references registry.
//
// List results come from the informer cache, so a just-created referrer may not be visible
// yet. Its own reconcile adds the protection finalizer back, so the window heals itself.
func registryReferenced(ctx context.Context, c client.Client, registry *solarv1alpha1.Registry, skipTarget *solarv1alpha1.Target, skipBinding *solarv1alpha1.RegistryBinding) (bool, error) {
	targets := &solarv1alpha1.TargetList{}
	if err := c.List(ctx, targets, client.MatchingFields{indexTargetByRegistryRef: registry.Namespace + "/" + registry.Name}); err != nil {
		return false, fmt.Errorf("failed to list Targets for Registry finalizer check: %w", err)
	}
	for _, t := range targets.Items {
		if skipTarget != nil && t.Name == skipTarget.Name && t.Namespace == skipTarget.Namespace {
			continue
		}
		if t.DeletionTimestamp.IsZero() {
			return true, nil
		}
	}

	bindings := &solarv1alpha1.RegistryBindingList{}
	if err := c.List(ctx, bindings,
		client.InNamespace(registry.Namespace),
		client.MatchingFields{indexRegistryBindingByRegistryName: registry.Name},
	); err != nil {
		return false, fmt.Errorf("failed to list RegistryBindings for Registry finalizer check: %w", err)
	}
	for _, rb := range bindings.Items {
		if skipBinding != nil && rb.Name == skipBinding.Name {
			continue
		}
		if rb.DeletionTimestamp.IsZero() {
			return true, nil
		}
	}

	return false, nil
}
