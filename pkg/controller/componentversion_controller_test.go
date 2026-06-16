// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ComponentVersionReconciler", Ordered, func() {
	var (
		validComponent = func(name string) *solarv1alpha1.Component {
			return &solarv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ComponentSpec{
					Scheme:     "oci",
					Registry:   "registry.example.com",
					Repository: "example/component",
				},
			}
		}

		validCV = func(name string, componentName string) *solarv1alpha1.ComponentVersion {
			return &solarv1alpha1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ComponentVersionSpec{
					ComponentRef: corev1.LocalObjectReference{Name: componentName},
					Tag:          "v1.0.0",
				},
			}
		}
	)

	Describe("protection finalizer on Component", func() {
		It("adds componentVersionFinalizer to ComponentVersion and componentRefFinalizer to Component", func() {
			comp := validComponent("dp-comp-a")
			Expect(k8sClient.Create(ctx, comp)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, comp, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, comp))
			})

			cv := validCV("dp-cv-a", comp.Name)
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, cv, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, cv))
			})

			Eventually(func(g Gomega) {
				updatedCV := &solarv1alpha1.ComponentVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), updatedCV)).To(Succeed())
				g.Expect(updatedCV.Finalizers).To(ContainElement(componentVersionFinalizer))

				updatedComp := &solarv1alpha1.Component{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updatedComp)).To(Succeed())
				g.Expect(updatedComp.Finalizers).To(ContainElement(componentRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("blocks Component deletion while ComponentVersion references it", func() {
			comp := validComponent("dp-comp-blocked")
			Expect(k8sClient.Create(ctx, comp)).To(Succeed())

			cv := validCV("dp-cv-blocked", comp.Name)
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			// Wait for the protection finalizer to be added.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Component{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(componentRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete Component — it should be blocked (DeletionTimestamp set, not gone).
			Expect(k8sClient.Delete(ctx, comp)).To(Succeed())

			Consistently(func(g Gomega) {
				updated := &solarv1alpha1.Component{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
				g.Expect(updated.DeletionTimestamp).NotTo(BeNil())
			}, consistentlyDuration).Should(Succeed())

			// Delete the ComponentVersion — controller removes componentRefFinalizer from Component,
			// then removes componentVersionFinalizer, unblocking Component deletion.
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, cv))).To(Succeed())

			Eventually(func() error {
				return client.IgnoreNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), &solarv1alpha1.Component{}))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("removes componentRefFinalizer from Component when last ComponentVersion is deleted", func() {
			comp := validComponent("dp-comp-last")
			Expect(k8sClient.Create(ctx, comp)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, comp, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, comp))
			})

			cv := validCV("dp-cv-last", comp.Name)
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			// Wait for protection finalizer.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Component{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(componentRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete the ComponentVersion — the controller's deletion handler removes componentRefFinalizer
			// from Component (no other references), then removes componentVersionFinalizer.
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, cv))).To(Succeed())

			// The componentRefFinalizer should eventually be removed from Component.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Component{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
				g.Expect(updated.Finalizers).NotTo(ContainElement(componentRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("retains componentRefFinalizer when a second ComponentVersion still references the Component", func() {
			comp := validComponent("dp-comp-multi")
			Expect(k8sClient.Create(ctx, comp)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, comp, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, comp))
			})

			cv1 := validCV("dp-cv-multi-1", comp.Name)
			Expect(k8sClient.Create(ctx, cv1)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, cv1, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, cv1))
			})

			cv2 := validCV("dp-cv-multi-2", comp.Name)
			Expect(k8sClient.Create(ctx, cv2)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, cv2, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, cv2))
			})

			// Wait for the protection finalizer to be established.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Component{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(componentRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete cv1 — cv2 still holds the reference, so component-ref must stay.
			Expect(k8sClient.Delete(ctx, cv1)).To(Succeed())

			// Wait for cv1 to be fully gone from the API.
			Eventually(func() error {
				return client.IgnoreNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv1), &solarv1alpha1.ComponentVersion{}))
			}, eventuallyTimeout).Should(Succeed())

			// component-ref must remain because cv2 still references the Component.
			Consistently(func(g Gomega) {
				updated := &solarv1alpha1.Component{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(componentRefFinalizer))
			}, consistentlyDuration).Should(Succeed())
		})
	})
})
