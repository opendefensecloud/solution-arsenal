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

var _ = Describe("ComponentReconciler", Ordered, func() {
	var (
		newComponent = func(name string) *solarv1alpha1.Component {
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

		newCV = func(name string, componentName string) *solarv1alpha1.ComponentVersion {
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

		// cleanup force-removes finalizers so namespace teardown never hangs.
		cleanup = func(obj client.Object) {
			patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
			_ = client.IgnoreNotFound(k8sClient.Patch(ctx, obj, patch))
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, obj))
		}
	)

	It("adds componentRefFinalizer to the Component while a live ComponentVersion references it", func() {
		comp := newComponent("cr-comp-protect")
		Expect(k8sClient.Create(ctx, comp)).To(Succeed())
		DeferCleanup(cleanup, comp)

		cv := newCV("cr-cv-protect", comp.Name)
		Expect(k8sClient.Create(ctx, cv)).To(Succeed())
		DeferCleanup(cleanup, cv)

		Eventually(func(g Gomega) {
			updated := &solarv1alpha1.Component{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
			g.Expect(updated.Finalizers).To(ContainElement(componentRefFinalizer))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("blocks Component deletion while a live ComponentVersion exists, completes it once the last CV is gone", func() {
		comp := newComponent("cr-comp-blocked")
		Expect(k8sClient.Create(ctx, comp)).To(Succeed())
		DeferCleanup(cleanup, comp)

		cv := newCV("cr-cv-blocked", comp.Name)
		Expect(k8sClient.Create(ctx, cv)).To(Succeed())
		DeferCleanup(cleanup, cv)

		Eventually(func(g Gomega) {
			updated := &solarv1alpha1.Component{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
			g.Expect(updated.Finalizers).To(ContainElement(componentRefFinalizer))
		}, eventuallyTimeout).Should(Succeed())

		Expect(k8sClient.Delete(ctx, comp)).To(Succeed())

		Consistently(func(g Gomega) {
			updated := &solarv1alpha1.Component{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
			g.Expect(updated.DeletionTimestamp).NotTo(BeNil())
		}, consistentlyDuration).Should(Succeed())

		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, cv))).To(Succeed())

		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), &solarv1alpha1.Component{}))
		}, eventuallyTimeout).Should(BeTrue())
	})

	It("garbage collects the Component when its last ComponentVersion is deleted", func() {
		comp := newComponent("cr-comp-gc")
		Expect(k8sClient.Create(ctx, comp)).To(Succeed())
		DeferCleanup(cleanup, comp)

		cv := newCV("cr-cv-gc", comp.Name)
		Expect(k8sClient.Create(ctx, cv)).To(Succeed())

		Eventually(func(g Gomega) {
			updated := &solarv1alpha1.Component{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
			g.Expect(updated.Finalizers).To(ContainElement(componentRefFinalizer))
		}, eventuallyTimeout).Should(Succeed())

		Expect(k8sClient.Delete(ctx, cv)).To(Succeed())

		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), &solarv1alpha1.Component{}))
		}, eventuallyTimeout).Should(BeTrue())
	})

	It("does not GC the Component when a CV is deleted while another is created concurrently", func() {
		comp := newComponent("cr-comp-recreate")
		Expect(k8sClient.Create(ctx, comp)).To(Succeed())
		DeferCleanup(cleanup, comp)

		cvOld := newCV("cr-cv-old", comp.Name)
		Expect(k8sClient.Create(ctx, cvOld)).To(Succeed())

		Eventually(func(g Gomega) {
			updated := &solarv1alpha1.Component{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
			g.Expect(updated.Finalizers).To(ContainElement(componentRefFinalizer))
		}, eventuallyTimeout).Should(Succeed())

		// Delete the old CV and immediately create a replacement, simulating
		// discovery re-creating a version during cleanup.
		cvNew := newCV("cr-cv-new", comp.Name)
		Expect(k8sClient.Delete(ctx, cvOld)).To(Succeed())
		Expect(k8sClient.Create(ctx, cvNew)).To(Succeed())
		DeferCleanup(cleanup, cvNew)

		// The Component must survive the churn and keep (or regain) protection.
		Consistently(func(g Gomega) {
			updated := &solarv1alpha1.Component{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
			g.Expect(updated.DeletionTimestamp).To(BeNil())
		}, consistentlyDuration).Should(Succeed())

		Eventually(func(g Gomega) {
			updated := &solarv1alpha1.Component{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
			g.Expect(updated.Finalizers).To(ContainElement(componentRefFinalizer))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("sweeps a Component that already carries componentRefFinalizer but has no live ComponentVersions", func() {
		// Seeds the state left behind by a crash between the delete call and
		// the finalizer strip (or an upgrade-time orphan predating GC): the
		// finalizer is present, no ComponentVersion ever references it. The
		// finalizer-adding patch itself is the update event that gets this
		// Component onto the reconciler's queue.
		comp := newComponent("cr-comp-orphan")
		Expect(k8sClient.Create(ctx, comp)).To(Succeed())
		DeferCleanup(cleanup, comp)

		original := comp.DeepCopy()
		comp.Finalizers = append(comp.Finalizers, componentRefFinalizer)
		Expect(k8sClient.Patch(ctx, comp, client.MergeFrom(original))).To(Succeed())

		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), &solarv1alpha1.Component{}))
		}, eventuallyTimeout).Should(BeTrue())
	})

	It("leaves a Component without any ComponentVersions alone", func() {
		comp := newComponent("cr-comp-manual")
		Expect(k8sClient.Create(ctx, comp)).To(Succeed())
		DeferCleanup(cleanup, comp)

		// No CV ever referenced it: no finalizer, no GC.
		Consistently(func(g Gomega) {
			updated := &solarv1alpha1.Component{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), updated)).To(Succeed())
			g.Expect(updated.Finalizers).NotTo(ContainElement(componentRefFinalizer))
		}, consistentlyDuration).Should(Succeed())
	})
})
