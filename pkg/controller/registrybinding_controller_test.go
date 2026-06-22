// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RegistryBindingReconciler", Ordered, func() {
	var (
		validRegistry = func(name string) *solarv1alpha1.Registry {
			return &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname: "registry.example.com",
				},
			}
		}

		validRegistryBinding = func(name string, registryName string) *solarv1alpha1.RegistryBinding {
			return &solarv1alpha1.RegistryBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistryBindingSpec{
					TargetRef:   corev1.LocalObjectReference{Name: "my-target"},
					RegistryRef: corev1.LocalObjectReference{Name: registryName},
				},
			}
		}
	)

	Describe("protection finalizer on Registry", func() {
		It("adds registryBindingFinalizer to RegistryBinding and registryRefFinalizer to Registry", func() {
			registry := validRegistry("dp-regb-registry-fin")
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, registry, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, registry))
			})

			rb := validRegistryBinding("dp-regb-fin", registry.Name)
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, rb, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, rb))
			})

			Eventually(func(g Gomega) {
				updatedRB := &solarv1alpha1.RegistryBinding{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rb), updatedRB)).To(Succeed())
				g.Expect(updatedRB.Finalizers).To(ContainElement(registryBindingFinalizer))

				updatedRegistry := &solarv1alpha1.Registry{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(registry), updatedRegistry)).To(Succeed())
				g.Expect(updatedRegistry.Finalizers).To(ContainElement(registryRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("blocks Registry deletion while a RegistryBinding references it", func() {
			registry := validRegistry("dp-regb-registry-blocked")
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			rb := validRegistryBinding("dp-regb-blocks-registry", registry.Name)
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())

			// Wait for protection finalizer on Registry.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Registry{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(registry), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(registryRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete Registry — it should be blocked.
			Expect(k8sClient.Delete(ctx, registry)).To(Succeed())

			Consistently(func(g Gomega) {
				updated := &solarv1alpha1.Registry{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(registry), updated)).To(Succeed())
				g.Expect(updated.DeletionTimestamp).NotTo(BeNil())
			}, consistentlyDuration).Should(Succeed())

			// Delete RegistryBinding — controller removes registryRefFinalizer from Registry,
			// then removes registryBindingFinalizer, unblocking Registry deletion.
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, rb))).To(Succeed())

			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(registry), &solarv1alpha1.Registry{}))
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("removes registryRefFinalizer from Registry when last RegistryBinding is deleted", func() {
			registry := validRegistry("dp-regb-registry-unprotect")
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, registry, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, registry))
			})

			rb := validRegistryBinding("dp-regb-last", registry.Name)
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())

			// Wait for protection finalizer.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Registry{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(registry), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(registryRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete the RegistryBinding — the controller's deletion handler will call
			// removeRegistryRefFinalizer (no other references) then remove registryBindingFinalizer.
			Expect(k8sClient.Delete(ctx, rb)).To(Succeed())

			// registryRefFinalizer should be removed from Registry.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Registry{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(registry), updated)).To(Succeed())
				g.Expect(updated.Finalizers).NotTo(ContainElement(registryRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("retains registryRefFinalizer when a second RegistryBinding still references the Registry", func() {
			registry := validRegistry("dp-reg-multi")
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, registry, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, registry))
			})

			rb1 := validRegistryBinding("dp-regb-multi-1", registry.Name)
			Expect(k8sClient.Create(ctx, rb1)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, rb1, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, rb1))
			})

			rb2 := validRegistryBinding("dp-regb-multi-2", registry.Name)
			Expect(k8sClient.Create(ctx, rb2)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, rb2, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, rb2))
			})

			// Wait for the protection finalizer to be established.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Registry{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(registry), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(registryRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete rb1 — rb2 still holds the reference, so registry-ref must stay.
			Expect(k8sClient.Delete(ctx, rb1)).To(Succeed())

			// Wait for rb1 to be fully gone from the API.
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(rb1), &solarv1alpha1.RegistryBinding{}))
			}, eventuallyTimeout).Should(BeTrue())

			// registry-ref must remain because rb2 still references the Registry.
			Consistently(func(g Gomega) {
				updated := &solarv1alpha1.Registry{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(registry), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(registryRefFinalizer))
			}, consistentlyDuration).Should(Succeed())
		})
	})
})
