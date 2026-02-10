// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
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

var _ = Describe("ReleaseReconciler", Ordered, func() {
	var (
		ctx       = envtest.Context()
		namespace = setupTest(ctx)

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

		validComponentVersion = func(name string, namespace *corev1.Namespace) *solarv1alpha1.ComponentVersion {
			return &solarv1alpha1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace.Name,
				},
				Spec: solarv1alpha1.ComponentVersionSpec{
					ComponentRef: corev1.LocalObjectReference{
						Name: "my-component-v1",
					},
					Tag: "v1.0.0",
					Helm: solarv1alpha1.ResourceAccess{
						Repository: "example.com/helm",
						Tag:        "1.0.1",
					},
					KRO: solarv1alpha1.ResourceAccess{
						Repository: "example.com/kro",
						Tag:        "^1.0",
					},
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"foo": {Repository: "example.com/resources/foo", Tag: "2.0.0"},
						"bar": {Repository: "example.com/resources/bar", Tag: "3.0.0"},
					},
				},
			}
		}
	)

	BeforeEach(func() {
		// Create the Componentversion
		cv := validComponentVersion("my-component-v1", namespace)
		Expect(k8sClient.Create(ctx, cv)).To(Succeed())
	})

	Describe("Release creation and RenderTask scheduling", func() {
		It("should create a Release and create a RenderTask", func() {
			// Create a Release
			release := validRelease("test-release", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Verify the Release was created
			createdRelease := &solarv1alpha1.Release{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release", Namespace: namespace.Name}, createdRelease)
			}).Should(Succeed())

			// Verify finalizer was added after a reconciliation cycle
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release", Namespace: namespace.Name}, createdRelease)
				if err != nil {
					return false
				}
				return len(createdRelease.Finalizers) > 0 && slices.Contains(createdRelease.Finalizers, releaseFinalizer)
			}, eventuallyTimeout).Should(BeTrue(), "finalizer should be added by reconciler")

			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release", Namespace: namespace.Name}, task)
			}, eventuallyTimeout).Should(Succeed())

			Expect(task.Spec.RendererConfig.Type).To(Equal(solarv1alpha1.RendererConfigTypeRelease))
			Expect(task.Spec.RendererConfig.ReleaseConfig.Chart.Name).To(Equal("test-release"))
			Expect(task.Spec.RendererConfig.ReleaseConfig.Chart.Version).To(Equal("v0.0.0"))
			Expect(task.Spec.RendererConfig.PushOptions.ReferenceURL).To(ContainSubstring("test-release:v0.0.0"))
			Expect(task.Spec.RendererConfig.PushOptions.ReferenceURL).To(ContainSubstring("oci://"))
		})
	})

	Describe("Release RenderTask completion", func() {
		It("should represent completion when RenderTask completes successfully", Pending, func() {
		})

		It("should represent failure when RenderTask failed", Pending, func() {
		})
	})

	Describe("Release deletion", func() {
		It("should cleanup RenderTask when Release is deleted", func() {
			// Create a Release
			release := validRelease("test-release-delete", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete", Namespace: namespace.Name}, task)
			}).Should(Succeed())

			// Delete the Release
			createdRelease := &solarv1alpha1.Release{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete", Namespace: namespace.Name}, createdRelease)).To(Succeed())
			Expect(k8sClient.Delete(ctx, createdRelease)).To(Succeed())

			// Verify RenderTask is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete", Namespace: namespace.Name}, task)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify Release is deleted (finalizer removed)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete", Namespace: namespace.Name}, createdRelease)
			}).Should(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("Release status references", func() {
		It("should maintain references to created RenderTask in Release status", func() {
			// Create a Release
			release := validRelease("test-release-refs", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-refs", Namespace: namespace.Name}, task)
			}).Should(Succeed())

			// Verify Release status has references
			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-refs", Namespace: namespace.Name}, updatedRelease)
				if err != nil {
					return false
				}
				return updatedRelease.Status.RenderTaskRef != nil
			}).Should(BeTrue())

			// Verify RenderTaskRef details
			Expect(updatedRelease.Status.RenderTaskRef.Name).To(Equal("test-release-refs"))
			Expect(updatedRelease.Status.RenderTaskRef.Namespace).To(Equal(namespace.Name))
			Expect(updatedRelease.Status.RenderTaskRef.Kind).To(Equal("RenderTask"))
			Expect(updatedRelease.Status.RenderTaskRef.APIVersion).To(Equal("solar.opendefense.cloud/v1alpha1"))
		})
	})
})
