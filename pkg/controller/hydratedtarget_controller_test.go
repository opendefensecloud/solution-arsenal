// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"go.opendefense.cloud/kit/envtest"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("HydratedTargetReconciler", Ordered, func() {
	var (
		ctx       = envtest.Context()
		namespace = setupTest(ctx)

		validHydratedTarget = func(name string, namespace *corev1.Namespace) *solarv1alpha1.HydratedTarget {
			return &solarv1alpha1.HydratedTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace.Name,
				},
				Spec: solarv1alpha1.HydratedTargetSpec{
					Releases: map[string]corev1.LocalObjectReference{
						"my-release": {
							Name: "my-release",
						},
					},
					Userdata: runtime.RawExtension{
						Raw: []byte(`{"key": "value"}`),
					},
				},
			}
		}

		validRelease = func(name string, namespace *corev1.Namespace) *solarv1alpha1.Release {
			return &solarv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace.Name,
				},
				Spec: solarv1alpha1.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{
						Name: "my-component-v1",
					},
					Values: runtime.RawExtension{
						Raw: []byte(`{"key": "value"}`),
					},
				},
			}
		}
	)

	BeforeEach(func() {
		// Create the referenced Release
		rel := validRelease("my-release", namespace)
		rel.Status.ChartURL = fmt.Sprintf("oci://%s/my-release:v0.0.0", namespace.Name)
		Expect(k8sClient.Create(ctx, rel)).To(Succeed())
		Expect(k8sClient.Status().Patch(ctx, rel, client.MergeFrom(rel))).To(Succeed())
	})

	Describe("HydratedTarget creation and job scheduling", func() {
		It("should create a HydratedTarget and schedule a renderer job", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Verify the HydratedTarget was created
			createdHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht", Namespace: namespace.Name}, createdHT)
			}).Should(Succeed())

			// Verify finalizer was added after a reconciliation cycle
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht", Namespace: namespace.Name}, createdHT)
				if err != nil {
					return false
				}
				return len(createdHT.Finalizers) > 0 && slices.Contains(createdHT.Finalizers, hydratedTargetFinalizer)
			}, eventuallyTimeout).Should(BeTrue(), "finalizer should be added by reconciler")

			//			TODO: Verify RenderTask was created
			//			task := &solarv1alpha1.RenderTask{}
			//			Eventually(func() error {
			//				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release", Namespace: namespace.Name}, task)
			//			}, eventuallyTimeout).Should(Succeed())

			// TODO: verify RenderTask
		})
	})

	Describe("HydratedTarget RenderTask completion", func() {
		It("should represent completion when RenderTask completes successfully", func() {
			// TODO
		})

		It("should represent failure when RenderTask failed", func() {
			// TODO
		})
	})

	Describe("HydratedTarget deletion", func() {
		It("should cleanup RenderTask when HydratedTarget is deleted", func() {
			// Create a HydratedTarget
			release := validHydratedTarget("test-ht-delete", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// TODO
			//			// Wait for RenderTask to be created
			//			task := &solarv1alpha1.RenderTask{}
			//			Eventually(func() error {
			//				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete", Namespace: namespace.Name}, task)
			//			}).Should(Succeed())
			//
			//			// Delete the HydratedTarget
			//			createdHT := &solarv1alpha1.HydratedTarget{}
			//			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete", Namespace: namespace.Name}, createdHT)).To(Succeed())
			//			Expect(k8sClient.Delete(ctx, createdHT)).To(Succeed())
			//
			//			// Verify RenderTask is deleted
			//			Eventually(func() error {
			//				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete", Namespace: namespace.Name}, task)
			//			}).Should(MatchError(ContainSubstring("not found")))
			//
			//			// Verify HydratedTarget is deleted (finalizer removed)
			//			Eventually(func() error {
			//				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete", Namespace: namespace.Name}, createdHT)
			//			}).Should(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("HydratedTarget status references", func() {
		It("should maintain references to created job and secret in HydratedTarget status", func() {
			// Create a HydratedTarget
			release := validHydratedTarget("test-ht-refs", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// TODO
			//			// Wait for RenderTask to be created
			//			task := &solarv1alpha1.RenderTask{}
			//			Eventually(func() error {
			//				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-refs", Namespace: namespace.Name}, task)
			//			}).Should(Succeed())
			//
			//			// Verify HydratedTarget status has references
			//			updatedHT := &solarv1alpha1.HydratedTarget{}
			//			Eventually(func() bool {
			//				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-refs", Namespace: namespace.Name}, updatedHT)
			//				if err != nil {
			//					return false
			//				}
			//				return updatedHT.Status.RenderTaskRef != nil
			//			}).Should(BeTrue())
			//
			//			// Verify RenderTaskRef details
			//			Expect(updatedHT.Status.RenderTaskRef.Name).To(Equal("test-ht-refs"))
			//			Expect(updatedHT.Status.RenderTaskRef.Namespace).To(Equal(namespace.Name))
			//			Expect(updatedHT.Status.RenderTaskRef.Kind).To(Equal("RenderTask"))
			//			Expect(updatedHT.Status.RenderTaskRef.APIVersion).To(Equal("solar..."))
		})
	})
})
