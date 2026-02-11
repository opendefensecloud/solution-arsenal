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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-0", Namespace: namespace.Name}, task)
			}, eventuallyTimeout).Should(Succeed())

			Expect(task.Spec.RendererConfig.Type).To(Equal(solarv1alpha1.RendererConfigTypeRelease))
			Expect(task.Spec.RendererConfig.ReleaseConfig.Chart.Name).To(Equal("test-release"))
			Expect(task.Spec.RendererConfig.ReleaseConfig.Chart.Version).To(Equal("v0.0.0"))
			Expect(task.Spec.RendererConfig.PushOptions.ReferenceURL).To(ContainSubstring("test-release:v0.0.0"))
			Expect(task.Spec.RendererConfig.PushOptions.ReferenceURL).To(ContainSubstring("oci://"))
		})
	})

	Describe("Release RenderTask completion", func() {
		It("should represent completion when RenderTask completes successfully", func() {
			// Create a Release
			release := validRelease("test-release-success", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-success-0", Namespace: namespace.Name}, task)
			}, eventuallyTimeout).Should(Succeed())

			// Manipulate Task to be Successful
			Expect(apimeta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeJobSucceeded,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: task.Generation,
				Reason:             ConditionTypeJobSucceeded,
			})).To(BeTrue())
			Expect(k8sClient.Status().Update(ctx, task)).To(Succeed())

			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-success", Namespace: namespace.Name}, updatedRelease); err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedRelease.Status.Conditions, ConditionTypeTaskCompleted)
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should represent failure when RenderTask failed", func() {
			// Create a Release
			release := validRelease("test-release-failed", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-failed-0", Namespace: namespace.Name}, task)
			}, eventuallyTimeout).Should(Succeed())

			// Manipulate Task to be Failed
			Expect(apimeta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeJobFailed,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: task.Generation,
				Reason:             ConditionTypeJobFailed,
			})).To(BeTrue())
			Expect(k8sClient.Status().Update(ctx, task)).To(Succeed())

			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-failed", Namespace: namespace.Name}, updatedRelease); err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedRelease.Status.Conditions, ConditionTypeTaskFailed)
			}, eventuallyTimeout).Should(BeTrue())
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
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete-0", Namespace: namespace.Name}, task)
			}).Should(Succeed())

			// Delete the Release
			createdRelease := &solarv1alpha1.Release{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete", Namespace: namespace.Name}, createdRelease)).To(Succeed())
			Expect(k8sClient.Delete(ctx, createdRelease)).To(Succeed())

			// Verify RenderTask is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete-0", Namespace: namespace.Name}, task)
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
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-refs-0", Namespace: namespace.Name}, task)
			}, eventuallyTimeout).Should(Succeed())

			// Verify Release status has references
			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-refs", Namespace: namespace.Name}, updatedRelease)
				if err != nil {
					return false
				}
				return updatedRelease.Status.RenderTaskRef != nil
			}, eventuallyTimeout).Should(BeTrue())

			// Verify RenderTaskRef details
			Expect(updatedRelease.Status.RenderTaskRef.Name).To(Equal("test-release-refs-0"))
			Expect(updatedRelease.Status.RenderTaskRef.Namespace).To(Equal(namespace.Name))
			Expect(updatedRelease.Status.RenderTaskRef.Kind).To(Equal("RenderTask"))
			Expect(updatedRelease.Status.RenderTaskRef.APIVersion).To(Equal("solar.opendefense.cloud/v1alpha1"))
		})
	})

	Describe("Release updates", func() {
		It("should increase the Generation when the Spec changes", func() {
			// Create a Release
			release := validRelease("test-release-gen", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Verify the Release was created
			createdRelease := &solarv1alpha1.Release{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-gen", Namespace: namespace.Name}, createdRelease)
			}).Should(Succeed())

			Expect(createdRelease.Generation).To(Equal(int64(0)))

			// Update the Release
			Eventually(func() error {
				latest := &solarv1alpha1.Release{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-gen", Namespace: namespace.Name}, latest); err != nil {
					return err
				}
				latest.Spec.Values.Raw = []byte(`{"new-shiny-value": true}`)
				return k8sClient.Update(ctx, latest)
			}).Should(Succeed())

			// Check Release after Update
			updatedRelease := &solarv1alpha1.Release{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-gen", Namespace: namespace.Name}, updatedRelease)).To(Succeed())

			Expect(updatedRelease.Generation).To(Equal(int64(1)))
		})

		It("should create a RenderTask for the latest Generation only", func() {
			// Create a Release
			release := validRelease("test-release-update", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Verify the RenderTask was created
			initialTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-update-0", Namespace: namespace.Name}, initialTask)
			}).Should(Succeed())

			// Update the Release
			Eventually(func() error {
				latest := &solarv1alpha1.Release{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-update", Namespace: namespace.Name}, latest); err != nil {
					return err
				}
				latest.Spec.Values.Raw = []byte(`{"new-shiny-value": true}`)
				return k8sClient.Update(ctx, latest)
			}).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-update-0", Namespace: namespace.Name}, initialTask)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())

			newTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-update-1", Namespace: namespace.Name}, newTask)
			}).Should(Succeed())
		})
	})
})
