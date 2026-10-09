// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"go.opendefense.cloud/kit/envtest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
				Namespace:    ns.Name,
				GenerateName: "test-",
				Spec:         solarv1alpha1.ComponentVersionSpec{},
			}
			Expect(k8sClient.Create(ctx, compver)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(compver), compver)).To(Succeed())
		})
		It("should allow deleting a component version", func() {
			By("deleting a test component version")
			Expect(k8sClient.Delete(ctx, compver)).To(Succeed())
		})
	})

	Context("field selectors", func() {
		It("filters ComponentVersions by componentName and tag", func() {
			for _, c := range []struct{ name, comp, tag string }{
				{"arc-v1", "opendefense.cloud/arc", "v1"},
				{"arc-v2", "opendefense.cloud/arc", "v2"},
				{"other-v1", "opendefense.cloud/other", "v1"},
			} {
				Expect(k8sClient.Create(ctx, &solarv1alpha1.ComponentVersion{
					Name: c.name, Namespace: ns.Name,
					Spec: solarv1alpha1.ComponentVersionSpec{ComponentName: c.comp, Tag: c.tag},
				})).To(Succeed())
			}

			list := &solarv1alpha1.ComponentVersionList{}
			Expect(k8sClient.List(ctx, list, client.InNamespace(ns.Name),
				client.MatchingFieldsSelector{Selector: fields.OneTermEqualSelector("spec.componentName", "opendefense.cloud/arc")},
			)).To(Succeed())
			Expect(list.Items).To(HaveLen(2))

			Expect(k8sClient.List(ctx, list, client.InNamespace(ns.Name),
				client.MatchingFields{"spec.componentName": "opendefense.cloud/arc", "spec.tag": "v2"},
			)).To(Succeed())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].Name).To(Equal("arc-v2"))
		})

		It("filters ReleaseBindings by releaseRef.name", func() {
			for _, c := range []struct{ name, rel string }{{"b1", "r1"}, {"b2", "r2"}} {
				Expect(k8sClient.Create(ctx, &solarv1alpha1.ReleaseBinding{
					Name: c.name, Namespace: ns.Name,
					Spec: solarv1alpha1.ReleaseBindingSpec{
						TargetRef:  solarv1alpha1.ObjectReference{Name: "t1"},
						ReleaseRef: corev1.LocalObjectReference{Name: c.rel},
					},
				})).To(Succeed())
			}

			list := &solarv1alpha1.ReleaseBindingList{}
			Expect(k8sClient.List(ctx, list, client.InNamespace(ns.Name),
				client.MatchingFields{"spec.releaseRef.name": "r1"})).To(Succeed())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].Name).To(Equal("b1"))
		})

		It("rejects an unknown selector key with 400", func() {
			err := k8sClient.List(ctx, &solarv1alpha1.ComponentVersionList{}, client.InNamespace(ns.Name),
				client.MatchingFields{"spec.bogus": "x"})
			Expect(apierrors.IsBadRequest(err)).To(BeTrue(), "got %v", err)
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
				Namespace:    ns.Name,
				GenerateName: "test-",
				Spec: solarv1alpha1.ReleaseSpec{
					ComponentVersionRef: solarv1alpha1.ObjectReference{Name: "my-component-v1"},
					UniqueName:          "my-component",
				},
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
				Namespace:    ns.Name,
				GenerateName: "test-",
				Spec:         solarv1alpha1.TargetSpec{},
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

var _ = Describe("Registry", func() {
	var (
		ctx = envtest.Context()
		ns  = SetupTest(ctx)
		reg = &solarv1alpha1.Registry{}
	)

	Context("Registry", func() {
		It("should allow creating a registry", func() {
			By("creating a test registry")
			reg = &solarv1alpha1.Registry{
				Namespace:    ns.Name,
				GenerateName: "test-",
				Spec: solarv1alpha1.RegistrySpec{
					Hostname: "registry.example.com",
				},
			}
			Expect(k8sClient.Create(ctx, reg)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(reg), reg)).To(Succeed())
		})
		It("should allow deleting a registry", func() {
			By("deleting a test registry")
			Expect(k8sClient.Delete(ctx, reg)).To(Succeed())
		})
	})
})

var _ = Describe("RegistryBinding", func() {
	var (
		ctx = envtest.Context()
		ns  = SetupTest(ctx)
		rb  = &solarv1alpha1.RegistryBinding{}
	)

	Context("RegistryBinding", func() {
		It("should allow creating a registry binding", func() {
			By("creating a test registry binding")
			rb = &solarv1alpha1.RegistryBinding{
				Namespace:    ns.Name,
				GenerateName: "test-",
				Spec:         solarv1alpha1.RegistryBindingSpec{},
			}
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rb), rb)).To(Succeed())
		})
		It("should allow deleting a registry binding", func() {
			By("deleting a test registry binding")
			Expect(k8sClient.Delete(ctx, rb)).To(Succeed())
		})
	})
})

var _ = Describe("ReleaseBinding", func() {
	var (
		ctx = envtest.Context()
		ns  = SetupTest(ctx)
		rlb = &solarv1alpha1.ReleaseBinding{}
	)

	Context("ReleaseBinding", func() {
		It("should allow creating a release binding", func() {
			By("creating a test release binding")
			rlb = &solarv1alpha1.ReleaseBinding{
				Namespace:    ns.Name,
				GenerateName: "test-",
				Spec:         solarv1alpha1.ReleaseBindingSpec{},
			}
			Expect(k8sClient.Create(ctx, rlb)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rlb), rlb)).To(Succeed())
		})
		It("should allow deleting a release binding", func() {
			By("deleting a test release binding")
			Expect(k8sClient.Delete(ctx, rlb)).To(Succeed())
		})
	})
})

var _ = Describe("Profile", func() {
	var (
		ctx     = envtest.Context()
		ns      = SetupTest(ctx)
		profile = &solarv1alpha1.Profile{}
	)

	Context("Profile", func() {
		It("should allow creating a profile", func() {
			By("creating a test profile")
			profile = &solarv1alpha1.Profile{
				Namespace:    ns.Name,
				GenerateName: "test-",
				Spec:         solarv1alpha1.ProfileSpec{},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(profile), profile)).To(Succeed())
		})
		It("should allow deleting a profile", func() {
			By("deleting a test profile")
			Expect(k8sClient.Delete(ctx, profile)).To(Succeed())
		})
	})
})
