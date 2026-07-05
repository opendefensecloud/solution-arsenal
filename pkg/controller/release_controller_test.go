// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package controller

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

	Describe("Priority", func() {
		It("should default Priority to 0", func() {
			release := validRelease("test-release-default-priority", ns)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			created := &solarv1alpha1.Release{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), created)).To(Succeed())
			Expect(created.Spec.Priority).To(Equal(int32(0)))
		})

		It("should persist a non-zero Priority value", func() {
			release := validRelease("test-release-with-priority", ns)
			release.Spec.Priority = 10
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			created := &solarv1alpha1.Release{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), created)).To(Succeed())
			Expect(created.Spec.Priority).To(Equal(int32(10)))
		})
	})

	Describe("UniqueName", func() {
		It("should allow creating a Release without a UniqueName", func() {
			release := validRelease("test-release-no-unique-name", ns)
			release.Spec.UniqueName = ""
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
		})

		It("should set status.effectiveUniqueName to the Component name when UniqueName is empty", func() {
			cv := validComponentVersion("eu-fallback-cv", ns)
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			release := validRelease("test-release-effective-unique-name-fallback", ns)
			release.Spec.ComponentVersionRef.Name = cv.Name
			release.Spec.UniqueName = ""
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			updated := &solarv1alpha1.Release{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Status.EffectiveUniqueName).To(Equal(cv.Spec.ComponentRef.Name))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("should set status.effectiveUniqueName to Spec.UniqueName when explicitly provided", func() {
			cv := validComponentVersion("eu-explicit-cv", ns)
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			release := validRelease("test-release-effective-unique-name-explicit", ns)
			release.Spec.ComponentVersionRef.Name = cv.Name
			release.Spec.UniqueName = "custom-unique-name"
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			updated := &solarv1alpha1.Release{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Status.EffectiveUniqueName).To(Equal("custom-unique-name"))
			}, eventuallyTimeout).Should(Succeed())
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
			DeferCleanup(func() { _ = client.IgnoreNotFound(k8sClient.Delete(ctx, release)) })

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
			DeferCleanup(func() { _ = client.IgnoreNotFound(k8sClient.Delete(ctx, release)) })

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
			DeferCleanup(func() { _ = client.IgnoreNotFound(k8sClient.Delete(ctx, release)) })

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

	Describe("deletion protection for ComponentVersion", func() {
		It("adds releaseFinalizer to Release and componentVersionRefFinalizer to ComponentVersion", func() {
			cv := validComponentVersion("dp-cv-fin", ns)
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, cv, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, cv))
			})

			release := validRelease("dp-release-fin", ns)
			release.Spec.ComponentVersionRef.Name = cv.Name
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, release, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, release))
			})

			Eventually(func(g Gomega) {
				updatedRelease := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updatedRelease)).To(Succeed())
				g.Expect(updatedRelease.Finalizers).To(ContainElement(releaseFinalizer))

				updatedCV := &solarv1alpha1.ComponentVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), updatedCV)).To(Succeed())
				g.Expect(updatedCV.Finalizers).To(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("blocks ComponentVersion deletion while a Release references it", func() {
			cv := validComponentVersion("dp-cv-blocked", ns)
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			release := validRelease("dp-release-blocks-cv", ns)
			release.Spec.ComponentVersionRef.Name = cv.Name
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for protection finalizer on CV.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.ComponentVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Try to delete the ComponentVersion — it should be blocked.
			Expect(k8sClient.Delete(ctx, cv)).To(Succeed())

			Consistently(func(g Gomega) {
				updated := &solarv1alpha1.ComponentVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), updated)).To(Succeed())
				g.Expect(updated.DeletionTimestamp).NotTo(BeNil())
			}, consistentlyDuration).Should(Succeed())

			// Delete the Release — controller removes componentVersionRefFinalizer from CV,
			// then removes releaseFinalizer, unblocking CV deletion.
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, release))).To(Succeed())

			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), &solarv1alpha1.ComponentVersion{}))
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("removes componentVersionRefFinalizer when the last Release is deleted", func() {
			cv := validComponentVersion("dp-cv-unprotect", ns)
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, cv, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, cv))
			})

			release := validRelease("dp-release-last", ns)
			release.Spec.ComponentVersionRef.Name = cv.Name
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.ComponentVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete the Release — the controller's deletion handler removes componentVersionRefFinalizer
			// from CV (no other references), then removes releaseFinalizer.
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, release))).To(Succeed())

			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.ComponentVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), updated)).To(Succeed())
				g.Expect(updated.Finalizers).NotTo(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})
	})
})
