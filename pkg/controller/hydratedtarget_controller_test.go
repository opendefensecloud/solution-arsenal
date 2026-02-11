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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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

	Describe("HydratedTarget creation and RenderTask creation", func() {
		It("should create a HydratedTarget and create a RenderTask", func() {
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

			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-0", Namespace: namespace.Name}, task)
			}, eventuallyTimeout).Should(Succeed())

			Expect(task.Spec.RendererConfig.Type).To(Equal(solarv1alpha1.RendererConfigTypeHydratedTarget))
			Expect(task.Spec.RendererConfig.HydratedTargetConfig.Chart.Name).To(Equal("test-ht"))
			Expect(task.Spec.RendererConfig.HydratedTargetConfig.Chart.Version).To(Equal("v0.0.0"))
			Expect(task.Spec.RendererConfig.PushOptions.ReferenceURL).To(ContainSubstring("test-ht:v0.0.0"))
			Expect(task.Spec.RendererConfig.PushOptions.ReferenceURL).To(ContainSubstring("oci://"))
		})
	})

	Describe("HydratedTarget RenderTask completion", func() {
		It("should represent failure when RenderTask failed", Pending, func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-failed", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-failed-0", Namespace: namespace.Name}, task)
			}).Should(Succeed())

			// Set the JobFailed condition
			now := metav1.Now()
			failedCondition := metav1.Condition{
				Type:               ConditionTypeJobFailed,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: now,
				ObservedGeneration: task.Generation,
			}
			Expect(apimeta.SetStatusCondition(&task.Status.Conditions, failedCondition)).To(BeTrue())
			Expect(k8sClient.Status().Update(ctx, task)).To(Succeed())

			// Verify HydratedTarget has Status Conditions
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-failed", Namespace: namespace.Name}, updatedHT); err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedHT.Status.Conditions, ConditionTypeTaskFailed)
			}, eventuallyTimeout).Should(BeTrue())

			condition := apimeta.FindStatusCondition(updatedHT.Status.Conditions, ConditionTypeTaskFailed)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("TaskFailed"))
		})

		It("should represent completion when RenderTask completes successfully", Pending, func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-success", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-success", Namespace: namespace.Name}, task)
			}).Should(Succeed())

			// Manipulate the RenderTask
			now := metav1.Now()
			// Set the Success condition
			taskCondition := metav1.Condition{
				Type:               ConditionTypeJobSucceeded,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: now,
			}
			task.Status.Conditions = append(task.Status.Conditions, taskCondition)
			Expect(k8sClient.Status().Update(ctx, task)).To(Succeed())

			// Verify HydratedTarget has Status Conditions
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-success", Namespace: namespace.Name}, updatedHT); err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedHT.Status.Conditions, ConditionTypeTaskCompleted)
			}, eventuallyTimeout).Should(BeTrue())

			condition := apimeta.FindStatusCondition(updatedHT.Status.Conditions, ConditionTypeTaskCompleted)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("TaskCompleted"))
		})
	})

	Describe("HydratedTarget deletion", func() {
		It("should cleanup RenderTask when HydratedTarget is deleted", func() {
			// Create a HydratedTarget
			release := validHydratedTarget("test-ht-delete", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete-0", Namespace: namespace.Name}, task)
			}).Should(Succeed())

			// Delete the HydratedTarget
			createdHT := &solarv1alpha1.HydratedTarget{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete", Namespace: namespace.Name}, createdHT)).To(Succeed())
			Expect(k8sClient.Delete(ctx, createdHT)).To(Succeed())

			// Verify RenderTask is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete-0", Namespace: namespace.Name}, task)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify HydratedTarget is deleted (finalizer removed)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete", Namespace: namespace.Name}, createdHT)
			}).Should(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("HydratedTarget status references", func() {
		It("should maintain references to created job and secret in HydratedTarget status", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-refs", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-refs-0", Namespace: namespace.Name}, task)
			}).Should(Succeed())

			// Verify HydratedTarget status has references
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-refs", Namespace: namespace.Name}, updatedHT)
				if err != nil {
					return false
				}
				return updatedHT.Status.RenderTaskRef != nil
			}).Should(BeTrue())

			// Verify RenderTaskRef details
			Expect(updatedHT.Status.RenderTaskRef.Name).To(Equal("test-ht-refs-0"))
			Expect(updatedHT.Status.RenderTaskRef.Namespace).To(Equal(namespace.Name))
			Expect(updatedHT.Status.RenderTaskRef.Kind).To(Equal("RenderTask"))
			Expect(updatedHT.Status.RenderTaskRef.APIVersion).To(Equal("solar.opendefense.cloud/v1alpha1"))
		})
	})

	Describe("HydratedTarget updates", func() {
		It("should increase the Generation when the Spec changes", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-gen", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Verify the HydratedTarget was created
			createdHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-gen", Namespace: namespace.Name}, createdHT)
			}).Should(Succeed())

			Expect(createdHT.Generation).To(Equal(int64(0)))

			// Update the HydratedTarget
			Eventually(func() error {
				latest := &solarv1alpha1.HydratedTarget{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-gen", Namespace: namespace.Name}, latest); err != nil {
					return err
				}
				latest.Spec.Userdata.Raw = []byte(`{"new-shiny-value": true}`)
				return k8sClient.Update(ctx, latest)
			}).Should(Succeed())

			// Check HydratedTarget after Update
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-gen", Namespace: namespace.Name}, updatedHT)).To(Succeed())

			Expect(updatedHT.Generation).To(Equal(int64(1)))
		})

		It("should create a RenderTask for the latest Generation only", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-update", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Verify the RenderTask was created
			initialTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-update-0", Namespace: namespace.Name}, initialTask)
			}).Should(Succeed())

			// Update the HydratedTarget
			Eventually(func() error {
				latest := &solarv1alpha1.HydratedTarget{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-update", Namespace: namespace.Name}, latest); err != nil {
					return err
				}
				latest.Spec.Userdata.Raw = []byte(`{"new-shiny-value": true}`)
				return k8sClient.Update(ctx, latest)
			}).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-update-0", Namespace: namespace.Name}, initialTask)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())

			newTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-update-1", Namespace: namespace.Name}, newTask)
			}).Should(Succeed())
		})
	})
})
