// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"

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
	status := ReleaseStatus{Name: obj.GetLabels()[ReleaseLabelKey]}

	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		status.Reason = "NoStatus"
		status.Message = "no status.conditions reported yet"

		return status
	}

	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok || cond["type"] != "Ready" {
			continue
		}

		status.Ready = cond["status"] == "True"
		status.Reason, _ = cond["reason"].(string)
		status.Message, _ = cond["message"].(string)
	}

	return status
}

func addResourceList(dst, src corev1.ResourceList) {
	for name, qty := range src {
		total := dst[name]
		total.Add(qty)
		dst[name] = total
	}
}
