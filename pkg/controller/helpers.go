// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	// Field index keys for looking up RenderTasks by owner.
	indexOwnerKind      = "spec.ownerKind"
	indexOwnerName      = "spec.ownerName"
	indexOwnerNamespace = "spec.ownerNamespace"

	// Field index keys for looking up ReleaseBindings by target or release name.
	indexReleaseBindingTargetName      = "spec.targetRef.name"
	indexReleaseBindingTargetNamespace = "spec.targetNamespace"
	indexReleaseBindingReleaseName     = "spec.releaseRef.name"

	// Field index key for looking up RegistryBindings by target name.
	indexRegistryBindingTargetName = "spec.targetRef.name"

	// Field index key for looking up RenderBindings by the RenderArtifact they reference.
	indexRenderBindingArtifactName = "spec.renderArtifactRef.name"

	// Field index keys for deletion-protection reference lookups.
	// Release: composite "<cvNamespace>/<cvName>" resolving cross-namespace refs.
	indexReleaseByCVRef = "dp.spec.componentVersionRef"
	// ComponentVersion: same-namespace lookup by component name.
	indexCVByComponentName = "dp.spec.componentRef.name"
	// Profile: same-namespace lookup by release name.
	indexProfileByReleaseName = "dp.spec.releaseRef.name"
	// Target: composite "<registryNamespace>/<registryName>" resolving cross-namespace refs.
	indexTargetByRegistryRef = "dp.spec.renderRegistryRef"
	// RegistryBinding: same-namespace lookup by registry name.
	indexRegistryBindingByRegistryName = "dp.spec.registryRef.name"

	maxK8sObjectNameLen = 253
	maxK8sLabelValueLen = 63

	// Self-finalizers: added to the referencing resource so the controller can observe deletion.
	releaseFinalizer          = "solar.opendefense.cloud/release-finalizer"
	profileFinalizer          = "solar.opendefense.cloud/profile-finalizer"
	releaseBindingFinalizer   = "solar.opendefense.cloud/releasebinding-finalizer"
	componentVersionFinalizer = "solar.opendefense.cloud/componentversion-finalizer"
	registryBindingFinalizer  = "solar.opendefense.cloud/registrybinding-finalizer"

	// Protection finalizers: added to the referenced resource to block deletion while referenced.
	componentVersionRefFinalizer = "solar.opendefense.cloud/componentversion-ref"
	componentRefFinalizer        = "solar.opendefense.cloud/component-ref"
	releaseRefFinalizer          = "solar.opendefense.cloud/release-ref"
	registryRefFinalizer         = "solar.opendefense.cloud/registry-ref"
)

// truncateName truncates a name to maxLen characters. If truncation is needed,
// it appends a short hash suffix to preserve uniqueness.
func truncateName(name string, maxLen int) string {
	if maxLen < 10 {
		maxLen = 10
	}
	if len(name) <= maxLen {
		return name
	}
	hash := sha256.Sum256([]byte(name))
	hashStr := hex.EncodeToString(hash[:])[:8]

	return name[:maxLen-9] + "-" + hashStr
}

// mapRenderTaskToOwner returns a handler.MapFunc that maps RenderTask events
// to reconcile requests for the owning resource of the specified kind.
func mapRenderTaskToOwner(kind string) handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		rt, ok := obj.(*solarv1alpha1.RenderTask)
		if !ok {
			return nil
		}

		if rt.Spec.OwnerKind != kind || rt.Spec.OwnerName == "" || rt.Spec.OwnerNamespace == "" {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      rt.Spec.OwnerName,
					Namespace: rt.Spec.OwnerNamespace,
				},
			},
		}
	}
}

// effectiveUniqueName returns the deduplication key for a release: Spec.UniqueName
// when set, otherwise the parent Component name from the referenced ComponentVersion.
// This mirrors the logic in the Release controller that writes Status.EffectiveUniqueName.
func effectiveUniqueName(rel *solarv1alpha1.Release, cv *solarv1alpha1.ComponentVersion) string {
	if rel.Spec.UniqueName != "" {
		return rel.Spec.UniqueName
	}

	return cv.Spec.ComponentRef.Name
}

// releaseRenderTaskName returns a deterministic name for a per-release RenderTask
// scoped to a specific target. Each target creates its own release RenderTasks;
// the renderer job handles deduplication by skipping rendering if the chart
// already exists in the registry.
func releaseRenderTaskName(releaseNamespace, releaseName, targetName string, generation int64) string {
	input := fmt.Sprintf("%s/%s-%s-%d", releaseNamespace, releaseName, targetName, generation)
	hash := sha256.Sum256([]byte(input))
	hashStr := hex.EncodeToString(hash[:])[:8]

	return truncateName(fmt.Sprintf("render-rel-%s-%s", releaseName, hashStr), maxK8sObjectNameLen)
}

// targetRenderTaskName returns a deterministic name for a per-target bootstrap RenderTask.
// The bootstrapVersion is incremented each time the bootstrap needs re-rendering.
func targetRenderTaskName(targetName string, bootstrapVersion int64) string {
	return truncateName(fmt.Sprintf("render-tgt-%s-%d", targetName, bootstrapVersion), maxK8sObjectNameLen)
}

// renderArtifactName returns a deterministic name for a RenderArtifact based on the
// artifact's registry coordinates. Multiple RenderTasks that push to the same (namespace,
// baseURL, repository, tag) tuple will resolve to the same artifact name, enabling sharing.
func renderArtifactName(namespace, baseURL, repository, tag string) string {
	input := fmt.Sprintf("%s/%s/%s/%s", namespace, baseURL, repository, tag)
	hash := sha256.Sum256([]byte(input))
	hashStr := hex.EncodeToString(hash[:])[:8]

	return "render-art-" + hashStr
}

// renderBindingName returns a deterministic name for a RenderBinding linking a Target
// to a RenderArtifact.
func renderBindingName(artifactName, ownerName string) string {
	input := fmt.Sprintf("%s/Target/%s", artifactName, ownerName)
	hash := sha256.Sum256([]byte(input))
	hashStr := hex.EncodeToString(hash[:])[:8]

	return "render-bind-" + hashStr
}

// renderChartURL constructs the fully-qualified OCI chart URL from push coordinates.
func renderChartURL(baseURL, repository, tag string) string {
	base := baseURL
	if !strings.HasPrefix(base, "oci://") {
		base = "oci://" + base
	}

	return strings.TrimSuffix(base, "/") + "/" + repository + ":" + tag
}

// registryHost extracts the registry host from a repository string and
// normalises it to lower-case (hostnames are case-insensitive per RFC 4343).
// For example, "Registry.Example.COM:5000/foo/bar" returns "registry.example.com:5000".
func registryHost(repository string) string {
	repo := strings.TrimPrefix(repository, "oci://")
	if before, _, ok := strings.Cut(repo, "/"); ok {
		return strings.ToLower(before)
	}

	return strings.ToLower(repo)
}

// resolveResources converts ResourceAccess entries from a ComponentVersion into
// ResolvedResourceAccess for the renderer. PullSecretName is looked up from
// pullSecretsByHost by extracting the registry host from each resource's repository.
// In strict mode, an error is returned if any resource's host has no matching
// RegistryBinding.
func resolveResources(resources map[string]solarv1alpha1.ResourceAccess, pullSecretsByHost map[string]string, strict bool) (map[string]solarv1alpha1.ResolvedResourceAccess, error) {
	resolved := make(map[string]solarv1alpha1.ResolvedResourceAccess, len(resources))
	for name, ra := range resources {
		host := registryHost(ra.Repository)
		pullSecret, found := pullSecretsByHost[host]
		if strict && !found {
			return nil, fmt.Errorf("no RegistryBinding for host %q (resource %q); create a RegistryBinding or use relaxed mode", host, name)
		}

		resolved[name] = solarv1alpha1.ResolvedResourceAccess{
			Repository:     ra.Repository,
			Insecure:       ra.Insecure,
			Tag:            ra.Tag,
			Helm:           ra.Helm,
			PullSecretName: pullSecret,
		}
	}

	return resolved, nil
}

// pullSecretsTag returns a short hash derived from the pull-secret names in
// resolved resources. It is appended to the chart tag so that charts whose
// content differs only in secretRef (due to RegistryBinding changes) get
// unique OCI tags, preventing the renderer's exists-check from skipping a
// necessary re-push.
func pullSecretsTag(resolved map[string]solarv1alpha1.ResolvedResourceAccess) string {
	keys := make([]string, 0, len(resolved))
	for k := range resolved {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	h := sha256.New()

	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s;", k, resolved[k].PullSecretName)
	}

	return hex.EncodeToString(h.Sum(nil))[:8]
}

// IndexFields registers field indexers on the manager for efficient lookups.
// Must be called once before any controller that uses these indexes is set up.
func IndexFields(ctx context.Context, mgr ctrl.Manager) error {
	if err := indexReleaseBindingFields(ctx, mgr); err != nil {
		return err
	}

	if err := indexRenderTaskOwnerFields(ctx, mgr); err != nil {
		return err
	}

	if err := indexRegistryBindingFields(ctx, mgr); err != nil {
		return err
	}

	if err := indexRenderBindingFields(ctx, mgr); err != nil {
		return err
	}

	return indexDeletionProtectionFields(ctx, mgr)
}

// indexDeletionProtectionFields registers field indexers used to count active references
// when deciding whether to remove a protection finalizer from a referenced resource.
func indexDeletionProtectionFields(ctx context.Context, mgr ctrl.Manager) error {
	indexer := mgr.GetFieldIndexer()

	// Release → ComponentVersion: composite "<cvNamespace>/<cvName>" to handle cross-namespace refs.
	if err := indexer.IndexField(ctx, &solarv1alpha1.Release{}, indexReleaseByCVRef, func(obj client.Object) []string {
		rel := obj.(*solarv1alpha1.Release)
		if rel.Spec.ComponentVersionRef.Name == "" {
			return nil
		}
		cvNs := rel.Namespace
		if rel.Spec.ComponentVersionNamespace != "" {
			cvNs = rel.Spec.ComponentVersionNamespace
		}

		return []string{cvNs + "/" + rel.Spec.ComponentVersionRef.Name}
	}); err != nil {
		return err
	}

	// ComponentVersion → Component: same-namespace, index by component name.
	if err := indexer.IndexField(ctx, &solarv1alpha1.ComponentVersion{}, indexCVByComponentName, func(obj client.Object) []string {
		cv := obj.(*solarv1alpha1.ComponentVersion)
		if cv.Spec.ComponentRef.Name == "" {
			return nil
		}

		return []string{cv.Spec.ComponentRef.Name}
	}); err != nil {
		return err
	}

	// Profile → Release: same-namespace, index by release name.
	if err := indexer.IndexField(ctx, &solarv1alpha1.Profile{}, indexProfileByReleaseName, func(obj client.Object) []string {
		p := obj.(*solarv1alpha1.Profile)
		if p.Spec.ReleaseRef.Name == "" {
			return nil
		}

		return []string{p.Spec.ReleaseRef.Name}
	}); err != nil {
		return err
	}

	// Target → Registry: composite "<registryNamespace>/<registryName>" to handle cross-namespace refs.
	if err := indexer.IndexField(ctx, &solarv1alpha1.Target{}, indexTargetByRegistryRef, func(obj client.Object) []string {
		t := obj.(*solarv1alpha1.Target)
		if t.Spec.RenderRegistryRef.Name == "" {
			return nil
		}
		regNs := t.Namespace
		if t.Spec.RenderRegistryNamespace != "" {
			regNs = t.Spec.RenderRegistryNamespace
		}

		return []string{regNs + "/" + t.Spec.RenderRegistryRef.Name}
	}); err != nil {
		return err
	}

	// RegistryBinding → Registry: same-namespace, index by registry name.
	return indexer.IndexField(ctx, &solarv1alpha1.RegistryBinding{}, indexRegistryBindingByRegistryName, func(obj client.Object) []string {
		rb := obj.(*solarv1alpha1.RegistryBinding)
		if rb.Spec.RegistryRef.Name == "" {
			return nil
		}

		return []string{rb.Spec.RegistryRef.Name}
	})
}

func indexReleaseBindingFields(ctx context.Context, mgr ctrl.Manager) error {
	indexer := mgr.GetFieldIndexer()

	if err := indexer.IndexField(ctx, &solarv1alpha1.ReleaseBinding{}, indexReleaseBindingTargetName, func(obj client.Object) []string {
		rb := obj.(*solarv1alpha1.ReleaseBinding)
		if rb.Spec.TargetRef.Name == "" {
			return nil
		}

		return []string{rb.Spec.TargetRef.Name}
	}); err != nil {
		return err
	}

	if err := indexer.IndexField(ctx, &solarv1alpha1.ReleaseBinding{}, indexReleaseBindingTargetNamespace, func(obj client.Object) []string {
		rb := obj.(*solarv1alpha1.ReleaseBinding)
		// Empty TargetNamespace is intentionally indexed as "" — the same-namespace binding
		// query in target_controller.go filters on "" to exclude cross-namespace bindings.
		return []string{rb.Spec.TargetNamespace}
	}); err != nil {
		return err
	}

	return indexer.IndexField(ctx, &solarv1alpha1.ReleaseBinding{}, indexReleaseBindingReleaseName, func(obj client.Object) []string {
		rb := obj.(*solarv1alpha1.ReleaseBinding)
		if rb.Spec.ReleaseRef.Name == "" {
			return nil
		}

		return []string{rb.Spec.ReleaseRef.Name}
	})
}

func indexRegistryBindingFields(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &solarv1alpha1.RegistryBinding{}, indexRegistryBindingTargetName, func(obj client.Object) []string {
		rb := obj.(*solarv1alpha1.RegistryBinding)
		if rb.Spec.TargetRef.Name == "" {
			return nil
		}

		return []string{rb.Spec.TargetRef.Name}
	})
}

// indexRenderTaskOwnerFields registers field indexers on the manager for
// looking up RenderTasks by owner kind, name, and namespace.
func indexRenderTaskOwnerFields(ctx context.Context, mgr ctrl.Manager) error {
	indexer := mgr.GetFieldIndexer()

	if err := indexer.IndexField(ctx, &solarv1alpha1.RenderTask{}, indexOwnerKind, func(obj client.Object) []string {
		rt := obj.(*solarv1alpha1.RenderTask)
		if rt.Spec.OwnerKind == "" {
			return nil
		}

		return []string{rt.Spec.OwnerKind}
	}); err != nil {
		return err
	}

	if err := indexer.IndexField(ctx, &solarv1alpha1.RenderTask{}, indexOwnerName, func(obj client.Object) []string {
		rt := obj.(*solarv1alpha1.RenderTask)
		if rt.Spec.OwnerName == "" {
			return nil
		}

		return []string{rt.Spec.OwnerName}
	}); err != nil {
		return err
	}

	return indexer.IndexField(ctx, &solarv1alpha1.RenderTask{}, indexOwnerNamespace, func(obj client.Object) []string {
		rt := obj.(*solarv1alpha1.RenderTask)
		if rt.Spec.OwnerNamespace == "" {
			return nil
		}

		return []string{rt.Spec.OwnerNamespace}
	})
}

// indexRenderBindingFields registers field indexers for RenderBinding lookups.
// The same owner field key names as RenderTask are used; controller-runtime
// indexes are keyed per object type, so there is no conflict.
func indexRenderBindingFields(ctx context.Context, mgr ctrl.Manager) error {
	indexer := mgr.GetFieldIndexer()

	if err := indexer.IndexField(ctx, &solarv1alpha1.RenderBinding{}, indexOwnerKind, func(obj client.Object) []string {
		rb := obj.(*solarv1alpha1.RenderBinding)
		if rb.Spec.OwnerKind == "" {
			return nil
		}

		return []string{rb.Spec.OwnerKind}
	}); err != nil {
		return err
	}

	if err := indexer.IndexField(ctx, &solarv1alpha1.RenderBinding{}, indexOwnerName, func(obj client.Object) []string {
		rb := obj.(*solarv1alpha1.RenderBinding)
		if rb.Spec.OwnerName == "" {
			return nil
		}

		return []string{rb.Spec.OwnerName}
	}); err != nil {
		return err
	}

	if err := indexer.IndexField(ctx, &solarv1alpha1.RenderBinding{}, indexOwnerNamespace, func(obj client.Object) []string {
		rb := obj.(*solarv1alpha1.RenderBinding)
		if rb.Spec.OwnerNamespace == "" {
			return nil
		}

		return []string{rb.Spec.OwnerNamespace}
	}); err != nil {
		return err
	}

	return indexer.IndexField(ctx, &solarv1alpha1.RenderBinding{}, indexRenderBindingArtifactName, func(obj client.Object) []string {
		rb := obj.(*solarv1alpha1.RenderBinding)
		if rb.Spec.RenderArtifactRef.Name == "" {
			return nil
		}

		return []string{rb.Spec.RenderArtifactRef.Name}
	})
}

// removeRegistryRefFinalizer removes registryRefFinalizer from registry when no other active
// Target (excluding skipTarget if non-nil) or RegistryBinding (excluding skipRegistryBinding
// if non-nil) still references it.
func removeRegistryRefFinalizer(ctx context.Context, c client.Client, skipTarget *solarv1alpha1.Target, skipRegistryBinding *solarv1alpha1.RegistryBinding, registry *solarv1alpha1.Registry) error {
	if !slices.Contains(registry.Finalizers, registryRefFinalizer) {
		return nil
	}

	refKey := registry.Namespace + "/" + registry.Name

	// List results come from the informer cache, so a just-created referencer may not be
	// visible yet. The referencer's own reconcile will re-add the protection finalizer in
	// that case, making the window self-healing and safe to accept.
	targetList := &solarv1alpha1.TargetList{}
	if err := c.List(ctx, targetList, client.MatchingFields{indexTargetByRegistryRef: refKey}); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to list Targets for Registry finalizer check")
	}

	for _, t := range targetList.Items {
		if skipTarget != nil && t.Name == skipTarget.Name && t.Namespace == skipTarget.Namespace {
			continue
		}
		if !t.DeletionTimestamp.IsZero() {
			continue
		}

		return nil
	}

	rbList := &solarv1alpha1.RegistryBindingList{}
	if err := c.List(ctx, rbList,
		client.InNamespace(registry.Namespace),
		client.MatchingFields{indexRegistryBindingByRegistryName: registry.Name},
	); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to list RegistryBindings for Registry finalizer check")
	}

	for _, rb := range rbList.Items {
		if skipRegistryBinding != nil && rb.Name == skipRegistryBinding.Name {
			continue
		}
		if !rb.DeletionTimestamp.IsZero() {
			continue
		}

		return nil
	}

	freshRegistry := &solarv1alpha1.Registry{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(registry), freshRegistry); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to get latest Registry for finalizer removal")
	}
	original := freshRegistry.DeepCopy()
	freshRegistry.Finalizers = slices.DeleteFunc(freshRegistry.Finalizers, func(s string) bool { return s == registryRefFinalizer })
	if err := c.Patch(ctx, freshRegistry, client.MergeFrom(original)); err != nil {
		return errLogAndWrap(ctrl.LoggerFrom(ctx), err, "failed to remove protection finalizer from Registry")
	}

	ctrl.LoggerFrom(ctx).V(1).Info("Removed protection finalizer from Registry", "registry", registry.Name)

	return nil
}
