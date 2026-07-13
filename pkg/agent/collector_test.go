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
		gvr := schema.GroupVersionResource{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"}
		listKinds := map[schema.GroupVersionResource]string{gvr: "HelmReleaseList"}

		// helmRelease builds a HelmRelease whose label is sha-truncated (as the
		// bootstrap chart does for long names) while the annotation keeps the real one.
		helmRelease := func(name string, generation int64, status map[string]any) *unstructured.Unstructured {
			return &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "helm.toolkit.fluxcd.io/v2",
				"kind":       "HelmRelease",
				"metadata": map[string]any{
					"name":        "demo-app",
					"namespace":   "tenant-a",
					"generation":  generation,
					"labels":      map[string]any{ReleaseLabelKey: "truncated-3f2a1b9c0d"},
					"annotations": map[string]any{ReleaseAnnotationKey: name},
				},
				"status": status,
			}}
		}

		collect := func(objs ...runtime.Object) []ReleaseStatus {
			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objs...)
			c := &Collector{Dynamic: dyn, Namespace: "tenant-a"}

			releases, err := c.CollectReleases(ctx)
			Expect(err).NotTo(HaveOccurred())

			return releases
		}

		It("reports Ready, names the release from the annotation and records the live chart version", func() {
			hr := helmRelease("a-very-long-release-name-that-the-label-cannot-hold", 3, map[string]any{
				"observedGeneration": int64(3),
				"history":            []any{map[string]any{"chartVersion": "1.4.2"}},
				"conditions": []any{
					map[string]any{
						"type": "Ready", "status": "True",
						"reason": "InstallSucceeded", "message": "Helm install succeeded",
					},
				},
			})

			releases := collect(hr)
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Name).To(Equal("a-very-long-release-name-that-the-label-cannot-hold"))
			Expect(releases[0].Phase).To(Equal(ReleaseReady))
			Expect(releases[0].Revision).To(Equal("1.4.2"))
			Expect(releases[0].HelmConditions).To(HaveLen(1))
			Expect(releases[0].HelmConditions[0].Reason).To(Equal("InstallSucceeded"))
		})

		It("reports Progressing for an object with no status yet", func() {
			releases := collect(helmRelease("pending-app", 1, map[string]any{}))
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(ReleaseProgressing))
			Expect(releases[0].HelmConditions).To(BeEmpty())
			Expect(releases[0].Revision).To(BeEmpty())
		})

		It("reports Progressing when Ready=True describes a stale generation", func() {
			hr := helmRelease("stale-app", 5, map[string]any{
				"observedGeneration": int64(4),
				"conditions": []any{
					map[string]any{"type": "Ready", "status": "True", "reason": "UpgradeSucceeded"},
				},
			})

			releases := collect(hr)
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(ReleaseProgressing))
		})

		It("reports Degraded when Ready=False", func() {
			hr := helmRelease("broken-app", 2, map[string]any{
				"observedGeneration": int64(2),
				"conditions": []any{
					map[string]any{"type": "Ready", "status": "False", "reason": "UpgradeFailed"},
				},
			})

			releases := collect(hr)
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(ReleaseDegraded))
		})
	})
})
