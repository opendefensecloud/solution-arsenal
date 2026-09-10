// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// The bootstrap chart creates one OCIRepository/HelmRelease pair per bound
// Release. Both halves are read: the source covers fetching and verifying the
// artifact, the HelmRelease covers applying the chart and the health of what it
// installed. They fail for different reasons, so neither substitutes for the
// other.
var (
	ociRepositoryGVR = schema.GroupVersionResource{
		Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "ocirepositories",
	}
	helmReleaseGVR = schema.GroupVersionResource{
		Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases",
	}
)

// Flux condition types read off the pair. Typed constants live in the
// helm-controller and source-controller API modules; the collector reads
// unstructured objects to avoid pulling both in for four strings.
const (
	condReady       = "Ready"
	condReconciling = "Reconciling"
	condStalled     = "Stalled"
	condRemediated  = "Remediated"
)

// Collector gathers point-in-time facts from the local target cluster.
type Collector struct {
	Client    kubernetes.Interface
	Dynamic   dynamic.Interface
	Namespace string // "" lists across all namespaces
}

// CollectCapacity sums node Allocatable and requested-by-Pods resources.
func (c *Collector) CollectCapacity(ctx context.Context) (ClusterCapacity, error) {
	nodes, err := c.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ClusterCapacity{}, fmt.Errorf("listing nodes: %w", err)
	}

	capacity := ClusterCapacity{
		NodeCount:   int32(len(nodes.Items)), //nolint:gosec // node count from a real cluster never approaches MaxInt32
		Allocatable: corev1.ResourceList{},
		Used:        corev1.ResourceList{},
	}

	for _, n := range nodes.Items {
		addResourceList(capacity.Allocatable, n.Status.Allocatable)
	}

	pods, err := c.Client.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ClusterCapacity{}, fmt.Errorf("listing pods: %w", err)
	}

	for _, p := range pods.Items {
		for _, container := range p.Spec.Containers {
			addResourceList(capacity.Used, container.Resources.Requests)
		}
	}

	return capacity, nil
}

// CollectReleases pairs each bound Release's OCIRepository with its HelmRelease
// and rolls the pair up into a single ReleaseStatus.
func (c *Collector) CollectReleases(ctx context.Context) ([]ReleaseStatus, error) {
	sources, err := c.listByRelease(ctx, ociRepositoryGVR)
	if err != nil {
		return nil, err
	}

	helms, err := c.listByRelease(ctx, helmReleaseGVR)
	if err != nil {
		return nil, err
	}

	keys := make([]releaseKey, 0, len(helms))
	seen := make(map[releaseKey]bool, len(helms))

	for _, objs := range []map[releaseKey]*unstructured.Unstructured{sources, helms} {
		for key := range objs {
			if !seen[key] {
				seen[key] = true

				keys = append(keys, key)
			}
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].namespace != keys[j].namespace {
			return keys[i].namespace < keys[j].namespace
		}

		return keys[i].name < keys[j].name
	})

	out := make([]ReleaseStatus, 0, len(keys))
	for _, key := range keys {
		out = append(out, releaseStatus(key, sources[key], helms[key]))
	}

	return out, nil
}

// releaseKey identifies one OCIRepository/HelmRelease pair. The namespace is
// part of it because Collector.Namespace may be "" (all namespaces), where
// Release names are not unique: without it two Targets' releases of the same
// name collide, and a source from one namespace can be paired with a
// HelmRelease from another.
type releaseKey struct {
	namespace string
	name      string
}

// listByRelease lists one half of the pairs, keyed by Release name.
func (c *Collector) listByRelease(
	ctx context.Context, gvr schema.GroupVersionResource,
) (map[releaseKey]*unstructured.Unstructured, error) {
	list, err := c.Dynamic.Resource(gvr).Namespace(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: ReleaseLabelKey,
	})
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", gvr.Resource, err)
	}

	out := make(map[releaseKey]*unstructured.Unstructured, len(list.Items))

	for i := range list.Items {
		item := &list.Items[i]
		out[releaseKey{namespace: item.GetNamespace(), name: releaseName(item)}] = item
	}

	return out, nil
}

// releaseName reads the untruncated Release name from the annotation, since the
// label of the same name is sha-truncated at 63 characters.
func releaseName(obj *unstructured.Unstructured) string {
	if name := obj.GetAnnotations()[ReleaseAnnotationKey]; name != "" {
		return name
	}

	// Pre-annotation bootstrap charts only carry the (sha-truncated) label.
	return obj.GetLabels()[ReleaseLabelKey]
}

func releaseStatus(key releaseKey, source, helm *unstructured.Unstructured) ReleaseStatus {
	status := ReleaseStatus{Name: key.name, Namespace: key.namespace}

	if source != nil {
		status.SourceConditions = conditionsFromUnstructured(source)
	}

	if helm != nil {
		status.HelmConditions = conditionsFromUnstructured(helm)
		status.Revision = liveChartVersion(helm)
	}

	status.Phase = phaseFromPair(source, helm, status)

	return status
}

// phaseFromPair derives the mutually-exclusive lifecycle state from both halves,
// in the order the states exclude each other: an incomplete pair is Pending, a
// terminal failure outranks a retryable one, and Ready needs both halves to
// agree.
func phaseFromPair(source, helm *unstructured.Unstructured, status ReleaseStatus) ReleasePhase {
	if source == nil || helm == nil {
		return ReleasePending
	}

	src := currentConditions(status.SourceConditions, source.GetGeneration())
	hlm := currentConditions(status.HelmConditions, helm.GetGeneration())

	if conditionIs(src, condStalled, metav1.ConditionTrue) || remediationExhausted(helm, hlm) {
		return ReleaseFailed
	}

	// Degraded outranks Reconciling on purpose. Flux sets Ready=False only after
	// an attempt has actually failed and leaves Reconciling=True while it retries,
	// so a persistently broken release sits in both at once. Checking Reconciling
	// first would report it as Progressing forever and never surface the failure.
	// A genuine first install has Ready Unknown or absent and still lands on
	// Progressing below.
	if conditionIs(src, condReady, metav1.ConditionFalse) ||
		conditionIs(hlm, condReady, metav1.ConditionFalse) {
		return ReleaseDegraded
	}

	if conditionIs(src, condReconciling, metav1.ConditionTrue) ||
		conditionIs(hlm, condReconciling, metav1.ConditionTrue) {
		return ReleaseProgressing
	}

	if conditionIs(src, condReady, metav1.ConditionTrue) &&
		conditionIs(hlm, condReady, metav1.ConditionTrue) {
		return ReleaseReady
	}

	return ReleaseProgressing
}

// currentConditions drops conditions left over from an earlier spec, so a report
// never presents the previous rollout's result as the current one's.
//
// The gate is per condition, not on status.observedGeneration: helm-controller
// parks that field at -1 for the whole time a reconciliation is in flight, so
// gating on it would discard perfectly current conditions and report every
// retrying release as Progressing. Flux stamps each condition with the
// generation it describes, which is the trustworthy signal. A condition with no
// stamp at all is taken at face value.
func currentConditions(conditions []metav1.Condition, generation int64) []metav1.Condition {
	out := make([]metav1.Condition, 0, len(conditions))

	for _, cond := range conditions {
		if cond.ObservedGeneration == 0 || cond.ObservedGeneration == generation {
			out = append(out, cond)
		}
	}

	return out
}

// remediationExhausted reports whether helm-controller has given up: it has
// remediated and the failure count has passed the retry budget in the object's
// own spec. A HelmRelease never sets Stalled on itself -- helm-controller only
// reads that off the source it depends on -- so this is the only way to tell a
// failure that will retry from one that will not.
//
// A missing retries field is treated as "not exhausted". Flux defaults it to 0,
// so that understates a terminal failure as Degraded, which is the safer of the
// two wrong answers.
func remediationExhausted(helm *unstructured.Unstructured, conditions []metav1.Condition) bool {
	if !conditionIs(conditions, condRemediated, metav1.ConditionTrue) {
		return false
	}

	for _, action := range []string{"install", "upgrade"} {
		retries, found, _ := unstructured.NestedInt64(helm.Object, "spec", action, "remediation", "retries")
		if !found || retries < 0 { // negative retries means retry forever
			continue
		}

		if failures, _, _ := unstructured.NestedInt64(helm.Object, "status", action+"Failures"); failures > retries {
			return true
		}
	}

	return false
}

func conditionIs(conditions []metav1.Condition, condType string, status metav1.ConditionStatus) bool {
	for _, cond := range conditions {
		if cond.Type == condType {
			return cond.Status == status
		}
	}

	return false
}

// conditionsFromUnstructured copies status.conditions verbatim. Reason and
// message ride along on each condition, so ReleaseStatus needs no separate
// fields for them.
func conditionsFromUnstructured(obj *unstructured.Unstructured) []metav1.Condition {
	raw, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return nil
	}

	out := make([]metav1.Condition, 0, len(raw))

	for _, c := range raw {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}

		cond := metav1.Condition{}
		cond.Type, _ = m["type"].(string)
		cond.Reason, _ = m["reason"].(string)
		cond.Message, _ = m["message"].(string)

		s, _ := m["status"].(string)
		cond.Status = metav1.ConditionStatus(s)

		if gen, ok := m["observedGeneration"].(int64); ok {
			cond.ObservedGeneration = gen
		}

		if ts, ok := m["lastTransitionTime"].(string); ok {
			parsed, err := time.Parse(time.RFC3339, ts)
			if err == nil {
				cond.LastTransitionTime = metav1.NewTime(parsed)
			}
		}

		out = append(out, cond)
	}

	return out
}

// liveChartVersion reads the chart version actually deployed from the
// HelmRelease's most recent history snapshot. After a rollback this lags the
// desired version, which is the only way to see a remediation from the report.
func liveChartVersion(obj *unstructured.Unstructured) string {
	history, found, _ := unstructured.NestedSlice(obj.Object, "status", "history")
	if !found || len(history) == 0 {
		return ""
	}

	snapshot, ok := history[0].(map[string]any)
	if !ok {
		return ""
	}

	version, _ := snapshot["chartVersion"].(string)

	return version
}

func addResourceList(dst, src corev1.ResourceList) {
	for name, qty := range src {
		total := dst[name]
		total.Add(qty)
		dst[name] = total
	}
}
