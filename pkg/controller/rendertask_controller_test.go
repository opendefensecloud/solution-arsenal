// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"encoding/json"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opendefense.cloud/kit/envtest"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/renderer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("RenderTaskController", Ordered, func() {
	var (
		ctx       = envtest.Context()
		namespace = setupTest(ctx)

		validRendererConfig = renderer.Config{
			Type:          renderer.TypeRelease,
			ReleaseConfig: renderer.ReleaseConfig{},
			PushOptions:   renderer.PushOptions{},
		}

		validRenderTask = func(name string, namespace *corev1.Namespace) *solarv1alpha1.RenderTask {
			b, err := json.Marshal(validRendererConfig)
			Expect(err).ToNot(HaveOccurred())

			return &solarv1alpha1.RenderTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace.Name,
				},
				Spec: solarv1alpha1.RenderTaskSpec{
					Config: runtime.RawExtension{
						Raw: b,
					},
				},
			}
		}
	)

	Describe("RenderTask creation and job scheduling", func() {
		It("should create a RenderTask and schedule a renderer job", func() {
			// Create a RenderTask
			release := validRenderTask("test-config", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Verify the RenderTask was created
			createdRenderTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-config", Namespace: namespace.Name}, createdRenderTask)
			}).Should(Succeed())

			// Verify finalizer was added after a reconciliation cycle
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-config", Namespace: namespace.Name}, createdRenderTask)
				if err != nil {
					return false
				}
				return slices.Contains(createdRenderTask.Finalizers, renderTaskFinalizer)
			}, eventuallyTimeout).Should(BeTrue(), "finalizer should be added by reconciler")

			// Verify config secret was created
			configSecret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-config", Namespace: namespace.Name}, configSecret)
			}, eventuallyTimeout).Should(Succeed())

			Expect(configSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(configSecret.Data).To(HaveKey("config.json"))

			// Verify job was created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-config", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("renderer"))
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal("image:tag"))
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(ContainElement("solar-renderer"))

			// Verify config secret is mounted
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Volumes[0].Name).To(Equal("config"))
			Expect(job.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal("render-test-config"))
		})
	})
})
