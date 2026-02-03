// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"go.opendefense.cloud/kit/envtest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Component", func() {
	var (
		ctx  = envtest.Context()
		ns   = SetupTest(ctx)
		comp = &solarv1alpha1.Component{}
	)

	Context("Component", func() {
		It("should allow creating a component", func() {
			By("creating a test component")
			comp = &solarv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    ns.Name,
					GenerateName: "test-",
				},
				Spec: solarv1alpha1.ComponentSpec{},
			}
			Expect(k8sClient.Create(ctx, comp)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), comp)).To(Succeed())
		})
		It("should allow deleting a component", func() {
			By("deleting a test component")
			Expect(k8sClient.Delete(ctx, comp)).To(Succeed())
		})
	})

})

var _ = Describe("ComponentVersion", func() {
	var (
		ctx     = envtest.Context()
		ns      = SetupTest(ctx)
		compver = &solarv1alpha1.ComponentVersion{}
	)

	Context("ComponentVersion", func() {
		It("should allow creating a component version", func() {
			By("creating a test component version")
			compver = &solarv1alpha1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    ns.Name,
					GenerateName: "test-",
				},
				Spec: solarv1alpha1.ComponentVersionSpec{},
			}
			Expect(k8sClient.Create(ctx, compver)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(compver), compver)).To(Succeed())
		})
		It("should allow deleting a component version", func() {
			By("deleting a test component version")
			Expect(k8sClient.Delete(ctx, compver)).To(Succeed())
		})
	})

})

var _ = Describe("Release", func() {
	var (
		ctx = envtest.Context()
		ns  = SetupTest(ctx)
		rel = &solarv1alpha1.Release{}
	)

	Context("Release", func() {
		It("should allow creating a release", func() {
			By("creating a test release")
			rel = &solarv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    ns.Name,
					GenerateName: "test-",
				},
				Spec: solarv1alpha1.ReleaseSpec{},
			}
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rel), rel)).To(Succeed())
		})
		It("should allow deleting a release", func() {
			By("deleting a test release")
			Expect(k8sClient.Delete(ctx, rel)).To(Succeed())
		})
	})

})

var _ = Describe("Target", func() {
	var (
		ctx    = envtest.Context()
		ns     = SetupTest(ctx)
		target = &solarv1alpha1.Target{}
	)

	Context("Target", func() {
		It("should allow creating a target", func() {
			By("creating a test target")
			target = &solarv1alpha1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    ns.Name,
					GenerateName: "test-",
				},
				Spec: solarv1alpha1.TargetSpec{},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(target), target)).To(Succeed())
		})
		It("should allow deleting a target", func() {
			By("deleting a test target")
			Expect(k8sClient.Delete(ctx, target)).To(Succeed())
		})
	})

})

var _ = Describe("HydratedTarget", func() {
	var (
		ctx      = envtest.Context()
		ns       = SetupTest(ctx)
		hydrated = &solarv1alpha1.HydratedTarget{}
	)

	Context("HydratedTarget", func() {
		It("should allow creating a hydrated target", func() {
			By("creating a test hydrated target")
			hydrated = &solarv1alpha1.HydratedTarget{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    ns.Name,
					GenerateName: "test-",
				},
				Spec: solarv1alpha1.HydratedTargetSpec{},
			}
			Expect(k8sClient.Create(ctx, hydrated)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(hydrated), hydrated)).To(Succeed())
		})
		It("should allow deleting a hydrated target", func() {
			By("deleting a test hydrated target")
			Expect(k8sClient.Delete(ctx, hydrated)).To(Succeed())
		})
	})

})
