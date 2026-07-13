// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// fluxReleaseGVRs are the resource types the bootstrap chart creates one pair
// of per bound Release. Both carry a Ready condition; ociRepository is
// listed alongside helmRelease so a source-side failure (e.g. the chart
// can't be pulled) is visible even before HelmRelease has anything to say.
var fluxReleaseGVRs = []schema.GroupVersionResource{
	{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"},
}

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

// CollectReleases lists Flux HelmRelease objects labeled with
// ReleaseLabelKey and rolls each one's Ready condition into a ReleaseStatus.
func (c *Collector) CollectReleases(ctx context.Context) ([]ReleaseStatus, error) {
	var out []ReleaseStatus

	for _, gvr := range fluxReleaseGVRs {
		list, err := c.Dynamic.Resource(gvr).Namespace(c.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: ReleaseLabelKey,
		})
		if err != nil {
			return nil, fmt.Errorf("listing %s: %w", gvr.Resource, err)
		}

		for _, item := range list.Items {
			out = append(out, releaseStatusFromUnstructured(item))
		}
	}

	return out, nil
}

func releaseStatusFromUnstructured(obj unstructured.Unstructured) ReleaseStatus {
	name := obj.GetAnnotations()[ReleaseAnnotationKey]
	if name == "" {
		// Pre-annotation bootstrap charts only carry the (sha-truncated) label.
		name = obj.GetLabels()[ReleaseLabelKey]
	}

	status := ReleaseStatus{
		Name:           name,
		Phase:          ReleaseProgressing,
		Revision:       liveChartVersion(obj),
		HelmConditions: conditionsFromUnstructured(obj),
	}
	status.Phase = phaseFromHelmRelease(obj, status.HelmConditions)

	return status
}

// conditionsFromUnstructured copies status.conditions verbatim. Reason and
// message ride along on each condition, so ReleaseStatus needs no separate
// fields for them.
func conditionsFromUnstructured(obj unstructured.Unstructured) []metav1.Condition {
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
func liveChartVersion(obj unstructured.Unstructured) string {
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

// phaseFromHelmRelease derives the lifecycle phase from the HelmRelease half of
// the pair.
//
// ponytail: HelmRelease-only, so Failed and Pending are never returned here.
// Failed needs either the OCIRepository's Stalled condition or a comparison of
// status.{install,upgrade}Failures against spec.remediation.retries; Pending
// needs the set of bound Releases to spot a missing pair. Both land with the
// OCIRepository half in #408 -- until then a terminal failure reports Degraded,
// which understates it but never claims a broken Release is healthy.
func phaseFromHelmRelease(obj unstructured.Unstructured, conditions []metav1.Condition) ReleasePhase {
	if !generationObserved(obj) {
		return ReleaseProgressing
	}

	for _, cond := range conditions {
		if cond.Type == "Reconciling" && cond.Status == metav1.ConditionTrue {
			return ReleaseProgressing
		}
	}

	for _, cond := range conditions {
		if cond.Type != "Ready" {
			continue
		}

		switch cond.Status {
		case metav1.ConditionTrue:
			return ReleaseReady
		case metav1.ConditionFalse:
			return ReleaseDegraded
		case metav1.ConditionUnknown:
			return ReleaseProgressing
		}
	}

	return ReleaseProgressing
}

// generationObserved reports whether the conditions describe the current spec.
// Without this gate a report can present the previous rollout's success as the
// current one's.
func generationObserved(obj unstructured.Unstructured) bool {
	observed, found, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")

	return found && observed == obj.GetGeneration()
}

func addResourceList(dst, src corev1.ResourceList) {
	for name, qty := range src {
		total := dst[name]
		total.Add(qty)
		dst[name] = total
	}
}
