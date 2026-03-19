// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReleaseReconciler", Ordered, func() {
	var (
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
					TargetNamespace: "my-namespace",
					Values: runtime.RawExtension{
						Raw: []byte(`{"key": "value"}`),
					},
				},
			}
		}

		validComponentVersion = func(name string, ns *corev1.Namespace) *solarv1alpha1.ComponentVersion {
			return &solarv1alpha1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ComponentVersionSpec{
					ComponentRef: corev1.LocalObjectReference{
						Name: "my-component-v1",
					},
					Tag: "v1.0.0",
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"foo": {Repository: "example.com/resources/foo", Tag: "2.0.0"},
						"bar": {Repository: "example.com/resources/bar", Tag: "3.0.0"},
					},
					Entrypoint: solarv1alpha1.Entrypoint{
						ResourceName: "foo",
						Type:         solarv1alpha1.EntrypointTypeHelm,
					},
				},
			}
		}
	)

	Describe("ComponentVersion resolution", func() {
		It("should set ComponentVersionResolved=True when ComponentVersion exists", func() {
			cv := validComponentVersion("my-component-v1", ns)
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			release := validRelease("test-release-resolved", ns)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Verify the Release was created
			createdRelease := &solarv1alpha1.Release{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release", Namespace: ns.Name}, createdRelease)
			}).Should(Succeed())

			// Verify finalizer was added after a reconciliation cycle
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release", Namespace: ns.Name}, createdRelease)
				if err != nil {
					return false
				}

				return len(createdRelease.Finalizers) > 0 && slices.Contains(createdRelease.Finalizers, releaseFinalizer)
			}, eventuallyTimeout).Should(BeTrue(), "finalizer should be added by reconciler")

			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-release-0", ns.Name)}, task)
			}, eventuallyTimeout).Should(Succeed())

			Expect(task.Spec.RendererConfig.Type).To(Equal(solarv1alpha1.RendererConfigTypeRelease))
			Expect(task.Spec.RendererConfig.ReleaseConfig.Chart.Name).To(Equal("release-test-release"))
			Expect(task.Spec.RendererConfig.ReleaseConfig.Chart.Version).To(Equal("v0.0.0"))
			Expect(task.Spec.Repository).To(Equal(fmt.Sprintf("%s/release-test-release", ns.Name)))
			Expect(task.Spec.RendererConfig.ReleaseConfig.TargetNamespace).To(Equal("my-namespace"))
			Expect(task.Spec.Tag).To(Equal("v0.0.0"))
		})

		It("should propagate FailedJobTTL from Release to RenderTask", func() {
			ttl := int32(3600)
			release := validRelease("test-release-ttl", ns)
			release.Spec.FailedJobTTL = &ttl
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-release-ttl-0", ns.Name)}, task)
			}, eventuallyTimeout).Should(Succeed())

			Expect(task.Spec.FailedJobTTL).ToNot(BeNil())
			Expect(*task.Spec.FailedJobTTL).To(Equal(int32(3600)))
		})
	})

	Describe("Release RenderTask completion", func() {
		It("should represent completion when RenderTask completes successfully", func() {
			// Create a Release
			release := validRelease("test-release-success", ns)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			task := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-release-success-0", ns.Name)}, task)
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
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-resolved", Namespace: ns.Name}, updatedRelease); err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(updatedRelease.Status.Conditions, ConditionTypeComponentVersionResolved)
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should set ComponentVersionResolved=False when ComponentVersion does not exist", func() {
			release := validRelease("test-release-missing-cv", ns)
			release.Spec.ComponentVersionRef.Name = "nonexistent-cv"
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-missing-cv", Namespace: ns.Name}, updatedRelease); err != nil {
					return false
				}

				cond := apimeta.FindStatusCondition(updatedRelease.Status.Conditions, ConditionTypeComponentVersionResolved)

				return cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == "NotFound"
			}, eventuallyTimeout).Should(BeTrue())
		})
	})
})
