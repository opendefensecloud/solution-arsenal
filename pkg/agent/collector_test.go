// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Collector", func() {
	ctx := context.Background()

	Describe("CollectCapacity", func() {
		It("sums node allocatable and pod requests across the cluster", func() {
			client := fake.NewClientset(
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("8Gi"),
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-b"},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						}},
					},
				},
			)

			c := &Collector{Client: client}

			capacity, err := c.CollectCapacity(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(capacity.NodeCount).To(Equal(int32(2)))
			Expect(capacity.Allocatable.Cpu().String()).To(Equal("6"))
			Expect(capacity.Allocatable.Memory().String()).To(Equal("12Gi"))
			Expect(capacity.Used.Cpu().String()).To(Equal("500m"))
			Expect(capacity.Used.Memory().String()).To(Equal("1Gi"))
		})
	})

	Describe("CollectReleases", func() {
		It("rolls up the Ready condition of labeled HelmRelease objects", func() {
			gvr := schema.GroupVersionResource{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"}
			listKinds := map[schema.GroupVersionResource]string{gvr: "HelmReleaseList"}

			hr := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "helm.toolkit.fluxcd.io/v2",
				"kind":       "HelmRelease",
				"metadata": map[string]any{
					"name":      "demo-app",
					"namespace": "tenant-a",
					"labels":    map[string]any{ReleaseLabelKey: "demo-app"},
				},
				"status": map[string]any{
					"conditions": []any{
						map[string]any{
							"type":    "Ready",
							"status":  "True",
							"reason":  "InstallSucceeded",
							"message": "Helm install succeeded",
						},
					},
				},
			}}

			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, hr)

			c := &Collector{Dynamic: dyn, Namespace: "tenant-a"}

			releases, err := c.CollectReleases(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(releases).To(ConsistOf(ReleaseStatus{
				Name:    "demo-app",
				Ready:   true,
				Reason:  "InstallSucceeded",
				Message: "Helm install succeeded",
			}))
		})

		It("reports NoStatus for objects without conditions yet", func() {
			gvr := schema.GroupVersionResource{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"}
			listKinds := map[schema.GroupVersionResource]string{gvr: "HelmReleaseList"}

			hr := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "helm.toolkit.fluxcd.io/v2",
				"kind":       "HelmRelease",
				"metadata": map[string]any{
					"name":      "pending-app",
					"namespace": "tenant-a",
					"labels":    map[string]any{ReleaseLabelKey: "pending-app"},
				},
			}}

			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, hr)

			c := &Collector{Dynamic: dyn, Namespace: "tenant-a"}

			releases, err := c.CollectReleases(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(releases).To(ConsistOf(ReleaseStatus{
				Name:    "pending-app",
				Ready:   false,
				Reason:  "NoStatus",
				Message: "no status.conditions reported yet",
			}))
		})
	})
})
