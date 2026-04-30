// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
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
					UniqueName: "my-component",
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

	Describe("UniqueName validation", func() {
		It("should reject a Release with an empty UniqueName", func() {
			release := validRelease("test-release-empty-unique-name", ns)
			release.Spec.UniqueName = ""
			Expect(k8sClient.Create(ctx, release)).NotTo(Succeed())
		})
	})

	Describe("ComponentVersion resolution", func() {
		It("should set ComponentVersionResolved=True when ComponentVersion exists", func() {
			cv := validComponentVersion("my-component-v1", ns)
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			release := validRelease("test-release-resolved", ns)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

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

	Describe("cross-namespace ComponentVersion resolution via ReferenceGrant", func() {
		var (
			catalogNs *corev1.Namespace
			catalogCV *solarv1alpha1.ComponentVersion
		)

		BeforeEach(func() {
			catalogNs = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "catalog-"},
			}
			Expect(k8sClient.Create(ctx, catalogNs)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, catalogNs)

			catalogCV = validComponentVersion("shared-cv", catalogNs)
			Expect(k8sClient.Create(ctx, catalogCV)).To(Succeed())
		})

		It("should set ComponentVersionResolved=True when a ReferenceGrant permits access", func() {
			grant := &solarv1alpha1.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cv-grant-",
					Namespace:    catalogNs.Name,
				},
				Spec: solarv1alpha1.ReferenceGrantSpec{
					From: []solarv1alpha1.ReferenceGrantFromSubject{
						{Group: solarGroup, Kind: "Release", Namespace: ns.Name},
					},
					To: []solarv1alpha1.ReferenceGrantToTarget{
						{Group: solarGroup, Kind: "ComponentVersion"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, grant)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, grant)

			release := validRelease("test-cross-ns-cv-granted", ns)
			release.Spec.ComponentVersionRef.Name = "shared-cv"
			release.Spec.ComponentVersionNamespace = catalogNs.Name
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, release)

			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updatedRelease)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(updatedRelease.Status.Conditions, ConditionTypeComponentVersionResolved)).To(BeTrue())
			}, eventuallyTimeout).Should(Succeed())
		})

		It("should set ComponentVersionResolved=False reason NotGranted when no ReferenceGrant exists", func() {
			release := validRelease("test-cross-ns-cv-no-grant", ns)
			release.Spec.ComponentVersionRef.Name = "shared-cv"
			release.Spec.ComponentVersionNamespace = catalogNs.Name
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, release)

			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updatedRelease)).To(Succeed())
				cond := apimeta.FindStatusCondition(updatedRelease.Status.Conditions, ConditionTypeComponentVersionResolved)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("NotGranted"))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("should revert to ComponentVersionResolved=False after the ReferenceGrant is deleted", func() {
			grant := &solarv1alpha1.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cv-grant-revoke-",
					Namespace:    catalogNs.Name,
				},
				Spec: solarv1alpha1.ReferenceGrantSpec{
					From: []solarv1alpha1.ReferenceGrantFromSubject{
						{Group: solarGroup, Kind: "Release", Namespace: ns.Name},
					},
					To: []solarv1alpha1.ReferenceGrantToTarget{
						{Group: solarGroup, Kind: "ComponentVersion"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, grant)).To(Succeed())

			release := validRelease("test-cross-ns-cv-revoke", ns)
			release.Spec.ComponentVersionRef.Name = "shared-cv"
			release.Spec.ComponentVersionNamespace = catalogNs.Name
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, release)

			// Wait for Resolved=True
			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updatedRelease)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(updatedRelease.Status.Conditions, ConditionTypeComponentVersionResolved)).To(BeTrue())
			}, eventuallyTimeout).Should(Succeed())

			// Delete the grant
			Expect(k8sClient.Delete(ctx, grant)).To(Succeed())

			// Condition should flip to NotGranted
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updatedRelease)).To(Succeed())
				cond := apimeta.FindStatusCondition(updatedRelease.Status.Conditions, ConditionTypeComponentVersionResolved)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("NotGranted"))
			}, eventuallyTimeout).Should(Succeed())
		})
	})
})
