// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	ociname "github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	targetFinalizer = "solar.opendefense.cloud/target-finalizer"

	ConditionTypeRegistryResolved = "RegistryResolved"
	ConditionTypeReleasesResolved = "ReleasesResolved"
	ConditionTypeReleasesRendered = "ReleasesRendered"
	ConditionTypeBootstrapReady   = "BootstrapReady"
)

var ErrReleaseNotRenderedYet = errors.New("release is not rendered yet")

type releaseInfo struct {
	// bindingKey is "<namespace>/<name>" of the originating ReleaseBinding, used as a
	// deterministic tiebreaker when two releases share the same priority.
	bindingKey string
	name       string
	release    *solarv1alpha1.Release
	cv         *solarv1alpha1.ComponentVersion
	rtName     string
	chartURL   string
}

type TargetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	WatchNamespace string
	// RegistryBindingStrict enables strict registry binding mode.
	// When true, rendering fails if a resource's registry host has no
	// matching RegistryBinding. When false (default/relaxed), unmatched
	// hosts are treated as anonymous pull (no secretRef rendered).
	RegistryBindingStrict bool
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registries,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releasebindings,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registrybindings,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=referencegrants,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile collects ReleaseBindings, resolves the render registry, creates per-release
// RenderTasks (with dedup), and creates a per-target bootstrap RenderTask.
func (r *TargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("Target is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	// Fetch target
	target := &solarv1alpha1.Target{}
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get object")
	}

	// Handle deletion
	if !target.DeletionTimestamp.IsZero() {
		log.V(1).Info("Target is being deleted")
		r.Recorder.Eventf(target, nil, corev1.EventTypeWarning, "Deleting", "Reconcile", "Target is being deleted, cleaning up RenderTasks")

		// Delete owned RenderTasks
		if err := r.deleteOwnedRenderTasks(ctx, target); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete owned RenderTasks")
		}

		// Remove finalizer
		if slices.Contains(target.Finalizers, targetFinalizer) {
			latest := &solarv1alpha1.Target{}
			if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest Target for finalizer removal")
			}

			original := latest.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool {
				return s == targetFinalizer
			})
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to remove finalizer from Target")
			}
		}

		return ctrl.Result{}, nil
	}

	// Set finalizer if not set
	if !slices.Contains(target.Finalizers, targetFinalizer) {
		latest := &solarv1alpha1.Target{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get latest Target for finalizer addition")
		}

		original := latest.DeepCopy()
		latest.Finalizers = append(latest.Finalizers, targetFinalizer)
		if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to add finalizer to Target")
		}

		return ctrl.Result{}, nil
	}

	// Resolve render registry — supports cross-namespace via ReferenceGrant
	registryNamespace := target.Namespace
	if target.Spec.RenderRegistryNamespace != "" {
		registryNamespace = target.Spec.RenderRegistryNamespace
	}

	// If the registry lives in a different namespace, verify a ReferenceGrant permits it
	// before attempting to fetch the object.
	if registryNamespace != target.Namespace {
		granted, err := r.registryGranted(ctx, registryNamespace, target.Namespace)
		if err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to check ReferenceGrant for Registry")
		}
		if !granted {
			if condErr := r.setCondition(ctx, target, ConditionTypeRegistryResolved, metav1.ConditionFalse, "NotGranted",
				"No ReferenceGrant allows access to Registry "+target.Spec.RenderRegistryRef.Name+" in namespace "+registryNamespace); condErr != nil {
				return ctrl.Result{}, condErr
			}

			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	registry := &solarv1alpha1.Registry{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      target.Spec.RenderRegistryRef.Name,
		Namespace: registryNamespace,
	}, registry); err != nil {
		if apierrors.IsNotFound(err) {
			if condErr := r.setCondition(ctx, target, ConditionTypeRegistryResolved, metav1.ConditionFalse, "NotFound",
				"Registry not found: "+target.Spec.RenderRegistryRef.Name); condErr != nil {
				return ctrl.Result{}, condErr
			}

			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Registry")
	}

	if registry.Spec.SolarSecretRef == nil {
		if condErr := r.setCondition(ctx, target, ConditionTypeRegistryResolved, metav1.ConditionFalse, "MissingSolarSecretRef",
			"Registry does not have SolarSecretRef set, required for rendering"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{}, nil
	}

	if condErr := r.setCondition(ctx, target, ConditionTypeRegistryResolved, metav1.ConditionTrue, "Resolved",
		"Registry resolved: "+registry.Name); condErr != nil {
		return ctrl.Result{}, condErr
	}

	// Build hostname→targetPullSecretName lookup from RegistryBindings for this target.
	pullSecretsByHost, err := r.buildPullSecretsLookup(ctx, target)
	if err != nil {
		if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "RegistryBindingConflict",
			err.Error()); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to build pull secrets lookup from RegistryBindings")
	}

	// Collect ReleaseBindings for this target — same namespace first, then cross-namespace via ReferenceGrants.
	// Filter on targetNamespace="" to exclude cross-namespace bindings (targetNamespace set) that share the
	// target name but point to a target in a different namespace.
	bindingList := &solarv1alpha1.ReleaseBindingList{}
	if err := r.List(ctx, bindingList,
		client.InNamespace(target.Namespace),
		client.MatchingFields{
			indexReleaseBindingTargetName:      target.Name,
			indexReleaseBindingTargetNamespace: "",
		},
	); err != nil {
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to list ReleaseBindings")
	}

	// Collect cross-namespace ReleaseBindings authorized by ReferenceGrants in target's namespace.
	crossNsBindings, crossNsErr := r.collectCrossNamespaceReleaseBindings(ctx, target)
	if crossNsErr != nil {
		return ctrl.Result{}, errLogAndWrap(log, crossNsErr, "failed to collect cross-namespace ReleaseBindings")
	}
	bindingList.Items = append(bindingList.Items, crossNsBindings...)

	// FIXME: collect cross-namespace RegistryBindings here once ADR-010 is finalized and
	// RegistryBinding collection is wired into the rendering pipeline.

	if len(bindingList.Items) == 0 {
		log.V(1).Info("No ReleaseBindings found for target")
		if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "NoReleaseBindings",
			"No ReleaseBindings found for this target"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		if condErr := r.setCondition(ctx, target, ConditionTypeReleasesResolved, metav1.ConditionFalse, "NoReleaseBindings",
			"No ReleaseBindings found for this target"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{}, nil
	}

	// For each bound release, ensure a per-release RenderTask exists
	var releases []releaseInfo

	pendingDeps := false

	for _, binding := range bindingList.Items {
		rel := &solarv1alpha1.Release{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      binding.Spec.ReleaseRef.Name,
			Namespace: binding.Namespace,
		}, rel); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("Release not found", "release", binding.Spec.ReleaseRef.Name)
				pendingDeps = true

				continue
			}

			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get Release")
		}

		cv := &solarv1alpha1.ComponentVersion{}
		cvNamespace := rel.Namespace
		if rel.Spec.ComponentVersionNamespace != "" {
			cvNamespace = rel.Spec.ComponentVersionNamespace
		}

		if cvNamespace != rel.Namespace {
			granted := false
			grantList := &solarv1alpha1.ReferenceGrantList{}
			if err := r.List(ctx, grantList, client.InNamespace(cvNamespace)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to check ReferenceGrant for cross-namespace ComponentVersion")
			}
			for i := range grantList.Items {
				if grantPermitsComponentVersionAccess(&grantList.Items[i], rel.Namespace) {
					granted = true
					break
				}
			}
			if !granted {
				log.V(1).Info("ComponentVersion access not granted", "cv", rel.Spec.ComponentVersionRef.Name, "namespace", cvNamespace)
				pendingDeps = true

				continue
			}
		}

		if err := r.Get(ctx, client.ObjectKey{
			Name:      rel.Spec.ComponentVersionRef.Name,
			Namespace: cvNamespace,
		}, cv); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("ComponentVersion not found", "cv", rel.Spec.ComponentVersionRef.Name)
				pendingDeps = true

				continue
			}

			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get ComponentVersion")
		}

		rtName := releaseRenderTaskName(rel.Namespace, rel.Name, target.Name, rel.GetGeneration())
		releases = append(releases, releaseInfo{
			bindingKey: binding.Namespace + "/" + binding.Name,
			name:       rel.Name,
			release:    rel,
			cv:         cv,
			rtName:     rtName,
		})
	}

	// Resolve conflicts: deduplicate by uniqueName (priority wins) and apply anti-affinity rules.
	var skipped []string
	releases, skipped = resolveReleaseConflicts(releases)
	if condErr := r.setResolvedCondition(ctx, target, skipped); condErr != nil {
		return ctrl.Result{}, condErr
	}

	if len(releases) == 0 && !pendingDeps {
		if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "AllReleaseBindingsFiltered",
			"All ReleaseBindings were filtered out by the release resolver (uniqueName conflicts or anti-affinity rules)"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{}, nil
	}

	// Create per-release RenderTasks (one per target+release pair).
	// The renderer job handles dedup by skipping if the chart already exists in the registry.
	allRendered := true

	for i, ri := range releases {
		rt := &solarv1alpha1.RenderTask{}
		err := r.Get(ctx, client.ObjectKey{Name: ri.rtName, Namespace: target.Namespace}, rt)

		switch {
		case apierrors.IsNotFound(err):
			spec, specErr := r.computeReleaseRenderTaskSpec(ri.release, ri.cv, registry, target, pullSecretsByHost)
			if specErr != nil {
				if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "MissingRegistryBinding",
					specErr.Error()); condErr != nil {
					return ctrl.Result{}, condErr
				}

				return ctrl.Result{}, errLogAndWrap(log, specErr, "failed to compute release RenderTask spec")
			}

			rt = &solarv1alpha1.RenderTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ri.rtName,
					Namespace: target.Namespace,
				},
				Spec: spec,
			}

			if err := r.Create(ctx, rt); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to create release RenderTask")
			}

			log.V(1).Info("Created release RenderTask", "release", ri.name, "renderTask", ri.rtName)
			r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Created", "Create",
				"Created release RenderTask %s for release %s", ri.rtName, ri.name)
		case err != nil:
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to get release RenderTask")
		default:
			// RenderTask exists — check for spec drift (e.g. pull secrets
			// changed after a RegistryBinding was created/updated).
			desiredSpec, specErr := r.computeReleaseRenderTaskSpec(ri.release, ri.cv, registry, target, pullSecretsByHost)
			if specErr != nil {
				if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "MissingRegistryBinding",
					specErr.Error()); condErr != nil {
					return ctrl.Result{}, condErr
				}

				return ctrl.Result{}, errLogAndWrap(log, specErr, "failed to compute release RenderTask spec for comparison")
			}

			if !apiequality.Semantic.DeepEqual(rt.Spec, desiredSpec) {
				if err := r.Delete(ctx, rt); err != nil {
					return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete stale release RenderTask")
				}

				rt = &solarv1alpha1.RenderTask{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ri.rtName,
						Namespace: target.Namespace,
					},
					Spec: desiredSpec,
				}

				if err := r.Create(ctx, rt); err != nil {
					return ctrl.Result{}, errLogAndWrap(log, err, "failed to recreate release RenderTask")
				}

				log.V(1).Info("Recreated release RenderTask (spec drift)", "release", ri.name, "renderTask", ri.rtName)
				r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Updated", "Update",
					"Recreated release RenderTask %s for release %s (spec drift)", ri.rtName, ri.name)
			}
		}

		// Check if release RenderTask is complete
		if apimeta.IsStatusConditionTrue(rt.Status.Conditions, ConditionTypeJobFailed) {
			if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "ReleaseFailed",
				fmt.Sprintf("Release %s rendering failed", ri.name)); condErr != nil {
				return ctrl.Result{}, condErr
			}

			return ctrl.Result{}, nil
		}

		if apimeta.IsStatusConditionTrue(rt.Status.Conditions, ConditionTypeJobSucceeded) && rt.Status.ChartURL != "" {
			releases[i].chartURL = rt.Status.ChartURL
		} else {
			allRendered = false
		}
	}

	if pendingDeps {
		if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "MissingDependencies",
			"One or more bound Releases or ComponentVersions not found"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if !allRendered {
		if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionFalse, "Pending",
			"Waiting for release RenderTasks to complete"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if condErr := r.setCondition(ctx, target, ConditionTypeReleasesRendered, metav1.ConditionTrue, "AllRendered",
		"All releases rendered successfully"); condErr != nil {
		return ctrl.Result{}, condErr
	}

	// Determine if a new bootstrap render is needed by checking whether the
	// current bootstrapVersion's RenderTask still matches the desired release set.
	bootstrapVersion := target.Status.BootstrapVersion
	bootstrapRTName := targetRenderTaskName(target.Name, bootstrapVersion)
	bootstrapRT := &solarv1alpha1.RenderTask{}
	err = r.Get(ctx, client.ObjectKey{Name: bootstrapRTName, Namespace: target.Namespace}, bootstrapRT)

	needsNewBootstrap := false

	switch {
	case apierrors.IsNotFound(err):
		// No RenderTask for the current version yet — create one
		needsNewBootstrap = true
	case err != nil:
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get bootstrap RenderTask")
	default:
		// RenderTask exists — check if the desired bootstrap input changed
		// (release set, resolved refs/tags, or userdata)
		desiredInput, inputErr := buildBootstrapInput(target, releases, registry.Spec.TargetPullSecretName)
		if inputErr != nil {
			return ctrl.Result{}, errLogAndWrap(log, inputErr, "failed to build desired bootstrap input for comparison")
		}

		existingInput := bootstrapRT.Spec.RendererConfig.BootstrapConfig.Input
		if !apiequality.Semantic.DeepEqual(desiredInput, existingInput) {
			bootstrapVersion++
			needsNewBootstrap = true
		}
	}

	if needsNewBootstrap {
		spec, specErr := r.computeBootstrapRenderTaskSpec(target, releases, registry, bootstrapVersion)
		if specErr != nil {
			return ctrl.Result{}, errLogAndWrap(log, specErr, "failed to compute bootstrap RenderTask spec")
		}

		bootstrapRTName = targetRenderTaskName(target.Name, bootstrapVersion)
		bootstrapRT = &solarv1alpha1.RenderTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bootstrapRTName,
				Namespace: target.Namespace,
			},
			Spec: spec,
		}

		if err := r.Create(ctx, bootstrapRT); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to create bootstrap RenderTask")
			}

			if err := r.Get(ctx, client.ObjectKey{Name: bootstrapRTName, Namespace: target.Namespace}, bootstrapRT); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to get existing bootstrap RenderTask")
			}
		} else {
			log.V(1).Info("Created bootstrap RenderTask", "renderTask", bootstrapRTName, "bootstrapVersion", bootstrapVersion)
			r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Created", "Create",
				"Created bootstrap RenderTask %s (version %d)", bootstrapRTName, bootstrapVersion)
		}

		// Persist the new bootstrapVersion in status
		if bootstrapVersion != target.Status.BootstrapVersion {
			target.Status.BootstrapVersion = bootstrapVersion
			if err := r.Status().Update(ctx, target); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to update Target bootstrapVersion")
			}
		}
	}

	// Update target status from bootstrap RenderTask
	if apimeta.IsStatusConditionTrue(bootstrapRT.Status.Conditions, ConditionTypeJobFailed) {
		if condErr := r.setCondition(ctx, target, ConditionTypeBootstrapReady, metav1.ConditionFalse, "Failed",
			"Bootstrap rendering failed"); condErr != nil {
			return ctrl.Result{}, condErr
		}

		return ctrl.Result{}, nil
	}

	if apimeta.IsStatusConditionTrue(bootstrapRT.Status.Conditions, ConditionTypeJobSucceeded) {
		if condErr := r.setCondition(ctx, target, ConditionTypeBootstrapReady, metav1.ConditionTrue, "Ready",
			"Bootstrap rendered successfully: "+bootstrapRT.Status.ChartURL); condErr != nil {
			return ctrl.Result{}, condErr
		}

		// Clean up stale RenderTasks owned by this target (old versions)
		currentRTNames := map[string]struct{}{bootstrapRTName: {}}
		for _, ri := range releases {
			currentRTNames[ri.rtName] = struct{}{}
		}
		if err := r.deleteStaleRenderTasks(ctx, target, currentRTNames); err != nil {
			log.Error(err, "failed to clean up stale RenderTasks")
		}

		return ctrl.Result{}, nil
	}

	// Still running
	return ctrl.Result{}, nil
}

func (r *TargetReconciler) setCondition(ctx context.Context, target *solarv1alpha1.Target, condType string, status metav1.ConditionStatus, reason, message string) error {
	changed := apimeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: target.Generation,
		Reason:             reason,
		Message:            message,
	})
	if changed {
		if err := r.Status().Update(ctx, target); err != nil {
			return fmt.Errorf("failed to update Target status condition %s: %w", condType, err)
		}
	}

	return nil
}

func (r *TargetReconciler) setResolvedCondition(ctx context.Context, target *solarv1alpha1.Target, skipped []string) error {
	if len(skipped) == 0 {
		return r.setCondition(ctx, target, ConditionTypeReleasesResolved, metav1.ConditionTrue, "NoConflicts", "")
	}

	return r.setCondition(ctx, target, ConditionTypeReleasesResolved, metav1.ConditionTrue, "Resolved", strings.Join(skipped, "; "))
}

// resolveReleaseConflicts deduplicates releases by uniqueName (keeping the highest-priority
// binding) and filters releases that violate anti-affinity rules of already-accepted releases.
// Releases without a uniqueName are deduplicated using the parent Component name from the CV.
// It returns the accepted releases and a slice of human-readable filter messages.
func resolveReleaseConflicts(releases []releaseInfo) ([]releaseInfo, []string) {
	if len(releases) == 0 {
		return releases, nil
	}

	// Step A: uniqueName deduplication.
	// When UniqueName is empty, fall back to the parent Component name from the CV.
	namedGroups := map[string][]releaseInfo{}

	for _, ri := range releases {
		uniqueName := ri.release.Spec.UniqueName
		if uniqueName == "" {
			uniqueName = ri.cv.Spec.ComponentRef.Name
		}

		namedGroups[uniqueName] = append(namedGroups[uniqueName], ri)
	}

	var accepted []releaseInfo

	var skipped []string

	// byPriority sorts releases with highest priority first; bindingKey breaks ties.
	byPriority := func(a, b releaseInfo) bool {
		if a.release.Spec.Priority != b.release.Spec.Priority {
			return a.release.Spec.Priority > b.release.Spec.Priority
		}

		return a.bindingKey < b.bindingKey
	}

	uniqueNames := make([]string, 0, len(namedGroups))
	for k := range namedGroups {
		uniqueNames = append(uniqueNames, k)
	}

	sort.Strings(uniqueNames)

	for _, uniqueName := range uniqueNames {
		group := namedGroups[uniqueName]
		sort.Slice(group, func(i, j int) bool { return byPriority(group[i], group[j]) })

		accepted = append(accepted, group[0])

		for _, loser := range group[1:] {
			skipped = append(skipped, fmt.Sprintf(
				"binding %s filtered: uniqueName %q conflict, lower priority than %s",
				loser.bindingKey, uniqueName, group[0].bindingKey,
			))
		}
	}

	// Step B: anti-affinity evaluation.
	// Walk in deterministic order (priority desc, bindingKey asc); accept each release only
	// if its AntiAffinity selector does not match any already-accepted release's labels.
	sort.Slice(accepted, func(i, j int) bool { return byPriority(accepted[i], accepted[j]) })

	resolved := make([]releaseInfo, 0, len(accepted))

	for _, ri := range accepted {
		// Parse ri's own anti-affinity selector once; bail early on invalid selector.
		var riSelector labels.Selector
		if ri.release.Spec.AntiAffinity != nil {
			sel, err := metav1.LabelSelectorAsSelector(ri.release.Spec.AntiAffinity)
			if err != nil {
				skipped = append(skipped, fmt.Sprintf(
					"binding %s filtered: invalid antiAffinity selector: %v",
					ri.bindingKey, err,
				))

				continue
			}

			riSelector = sel
		}

		// Check both directions: ri's anti-affinity against already-resolved labels,
		// and already-resolved anti-affinities against ri's labels.
		conflict := ""
		for _, other := range resolved {
			if riSelector != nil && riSelector.Matches(labels.Set(other.release.Labels)) {
				conflict = other.bindingKey
				break
			}

			if other.release.Spec.AntiAffinity != nil {
				otherSel, err := metav1.LabelSelectorAsSelector(other.release.Spec.AntiAffinity)
				if err == nil && otherSel.Matches(labels.Set(ri.release.Labels)) {
					conflict = other.bindingKey
					break
				}
			}
		}

		if conflict != "" {
			skipped = append(skipped, fmt.Sprintf(
				"binding %s filtered: anti-affinity conflict with %s",
				ri.bindingKey, conflict,
			))
		} else {
			resolved = append(resolved, ri)
		}
	}

	return resolved, skipped
}

// deleteStaleRenderTasks removes RenderTasks owned by this target that are no
// longer needed. Any owned RenderTask whose name is not in currentRTNames is
// deleted. This covers both old bootstrap versions and old release generations.
func (r *TargetReconciler) deleteStaleRenderTasks(ctx context.Context, target *solarv1alpha1.Target, currentRTNames map[string]struct{}) error {
	log := ctrl.LoggerFrom(ctx)

	rtList := &solarv1alpha1.RenderTaskList{}
	if err := r.List(ctx, rtList,
		client.InNamespace(target.Namespace),
		client.MatchingFields{indexOwnerKind: "Target"},
	); err != nil {
		return err
	}

	for i := range rtList.Items {
		rt := &rtList.Items[i]
		if rt.Spec.OwnerName != target.Name || rt.Spec.OwnerNamespace != target.Namespace {
			continue
		}

		if _, current := currentRTNames[rt.Name]; current {
			continue
		}

		log.V(1).Info("Deleting stale RenderTask", "renderTask", rt.Name)
		if err := r.Delete(ctx, rt, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			return err
		}

		r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Deleted", "Delete",
			"Deleted stale RenderTask %s", rt.Name)
	}

	return nil
}

func (r *TargetReconciler) deleteOwnedRenderTasks(ctx context.Context, target *solarv1alpha1.Target) error {
	rtList := &solarv1alpha1.RenderTaskList{}
	if err := r.List(ctx, rtList,
		client.InNamespace(target.Namespace),
		client.MatchingFields{indexOwnerKind: "Target"},
	); err != nil {
		return err
	}

	for i := range rtList.Items {
		rt := &rtList.Items[i]
		if rt.Spec.OwnerName == target.Name && rt.Spec.OwnerNamespace == target.Namespace {
			if err := r.Delete(ctx, rt, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}

	return nil
}

func (r *TargetReconciler) computeReleaseRenderTaskSpec(rel *solarv1alpha1.Release, cv *solarv1alpha1.ComponentVersion, registry *solarv1alpha1.Registry, target *solarv1alpha1.Target, pullSecretsByHost map[string]string) (solarv1alpha1.RenderTaskSpec, error) {
	chartName := fmt.Sprintf("release-%s", rel.Name)
	repo := fmt.Sprintf("%s/%s/%s", target.Namespace, rel.Namespace, chartName)

	var targetNamespace string
	if rel.Spec.TargetNamespace != nil {
		targetNamespace = *rel.Spec.TargetNamespace
	}

	resolvedResources, err := resolveResources(cv.Spec.Resources, pullSecretsByHost, r.RegistryBindingStrict)
	if err != nil {
		return solarv1alpha1.RenderTaskSpec{}, fmt.Errorf("release %s: %w", rel.Name, err)
	}

	// Include a hash of pull-secret names in the tag so that charts whose
	// content differs only in secretRef get unique OCI tags. Without this,
	// the renderer's exists-check skips re-pushing after a spec-drift
	// recreation (e.g. RegistryBinding created after the first render).
	tag := fmt.Sprintf("v0.0.%d-%s", rel.GetGeneration(), pullSecretsTag(resolvedResources))

	return solarv1alpha1.RenderTaskSpec{
		RendererConfig: solarv1alpha1.RendererConfig{
			Type: solarv1alpha1.RendererConfigTypeRelease,
			ReleaseConfig: solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        chartName,
					Description: fmt.Sprintf("Release of %s", rel.Spec.ComponentVersionRef.Name),
					Version:     tag,
					AppVersion:  tag,
				},
				Input: solarv1alpha1.ReleaseInput{
					Component:  solarv1alpha1.ReleaseComponent{Name: cv.Spec.ComponentRef.Name},
					Resources:  resolvedResources,
					Entrypoint: cv.Spec.Entrypoint,
				},
				Values:          rel.Spec.Values,
				TargetNamespace: targetNamespace,
			},
		},
		Repository:     repo,
		Tag:            tag,
		BaseURL:        registry.Spec.Hostname,
		PushSecretRef:  registry.Spec.SolarSecretRef,
		FailedJobTTL:   rel.Spec.FailedJobTTL,
		OwnerName:      target.Name,
		OwnerNamespace: target.Namespace,
		OwnerKind:      "Target",
	}, nil
}

// buildBootstrapInput constructs the desired BootstrapInput from the current
// target and resolved releases. Used for both comparison and spec construction.
func buildBootstrapInput(target *solarv1alpha1.Target, releases []releaseInfo, renderRegistryPullSecret string) (solarv1alpha1.BootstrapInput, error) {
	resolvedReleases := map[string]solarv1alpha1.ResolvedResourceAccess{}

	for _, ri := range releases {
		ref, err := ociname.ParseReference(ri.chartURL)
		if err != nil {
			return solarv1alpha1.BootstrapInput{}, fmt.Errorf("failed to parse chartURL %s: %w", ri.chartURL, err)
		}

		repo, err := url.JoinPath(ref.Context().RegistryStr(), ref.Context().RepositoryStr())
		if err != nil {
			return solarv1alpha1.BootstrapInput{}, err
		}

		resolvedReleases[ri.name] = solarv1alpha1.ResolvedResourceAccess{
			Repository:     strings.TrimPrefix(repo, "oci://"),
			Tag:            ref.Identifier(),
			PullSecretName: renderRegistryPullSecret,
		}
	}

	return solarv1alpha1.BootstrapInput{
		Releases: resolvedReleases,
		Userdata: target.Spec.Userdata,
	}, nil
}

func (r *TargetReconciler) computeBootstrapRenderTaskSpec(target *solarv1alpha1.Target, releases []releaseInfo, registry *solarv1alpha1.Registry, bootstrapVersion int64) (solarv1alpha1.RenderTaskSpec, error) {
	input, err := buildBootstrapInput(target, releases, registry.Spec.TargetPullSecretName)
	if err != nil {
		return solarv1alpha1.RenderTaskSpec{}, err
	}

	releaseNames := make([]string, 0, len(releases))
	for _, ri := range releases {
		releaseNames = append(releaseNames, ri.name)
	}

	sort.Strings(releaseNames)

	chartName := fmt.Sprintf("bootstrap-%s", target.Name)
	repo := fmt.Sprintf("%s/%s", target.Namespace, chartName)
	tag := fmt.Sprintf("v0.0.%d", bootstrapVersion)

	return solarv1alpha1.RenderTaskSpec{
		RendererConfig: solarv1alpha1.RendererConfig{
			Type: solarv1alpha1.RendererConfigTypeBootstrap,
			BootstrapConfig: solarv1alpha1.BootstrapConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        chartName,
					Description: fmt.Sprintf("Bootstrap of %v", releaseNames),
					Version:     tag,
					AppVersion:  tag,
				},
				Input: input,
			},
		},
		Repository:     repo,
		Tag:            tag,
		BaseURL:        registry.Spec.Hostname,
		PushSecretRef:  registry.Spec.SolarSecretRef,
		OwnerName:      target.Name,
		OwnerNamespace: target.Namespace,
		OwnerKind:      "Target",
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Target{}).
		Watches(
			&solarv1alpha1.ReleaseBinding{},
			handler.EnqueueRequestsFromMapFunc(r.mapReleaseBindingToTarget),
		).
		Watches(
			&solarv1alpha1.RenderTask{},
			handler.EnqueueRequestsFromMapFunc(mapRenderTaskToOwner("Target")),
			builder.WithPredicates(renderTaskStatusChangePredicate()),
		).
		Watches(
			&solarv1alpha1.Registry{},
			handler.EnqueueRequestsFromMapFunc(r.mapRegistryToTargets),
		).
		Watches(
			&solarv1alpha1.RegistryBinding{},
			handler.EnqueueRequestsFromMapFunc(r.mapRegistryBindingToTarget),
		).
		Watches(
			&solarv1alpha1.ReferenceGrant{},
			handler.EnqueueRequestsFromMapFunc(r.mapReferenceGrantToTargets),
		).
		Watches(
			&solarv1alpha1.Release{},
			handler.EnqueueRequestsFromMapFunc(r.mapReleaseToTargets),
		).
		Complete(r)
}

// registryGranted checks whether a ReferenceGrant in registryNamespace permits
// fromNamespace to reference the named registry.
func (r *TargetReconciler) registryGranted(ctx context.Context, registryNamespace, fromNamespace string) (bool, error) {
	grantList := &solarv1alpha1.ReferenceGrantList{}
	if err := r.List(ctx, grantList, client.InNamespace(registryNamespace)); err != nil {
		return false, err
	}
	for i := range grantList.Items {
		grant := &grantList.Items[i]
		if grantPermitsRegistryAccess(grant, fromNamespace) {
			return true, nil
		}
	}

	return false, nil
}

// grantPermitsRegistryAccess returns true if the ReferenceGrant allows a Target in
// fromNamespace to reference Registry resources in the grant's namespace.
func grantPermitsRegistryAccess(grant *solarv1alpha1.ReferenceGrant, fromNamespace string) bool {
	return grantPermits(grant, solarGroup, "Target", fromNamespace, solarGroup, "Registry")
}

// mapRegistryToTargets maps a Registry event to reconcile requests for all
// Targets that reference it — either in the same namespace or cross-namespace.
func (r *TargetReconciler) mapRegistryToTargets(ctx context.Context, obj client.Object) []reconcile.Request {
	reg, ok := obj.(*solarv1alpha1.Registry)
	if !ok {
		return nil
	}

	// Same-namespace targets
	targetList := &solarv1alpha1.TargetList{}
	if err := r.List(ctx, targetList, client.InNamespace(reg.Namespace)); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to list Targets for Registry", "registry", reg.Name)

		return nil
	}

	var requests []reconcile.Request
	for _, t := range targetList.Items {
		if t.Spec.RenderRegistryRef.Name == reg.Name &&
			(t.Spec.RenderRegistryNamespace == "" || t.Spec.RenderRegistryNamespace == reg.Namespace) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      t.Name,
					Namespace: t.Namespace,
				},
			})
		}
	}

	// Cross-namespace targets: find namespaces that have been granted access to
	// registries in reg.Namespace, then check their targets.
	grantList := &solarv1alpha1.ReferenceGrantList{}
	if err := r.List(ctx, grantList, client.InNamespace(reg.Namespace)); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to list ReferenceGrants for cross-namespace Registry mapping")
		return requests
	}

	for i := range grantList.Items {
		grant := &grantList.Items[i]
		if !grantsRegistryResource(grant) {
			continue
		}
		for _, from := range grant.Spec.From {
			if from.Kind != "Target" || from.Group != solarGroup {
				continue
			}
			crossTargets := &solarv1alpha1.TargetList{}
			if err := r.List(ctx, crossTargets, client.InNamespace(from.Namespace)); err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "failed to list cross-namespace Targets", "namespace", from.Namespace)
				continue
			}
			for _, t := range crossTargets.Items {
				if t.Spec.RenderRegistryRef.Name == reg.Name && t.Spec.RenderRegistryNamespace == reg.Namespace {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      t.Name,
							Namespace: t.Namespace,
						},
					})
				}
			}
		}
	}

	return requests
}

// buildPullSecretsLookup lists RegistryBindings for the given target, resolves
// each bound Registry, and returns a map from registry hostname to
// targetPullSecretName. Registries without a targetPullSecretName are included
// with an empty string (anonymous pull).
func (r *TargetReconciler) buildPullSecretsLookup(ctx context.Context, target *solarv1alpha1.Target) (map[string]string, error) {
	rbList := &solarv1alpha1.RegistryBindingList{}
	if err := r.List(ctx, rbList,
		client.InNamespace(target.Namespace),
		client.MatchingFields{indexRegistryBindingTargetName: target.Name},
	); err != nil {
		return nil, err
	}

	type hostEntry struct {
		pullSecret  string
		bindingName string
	}

	lookup := make(map[string]hostEntry, len(rbList.Items))

	for _, rb := range rbList.Items {
		reg := &solarv1alpha1.Registry{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      rb.Spec.RegistryRef.Name,
			Namespace: rb.Namespace,
		}, reg); err != nil {
			return nil, fmt.Errorf("failed to get Registry %s referenced by RegistryBinding %s: %w",
				rb.Spec.RegistryRef.Name, rb.Name, err)
		}

		host := strings.ToLower(reg.Spec.Hostname)
		if prev, ok := lookup[host]; ok && prev.pullSecret != reg.Spec.TargetPullSecretName {
			return nil, fmt.Errorf("conflicting RegistryBindings for host %q: RegistryBinding %s (pull secret %q) vs RegistryBinding %s (pull secret %q)",
				host, prev.bindingName, prev.pullSecret, rb.Name, reg.Spec.TargetPullSecretName)
		}

		lookup[host] = hostEntry{pullSecret: reg.Spec.TargetPullSecretName, bindingName: rb.Name}
	}

	result := make(map[string]string, len(lookup))
	for host, entry := range lookup {
		result[host] = entry.pullSecret
	}

	return result, nil
}

// mapRegistryBindingToTarget maps a RegistryBinding event to a reconcile request
// for the referenced Target.
func (r *TargetReconciler) mapRegistryBindingToTarget(ctx context.Context, obj client.Object) []reconcile.Request {
	rb, ok := obj.(*solarv1alpha1.RegistryBinding)
	if !ok {
		return nil
	}

	if rb.Spec.TargetRef.Name == "" {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      rb.Spec.TargetRef.Name,
				Namespace: rb.Namespace,
			},
		},
	}
}

// mapReferenceGrantToTargets enqueues Targets affected by a ReferenceGrant change
// either because the grant controls Registry access (Target → Registry) or because
// it controls ComponentVersion access (Release → ComponentVersion).
func (r *TargetReconciler) mapReferenceGrantToTargets(ctx context.Context, obj client.Object) []reconcile.Request {
	grant, ok := obj.(*solarv1alpha1.ReferenceGrant)
	if !ok {
		return nil
	}

	var requests []reconcile.Request

	if grantsRegistryResource(grant) {
		for _, from := range grant.Spec.From {
			if from.Kind != "Target" || from.Group != solarGroup {
				continue
			}
			targets := &solarv1alpha1.TargetList{}
			if err := r.List(ctx, targets, client.InNamespace(from.Namespace)); err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "failed to list Targets for ReferenceGrant mapping", "namespace", from.Namespace)
				continue
			}
			for _, t := range targets.Items {
				// Enqueue targets that reference a registry specifically in the grant's namespace
				if t.Spec.RenderRegistryNamespace == grant.Namespace {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      t.Name,
							Namespace: t.Namespace,
						},
					})
				}
			}
		}
	}

	if grantsComponentVersionResource(grant) {
		seen := map[string]struct{}{}
		for _, from := range grant.Spec.From {
			if from.Kind != "Release" || from.Group != solarGroup {
				continue
			}
			bindings := &solarv1alpha1.ReleaseBindingList{}
			if err := r.List(ctx, bindings, client.InNamespace(from.Namespace)); err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "failed to list ReleaseBindings for ComponentVersion grant mapping", "namespace", from.Namespace)
				continue
			}
			for _, rb := range bindings.Items {
				if rb.Spec.TargetRef.Name == "" {
					continue
				}
				targetNs := rb.Namespace
				if rb.Spec.TargetNamespace != "" {
					targetNs = rb.Spec.TargetNamespace
				}
				key := targetNs + "/" + rb.Spec.TargetRef.Name
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      rb.Spec.TargetRef.Name,
						Namespace: targetNs,
					},
				})
			}
		}
	}

	if grantsReleaseBindingToTargetResource(grant) {
		// The grant lives in the Target's namespace and authorizes ReleaseBindings from
		// other namespaces. Enqueue all Targets in the grant's namespace so they pick up
		// the new or removed cross-namespace ReleaseBindings.
		targets := &solarv1alpha1.TargetList{}
		if err := r.List(ctx, targets, client.InNamespace(grant.Namespace)); err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "failed to list Targets for ReleaseBinding grant mapping", "namespace", grant.Namespace)
		} else {
			for _, t := range targets.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      t.Name,
						Namespace: t.Namespace,
					},
				})
			}
		}
	}

	return requests
}

// grantsRegistryResource returns true if the ReferenceGrant includes Registry in its To list.
func grantsRegistryResource(grant *solarv1alpha1.ReferenceGrant) bool {
	for _, t := range grant.Spec.To {
		if t.Kind == "Registry" && t.Group == solarGroup {
			return true
		}
	}

	return false
}

// grantsReleaseBindingToTargetResource returns true if the ReferenceGrant authorizes
// ReleaseBindings in another namespace to reference Targets in the grant's namespace.
func grantsReleaseBindingToTargetResource(grant *solarv1alpha1.ReferenceGrant) bool {
	hasReleaseBindingFrom := false
	for _, f := range grant.Spec.From {
		if f.Kind == "ReleaseBinding" && f.Group == solarGroup {
			hasReleaseBindingFrom = true
			break
		}
	}
	if !hasReleaseBindingFrom {
		return false
	}
	for _, t := range grant.Spec.To {
		if t.Kind == "Target" && t.Group == solarGroup {
			return true
		}
	}

	return false
}

// collectCrossNamespaceReleaseBindings returns ReleaseBindings from other namespaces
// that reference target via spec.targetRef.name + spec.targetNamespace, authorized by
// a ReferenceGrant in target's namespace.
func (r *TargetReconciler) collectCrossNamespaceReleaseBindings(ctx context.Context, target *solarv1alpha1.Target) ([]solarv1alpha1.ReleaseBinding, error) {
	grantList := &solarv1alpha1.ReferenceGrantList{}
	if err := r.List(ctx, grantList, client.InNamespace(target.Namespace)); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var result []solarv1alpha1.ReleaseBinding
	for i := range grantList.Items {
		grant := &grantList.Items[i]
		if !grantsReleaseBindingToTargetResource(grant) {
			continue
		}
		for _, from := range grant.Spec.From {
			if from.Kind != "ReleaseBinding" || from.Group != solarGroup {
				continue
			}
			crossBindings := &solarv1alpha1.ReleaseBindingList{}
			if err := r.List(ctx, crossBindings,
				client.InNamespace(from.Namespace),
				client.MatchingFields{indexReleaseBindingTargetName: target.Name},
			); err != nil {
				return nil, err
			}
			for _, rb := range crossBindings.Items {
				if rb.Spec.TargetNamespace != target.Namespace {
					continue
				}
				key := rb.Namespace + "/" + rb.Name
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				result = append(result, rb)
			}
		}
	}

	return result, nil
}

// mapReleaseToTargets maps a Release event to reconcile requests for all
// Targets that are bound to the release via ReleaseBindings.
func (r *TargetReconciler) mapReleaseToTargets(ctx context.Context, obj client.Object) []reconcile.Request {
	rel, ok := obj.(*solarv1alpha1.Release)
	if !ok {
		return nil
	}

	bindingList := &solarv1alpha1.ReleaseBindingList{}
	if err := r.List(ctx, bindingList,
		client.InNamespace(rel.Namespace),
		client.MatchingFields{indexReleaseBindingReleaseName: rel.Name},
	); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to list ReleaseBindings for Release", "release", rel.Name)

		return nil
	}

	seen := map[string]struct{}{}
	var requests []reconcile.Request

	for _, rb := range bindingList.Items {
		targetNs := rb.Namespace
		if rb.Spec.TargetNamespace != "" {
			targetNs = rb.Spec.TargetNamespace
		}

		key := targetNs + "/" + rb.Spec.TargetRef.Name
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      rb.Spec.TargetRef.Name,
				Namespace: targetNs,
			},
		})
	}

	return requests
}

func (r *TargetReconciler) mapReleaseBindingToTarget(_ context.Context, obj client.Object) []reconcile.Request {
	rb, ok := obj.(*solarv1alpha1.ReleaseBinding)
	if !ok || rb.Spec.TargetRef.Name == "" {
		return nil
	}

	targetNs := rb.Namespace
	if rb.Spec.TargetNamespace != "" {
		targetNs = rb.Spec.TargetNamespace
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      rb.Spec.TargetRef.Name,
				Namespace: targetNs,
			},
		},
	}
}
