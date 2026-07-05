// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package controller

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReleaseBindingReconciler", Ordered, func() {
	var (
		validRelease = func(name string) *solarv1alpha1.Release {
			return &solarv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{Name: "my-cv"},
					UniqueName:          name,
				},
			}
		}

		validReleaseBinding = func(name string, releaseName string) *solarv1alpha1.ReleaseBinding {
			return &solarv1alpha1.ReleaseBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ReleaseBindingSpec{
					TargetRef:  corev1.LocalObjectReference{Name: "my-target"},
					ReleaseRef: corev1.LocalObjectReference{Name: releaseName},
				},
			}
		}
	)

	Describe("protection finalizer on Release", func() {
		It("adds releaseBindingFinalizer to ReleaseBinding and releaseRefFinalizer to Release", func() {
			release := validRelease("dp-rb-release-fin")
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, release, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, release))
			})

			rb := validReleaseBinding("dp-rb-fin", release.Name)
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, rb, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, rb))
			})

			Eventually(func(g Gomega) {
				updatedRB := &solarv1alpha1.ReleaseBinding{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rb), updatedRB)).To(Succeed())
				g.Expect(updatedRB.Finalizers).To(ContainElement(releaseBindingFinalizer))

				updatedRelease := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updatedRelease)).To(Succeed())
				g.Expect(updatedRelease.Finalizers).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("blocks Release deletion while a ReleaseBinding references it", func() {
			release := validRelease("dp-rb-release-blocked")
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			rb := validReleaseBinding("dp-rb-blocks-release", release.Name)
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())

			// Wait for protection finalizer on Release.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete Release — it should be blocked.
			Expect(k8sClient.Delete(ctx, release)).To(Succeed())

			Consistently(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.DeletionTimestamp).NotTo(BeNil())
			}, consistentlyDuration).Should(Succeed())

			// Delete ReleaseBinding — controller removes releaseRefFinalizer from Release,
			// then removes releaseBindingFinalizer, unblocking Release deletion.
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, rb))).To(Succeed())

			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), &solarv1alpha1.Release{}))
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("removes releaseRefFinalizer from Release when last ReleaseBinding is deleted", func() {
			release := validRelease("dp-rb-release-unprotect")
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, release, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, release))
			})

			rb := validReleaseBinding("dp-rb-last", release.Name)
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())

			// Wait for protection finalizer.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete the ReleaseBinding — the controller's deletion handler removes releaseRefFinalizer
			// from Release (no other references), then removes releaseBindingFinalizer.
			Expect(k8sClient.Delete(ctx, rb)).To(Succeed())

			// releaseRefFinalizer should be removed from Release.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).NotTo(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("retains releaseRefFinalizer when a second ReleaseBinding still references the Release", func() {
			release := validRelease("dp-rb-multi-release")
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, release, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, release))
			})

			rb1 := validReleaseBinding("dp-rb-multi-1", release.Name)
			Expect(k8sClient.Create(ctx, rb1)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, rb1, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, rb1))
			})

			rb2 := validReleaseBinding("dp-rb-multi-2", release.Name)
			Expect(k8sClient.Create(ctx, rb2)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, rb2, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, rb2))
			})

			// Wait for the protection finalizer to be established.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete rb1 — rb2 still holds the reference, so release-ref must stay.
			Expect(k8sClient.Delete(ctx, rb1)).To(Succeed())

			// Wait for rb1 to be fully gone from the API.
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(rb1), &solarv1alpha1.ReleaseBinding{}))
			}, eventuallyTimeout).Should(BeTrue())

			// release-ref must remain because rb2 still references the Release.
			Consistently(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(releaseRefFinalizer))
			}, consistentlyDuration).Should(Succeed())
		})
	})

	Describe("Profile-owned binding guard", func() {
		It("skips removeReleaseRefFinalizer while owner Profile still holds profileFinalizer", func() {
			release := &solarv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{Name: "dp-rb-guard-release", Namespace: ns.Name},
				Spec: solarv1alpha1.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{Name: "my-cv"},
					UniqueName:          "dp-rb-guard-release",
				},
			}
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, release, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, release))
			})

			target := &solarv1alpha1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dp-rb-guard-target",
					Namespace: ns.Name,
					Labels:    map[string]string{"env": "dp-rb-guard"},
				},
				Spec: solarv1alpha1.TargetSpec{},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())
			DeferCleanup(func() { _ = client.IgnoreNotFound(k8sClient.Delete(ctx, target)) })

			profile := &solarv1alpha1.Profile{
				ObjectMeta: metav1.ObjectMeta{Name: "dp-rb-guard-profile", Namespace: ns.Name},
				Spec: solarv1alpha1.ProfileSpec{
					TargetSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "dp-rb-guard"},
					},
					ReleaseRef: corev1.LocalObjectReference{Name: release.Name},
				},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, profile, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, profile))
			})

			// Wait for the Profile controller to create the owned ReleaseBinding and for
			// release-ref to appear on the Release.
			var bindingKey types.NamespacedName
			Eventually(func(g Gomega) {
				list := &solarv1alpha1.ReleaseBindingList{}
				g.Expect(k8sClient.List(ctx, list, client.InNamespace(ns.Name))).To(Succeed())
				var owned []solarv1alpha1.ReleaseBinding
				for _, rb := range list.Items {
					for _, ref := range rb.OwnerReferences {
						if ref.Name == profile.Name && ref.Kind == "Profile" {
							owned = append(owned, rb)
						}
					}
				}
				g.Expect(owned).To(HaveLen(1))
				bindingKey = client.ObjectKeyFromObject(&owned[0])

				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Add a test-only blocker finalizer to hold the binding in Terminating state so we can
			// assert invariants during the window when Profile still holds profileFinalizer.
			const testBlocker = "test.solar/blocker"
			binding := &solarv1alpha1.ReleaseBinding{}
			Expect(k8sClient.Get(ctx, bindingKey, binding)).To(Succeed())
			withBlocker := binding.DeepCopy()
			withBlocker.Finalizers = append(withBlocker.Finalizers, testBlocker)
			Expect(k8sClient.Patch(ctx, withBlocker, client.MergeFrom(binding))).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, withBlocker, patch))
			})

			// Delete the Profile — the Profile controller will delete the owned binding, which
			// enters Terminating state (held by testBlocker).
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, profile))).To(Succeed())
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Profile{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(profile), updated)).To(Succeed())
				g.Expect(updated.DeletionTimestamp).NotTo(BeNil())
			}, eventuallyTimeout).Should(Succeed())

			// While the binding is in Terminating (testBlocker present), the ReleaseBinding
			// controller must NOT remove release-ref — the guard defers to the Profile controller.
			Consistently(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(releaseRefFinalizer),
					"release-ref must not be removed while Profile controller still owns cleanup")
			}, consistentlyDuration).Should(Succeed())

			// Release the blocker — binding completes deletion, Profile controller removes
			// release-ref from Release, then removes profileFinalizer from Profile.
			bindingDeleting := &solarv1alpha1.ReleaseBinding{}
			Expect(k8sClient.Get(ctx, bindingKey, bindingDeleting)).To(Succeed())
			withoutBlocker := bindingDeleting.DeepCopy()
			withoutBlocker.Finalizers = slices.DeleteFunc(withoutBlocker.Finalizers,
				func(s string) bool { return s == testBlocker })
			Expect(k8sClient.Patch(ctx, withoutBlocker, client.MergeFrom(bindingDeleting))).To(Succeed())

			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(profile), &solarv1alpha1.Profile{}))
			}, eventuallyTimeout).Should(BeTrue())
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).NotTo(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})
	})
})
