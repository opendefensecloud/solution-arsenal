// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"slices"

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

var _ = Describe("BootstrapController", Ordered, func() {
	var (
		validBootstrap = func(name string) *solarv1alpha1.Bootstrap {
			return &solarv1alpha1.Bootstrap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.BootstrapSpec{
					Releases: map[string]corev1.LocalObjectReference{
						"rel": {
							Name: "my-release-1",
						},
					},
					Profiles: map[string]corev1.LocalObjectReference{
						"prf-1": {
							Name: "my-profile-1",
						},
						"prf-2": {
							Name: "my-profile-2",
						},
					},
					Userdata: runtime.RawExtension{
						Raw: []byte(`{"key": "value"}`),
					},
				},
			}
		}
		validRelease = func(name string) *solarv1alpha1.Release {
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
		validProfile = func(name string, releaseName string) *solarv1alpha1.Profile {
			return &solarv1alpha1.Profile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ProfileSpec{
					ReleaseRef: corev1.LocalObjectReference{
						Name: releaseName,
					},
				},
			}
		}
		setReleaseStatus = func(rel *solarv1alpha1.Release, tag string) {
			rel.Status.ChartURL = fmt.Sprintf("oci://%s/%s:%s", ns.Name, rel.Name, tag)
			apimeta.SetStatusCondition(&rel.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeTaskCompleted,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: rel.Generation,
			})
		}
	)

	BeforeEach(func() {
		// Create the referenced Releases and Profiles
		rel1 := validRelease("my-release-1")
		Expect(k8sClient.Create(ctx, rel1)).To(Succeed())
		patch1 := client.MergeFrom(rel1.DeepCopy())
		setReleaseStatus(rel1, "v1.1.1")
		Expect(k8sClient.Status().Patch(ctx, rel1, patch1)).To(Succeed())

		rel2 := validRelease("my-release-2")
		Expect(k8sClient.Create(ctx, rel2)).To(Succeed())
		patch2 := client.MergeFrom(rel2.DeepCopy())
		setReleaseStatus(rel2, "v2.2.2")
		Expect(k8sClient.Status().Patch(ctx, rel2, patch2)).To(Succeed())

		prf1 := validProfile("my-profile-1", "my-release-1")
		Expect(k8sClient.Create(ctx, prf1)).To(Succeed())

		prf2 := validProfile("my-profile-2", "my-release-2")
		Expect(k8sClient.Create(ctx, prf2)).To(Succeed())
	})

	Describe("Bootstrap creation and RenderTask creation", func() {
		It("should create a Bootstrap and create a RenderTask", func() {
			// Create a Bootstrap
			bs := validBootstrap("test-bootstrap")
			Expect(k8sClient.Create(ctx, bs)).To(Succeed())

			// Verify the Bootstrap was created
			createdBootstrap := &solarv1alpha1.Bootstrap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap", Namespace: ns.Name}, createdBootstrap)
			}).Should(Succeed())

			// Verify finalizer was added after a reconciliation cycle
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap", Namespace: ns.Name}, createdBootstrap)
				if err != nil {
					return false
				}

				return len(createdBootstrap.Finalizers) > 0 && slices.Contains(createdBootstrap.Finalizers, bootstrapFinalizer)
			}, eventuallyTimeout).Should(BeTrue(), "finalizer should be added by reconciler")

			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-bootstrap-0", ns.Name)}, task)
			}, eventuallyTimeout).Should(Succeed())

			Expect(task.Spec.RendererConfig.Type).To(Equal(solarv1alpha1.RendererConfigTypeBootstrap))
			Expect(task.Spec.RendererConfig.BootstrapConfig.Chart.Name).To(Equal("bootstrap-test-bootstrap"))
			Expect(task.Spec.RendererConfig.BootstrapConfig.Chart.Version).To(Equal("v0.0.0"))

			checkRelease := func(name string, repo string, tag string) {
				Expect(task.Spec.RendererConfig.BootstrapConfig.Input.Releases).To(HaveKey(name))
				Expect(task.Spec.RendererConfig.BootstrapConfig.Input.Releases[name].Repository).To(Equal(repo))
				Expect(task.Spec.RendererConfig.BootstrapConfig.Input.Releases[name].Tag).To(Equal(tag))
			}
			checkRelease("my-release-1", fmt.Sprintf("%s/my-release-1", ns.Name), "v1.1.1")
			checkRelease("my-release-2", fmt.Sprintf("%s/my-release-2", ns.Name), "v2.2.2")

			Expect(task.Spec.Repository).To(Equal(fmt.Sprintf("%s/bootstrap-test-bootstrap", ns.Name)))
			Expect(task.Spec.Tag).To(Equal("v0.0.0"))
		})
	})

	Describe("Bootstrap RenderTask completion", func() {
		It("should represent completion when RenderTask completes successfully", func() {
			// Create a Bootstrap
			bs := validBootstrap("test-bootstrap-success")
			Expect(k8sClient.Create(ctx, bs)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-bootstrap-success-0", ns.Name)}, task)
			}).Should(Succeed())

			// Manipulate Task to be Successful
			Expect(apimeta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeJobSucceeded,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: task.Generation,
				Reason:             ConditionTypeJobSucceeded,
			})).To(BeTrue())
			Expect(k8sClient.Status().Update(ctx, task)).To(Succeed())

			// Verify Bootstrap has Status Conditions
			updatedBootstrap := &solarv1alpha1.Bootstrap{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap-success", Namespace: ns.Name}, updatedBootstrap); err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(updatedBootstrap.Status.Conditions, ConditionTypeTaskCompleted)
			}, eventuallyTimeout).Should(BeTrue())

			condition := apimeta.FindStatusCondition(updatedBootstrap.Status.Conditions, ConditionTypeTaskCompleted)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("TaskCompleted"))
		})

		It("should represent failure when RenderTask failed", func() {
			// Create a Bootstrap
			bs := validBootstrap("test-bootstrap-failed")
			Expect(k8sClient.Create(ctx, bs)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-bootstrap-failed-0", ns.Name)}, task)
			}).Should(Succeed())

			// Manipulate Task to be Failed
			Expect(apimeta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeJobFailed,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: task.Generation,
				Reason:             ConditionTypeJobFailed,
			})).To(BeTrue())
			Expect(k8sClient.Status().Update(ctx, task)).To(Succeed())

			// Verify Bootstrap has Status Conditions
			updatedBootstrap := &solarv1alpha1.Bootstrap{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap-failed", Namespace: ns.Name}, updatedBootstrap); err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(updatedBootstrap.Status.Conditions, ConditionTypeTaskFailed)
			}, eventuallyTimeout).Should(BeTrue())

			condition := apimeta.FindStatusCondition(updatedBootstrap.Status.Conditions, ConditionTypeTaskFailed)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("TaskFailed"))
		})
	})

	Describe("Bootstrap deletion", func() {
		It("should cleanup RenderTask when Bootstrap is deleted", func() {
			// Create a Bootstrap
			bs := validBootstrap("test-bootstrap-delete")
			Expect(k8sClient.Create(ctx, bs)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-bootstrap-delete-0", ns.Name)}, task)
			}).Should(Succeed())

			// Delete the Bootstrap
			createdBootstrap := &solarv1alpha1.Bootstrap{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap-delete", Namespace: ns.Name}, createdBootstrap)).To(Succeed())
			Expect(k8sClient.Delete(ctx, createdBootstrap)).To(Succeed())

			// Verify RenderTask is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-bootstrap-delete-0", ns.Name)}, task)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify Bootstrap is deleted (finalizer removed)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap-delete", Namespace: ns.Name}, createdBootstrap)
			}).Should(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("Bootstrap status references", func() {
		It("should maintain references to created RenderTask in Bootstrap status", func() {
			// Create a Bootstrap
			bs := validBootstrap("test-bootstrap-refs")
			Expect(k8sClient.Create(ctx, bs)).To(Succeed())

			// Wait for RenderTask to be created
			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-bootstrap-refs-0", ns.Name)}, task)
			}).Should(Succeed())

			// Verify Bootstrap status has references
			updatedBootstrap := &solarv1alpha1.Bootstrap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap-refs", Namespace: ns.Name}, updatedBootstrap)
				if err != nil {
					return false
				}

				return updatedBootstrap.Status.RenderTaskRef != nil
			}).Should(BeTrue())

			// Verify RenderTaskRef details
			Expect(updatedBootstrap.Status.RenderTaskRef.Name).To(Equal(fmt.Sprintf("%s-test-bootstrap-refs-0", ns.Name)))
			Expect(updatedBootstrap.Status.RenderTaskRef.Kind).To(Equal("RenderTask"))
			Expect(updatedBootstrap.Status.RenderTaskRef.APIVersion).To(Equal("solar.opendefense.cloud/v1alpha1"))
		})
	})

	Describe("Bootstrap updates", func() {
		It("should increase the Generation when the Spec changes", func() {
			// Create a Bootstrap
			bs := validBootstrap("test-bootstrap-gen")
			Expect(k8sClient.Create(ctx, bs)).To(Succeed())

			// Verify the Bootstrap was created
			createdBootstrap := &solarv1alpha1.Bootstrap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap-gen", Namespace: ns.Name}, createdBootstrap)
			}).Should(Succeed())

			Expect(createdBootstrap.Generation).To(Equal(int64(0)))

			// Update the Bootstrap
			Eventually(func() error {
				latest := &solarv1alpha1.Bootstrap{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap-gen", Namespace: ns.Name}, latest); err != nil {
					return err
				}
				latest.Spec.Userdata.Raw = []byte(`{"new-shiny-value": true}`)

				return k8sClient.Update(ctx, latest)
			}).Should(Succeed())

			// Check Bootstrap after Update
			updatedBootstrap := &solarv1alpha1.Bootstrap{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap-gen", Namespace: ns.Name}, updatedBootstrap)).To(Succeed())

			Expect(updatedBootstrap.Generation).To(Equal(int64(1)))
		})

		It("should create a RenderTask for the latest Generation only", func() {
			// Create a Bootstrap
			bs := validBootstrap("test-bootstrap-update")
			Expect(k8sClient.Create(ctx, bs)).To(Succeed())

			// Verify the RenderTask was created
			initialTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-bootstrap-update-0", ns.Name)}, initialTask)
			}).Should(Succeed())

			// Update the Bootstrap
			Eventually(func() error {
				latest := &solarv1alpha1.Bootstrap{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-bootstrap-update", Namespace: ns.Name}, latest); err != nil {
					return err
				}
				latest.Spec.Userdata.Raw = []byte(`{"new-shiny-value": true}`)

				return k8sClient.Update(ctx, latest)
			}).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-bootstrap-update-0", ns.Name)}, initialTask)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())

			newTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-bootstrap-update-1", ns.Name)}, newTask)
			}).Should(Succeed())
		})
	})
})
