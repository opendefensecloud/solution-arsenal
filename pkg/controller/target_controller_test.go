// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"go.opendefense.cloud/kit/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TargetController", Ordered, func() {
	var (
		ctx = envtest.Context()
		ns  = setupTest(ctx)
	)

	Context("when reconciling Target", Label("target"), func() {
		It("should create HydratedTarget for Target", func() {
			target := &solarv1alpha1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.TargetSpec{
					Userdata: runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
					Releases: map[string]corev1.LocalObjectReference{
						"example-release": {Name: "initial-release-name"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			hydratedTarget := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(target), hydratedTarget)
			}).Should(Succeed())
			Expect(hydratedTarget).NotTo(BeNil())
		})
	})

	Context("when Target is deleted", Label("target"), func() {
		It("should clean up HydratedTarget", func() {
			target := &solarv1alpha1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target-to-delete",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.TargetSpec{
					Userdata: runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
					Releases: map[string]corev1.LocalObjectReference{
						"example-release": {Name: "initial-release-name"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			hydratedTarget := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(target), hydratedTarget)
			}).Should(Succeed())
			Expect(hydratedTarget).NotTo(BeNil())

			Expect(k8sClient.Delete(ctx, target)).To(Succeed())

			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(target), hydratedTarget)
			}).ShouldNot(Succeed())
		})
	})

	Context("when Target is updated", Label("target"), func() {
		It("should update HydratedTarget", func() {
			target := &solarv1alpha1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target-to-update",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.TargetSpec{
					Userdata: runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
					Releases: map[string]corev1.LocalObjectReference{
						"example-release": {Name: "initial-release-name"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			hydratedTarget := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(target), hydratedTarget)
			}).Should(Succeed())
			Expect(hydratedTarget.Spec.Releases).To(Equal(target.Spec.Releases))

			// Get fresh version of Target and update example-release
			latestTarget := &solarv1alpha1.Target{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(target), latestTarget)).To(Succeed())
			latestTarget.Spec.Releases["example-release"] = corev1.LocalObjectReference{Name: "updated-release-name"}
			Expect(k8sClient.Update(ctx, latestTarget)).To(Succeed())

			// Verify HydratedTarget has been updated by the controller
			Eventually(func() bool {
				ht := &solarv1alpha1.HydratedTarget{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), ht)
				if err != nil {
					return false
				}
				if release, exists := ht.Spec.Releases["example-release"]; exists {
					return release.Name == "updated-release-name"
				}

				return false
			}).Should(BeTrue(), "HydratedTarget was not updated with new release name")
		})
	})
})
