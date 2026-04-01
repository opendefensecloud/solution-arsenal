// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HydratedTargetReconciler", Ordered, func() {
	var (
		validHydratedTarget = func(name string, ns *corev1.Namespace) *solarv1alpha1.HydratedTarget {
			return &solarv1alpha1.HydratedTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
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
		validRelease = func(name string, ns *corev1.Namespace) *solarv1alpha1.Release {
			return &solarv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
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
		rel := validRelease("my-release", ns)
		rel.Status.ChartURL = fmt.Sprintf("oci://%s/my-release:v0.0.0", ns.Name)
		Expect(k8sClient.Create(ctx, rel)).To(Succeed())
		Expect(k8sClient.Status().Patch(ctx, rel, client.MergeFrom(rel))).To(Succeed())
	})

	Describe("HydratedTarget creation and RenderTask creation", func() {
		It("should create a HydratedTarget and create a RenderTask", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht", ns)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Verify the HydratedTarget was created
			createdHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht", Namespace: ns.Name}, createdHT)
			}).Should(Succeed())

			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-ht-0", ns.Name)}, task)
			}, eventuallyTimeout).Should(Succeed())

			Expect(task.Spec.RendererConfig.Type).To(Equal(solarv1alpha1.RendererConfigTypeHydratedTarget))
			Expect(task.Spec.RendererConfig.HydratedTargetConfig.Chart.Name).To(Equal("ht-test-ht"))
			Expect(task.Spec.RendererConfig.HydratedTargetConfig.Chart.Version).To(Equal("v0.0.0"))
			Expect(task.Spec.Repository).To(Equal(fmt.Sprintf("%s/ht-test-ht", ns.Name)))
			Expect(task.Spec.Tag).To(Equal("v0.0.0"))
		})
	})

	Describe("HydratedTarget RenderTask completion", func() {
		It("should represent completion when RenderTask completes successfully", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-success", ns)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-ht-success-0", ns.Name)}, task)
			}).Should(Succeed())

			// Manipulate Task to be Successful
			Expect(apimeta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeJobSucceeded,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: task.Generation,
				Reason:             ConditionTypeJobSucceeded,
			})).To(BeTrue())
			Expect(k8sClient.Status().Update(ctx, task)).To(Succeed())

			// Verify HydratedTarget has Status Conditions
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-success", Namespace: ns.Name}, updatedHT); err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(updatedHT.Status.Conditions, ConditionTypeTaskCompleted)
			}, eventuallyTimeout).Should(BeTrue())

			condition := apimeta.FindStatusCondition(updatedHT.Status.Conditions, ConditionTypeTaskCompleted)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("TaskCompleted"))
		})

		It("should represent failure when RenderTask failed", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-failed", ns)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-ht-failed-0", ns.Name)}, task)
			}).Should(Succeed())

			// Manipulate Task to be Failed
			Expect(apimeta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeJobFailed,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: task.Generation,
				Reason:             ConditionTypeJobFailed,
			})).To(BeTrue())
			Expect(k8sClient.Status().Update(ctx, task)).To(Succeed())

			// Verify HydratedTarget has Status Conditions
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-failed", Namespace: ns.Name}, updatedHT); err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(updatedHT.Status.Conditions, ConditionTypeTaskFailed)
			}, eventuallyTimeout).Should(BeTrue())

			condition := apimeta.FindStatusCondition(updatedHT.Status.Conditions, ConditionTypeTaskFailed)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("TaskFailed"))
		})
	})

	Describe("HydratedTarget owner references", func() {
		It("should be set for managed resources", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-delete", ns)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete-0", Namespace: ns.Name}, task)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(task.OwnerReferences).To(ContainElement(And(
					HaveField("Kind", "HydratedTarget"),
					HaveField("Name", ht.Name),
					HaveField("UID", ht.UID),
					HaveField("Controller", HaveValue(BeTrue())),
				)))
			}, eventuallyTimeout).Should(Succeed())
		})
	})

	Describe("HydratedTarget status references", func() {
		It("should maintain references to created RenderTask in HydratedTarget status", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-refs", ns)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-ht-refs-0", ns.Name)}, task)
			}).Should(Succeed())

			// Verify HydratedTarget status has references
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-refs", Namespace: ns.Name}, updatedHT)
				if err != nil {
					return false
				}

				return updatedHT.Status.RenderTaskRef != nil
			}).Should(BeTrue())

			// Verify RenderTaskRef details
			Expect(updatedHT.Status.RenderTaskRef.Name).To(Equal(fmt.Sprintf("%s-test-ht-refs-0", ns.Name)))
			Expect(updatedHT.Status.RenderTaskRef.Kind).To(Equal("RenderTask"))
			Expect(updatedHT.Status.RenderTaskRef.APIVersion).To(Equal("solar.opendefense.cloud/v1alpha1"))
		})
	})

	Describe("HydratedTarget updates", func() {
		It("should increase the Generation when the Spec changes", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-gen", ns)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Verify the HydratedTarget was created
			createdHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-gen", Namespace: ns.Name}, createdHT)
			}).Should(Succeed())

			Expect(createdHT.Generation).To(Equal(int64(0)))

			// Update the HydratedTarget
			Eventually(func() error {
				latest := &solarv1alpha1.HydratedTarget{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-gen", Namespace: ns.Name}, latest); err != nil {
					return err
				}
				latest.Spec.Userdata.Raw = []byte(`{"new-shiny-value": true}`)

				return k8sClient.Update(ctx, latest)
			}).Should(Succeed())

			// Check HydratedTarget after Update
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-gen", Namespace: ns.Name}, updatedHT)).To(Succeed())

			Expect(updatedHT.Generation).To(Equal(int64(1)))
		})

		It("should create a RenderTask for the latest Generation only", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-update", ns)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Verify the RenderTask was created
			initialTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-ht-update-0", ns.Name)}, initialTask)
			}).Should(Succeed())

			// Update the HydratedTarget
			Eventually(func() error {
				latest := &solarv1alpha1.HydratedTarget{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-update", Namespace: ns.Name}, latest); err != nil {
					return err
				}
				latest.Spec.Userdata.Raw = []byte(`{"new-shiny-value": true}`)

				return k8sClient.Update(ctx, latest)
			}).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-ht-update-0", ns.Name)}, initialTask)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())

			newTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-ht-update-1", ns.Name)}, newTask)
			}).Should(Succeed())
		})
	})
})
