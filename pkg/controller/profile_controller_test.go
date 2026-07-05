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

var _ = Describe("ProfileReconciler", Ordered, func() {
	var (
		newProfile = func(name string, matchLabels map[string]string) *solarv1alpha1.Profile {
			return &solarv1alpha1.Profile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ProfileSpec{
					TargetSelector: metav1.LabelSelector{
						MatchLabels: matchLabels,
					},
					ReleaseRef: corev1.LocalObjectReference{Name: "test-release"},
				},
			}
		}

		newTarget = func(name string, labels map[string]string) *solarv1alpha1.Target {
			return &solarv1alpha1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
					Labels:    labels,
				},
				Spec: solarv1alpha1.TargetSpec{},
			}
		}

		listOwnedBindings = func(profileName string) []solarv1alpha1.ReleaseBinding {
			allBindings := &solarv1alpha1.ReleaseBindingList{}
			ExpectWithOffset(1, k8sClient.List(ctx, allBindings, client.InNamespace(ns.Name))).To(Succeed())

			var owned []solarv1alpha1.ReleaseBinding
			for _, rb := range allBindings.Items {
				for _, ref := range rb.OwnerReferences {
					if ref.Name == profileName && ref.Kind == "Profile" {
						owned = append(owned, rb)
					}
				}
			}

			return owned
		}
	)

	Context("when Profile matches Targets", func() {
		It("should create ReleaseBindings for matching Targets", func() {
			target1 := newTarget("target-env-prod", map[string]string{"env": "prod"})
			Expect(k8sClient.Create(ctx, target1)).To(Succeed())
			target2 := newTarget("target-env-test", map[string]string{"env": "test"})
			Expect(k8sClient.Create(ctx, target2)).To(Succeed())

			profile := newProfile("profile-prod", map[string]string{"env": "prod"})
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			// Should create ReleaseBinding for target-env-prod only
			Eventually(func() int {
				return len(listOwnedBindings("profile-prod"))
			}, eventuallyTimeout).Should(Equal(1))

			bindings := listOwnedBindings("profile-prod")
			Expect(bindings[0].Spec.TargetRef.Name).To(Equal("target-env-prod"))
			Expect(bindings[0].Spec.ReleaseRef.Name).To(Equal("test-release"))
		})

		It("should create ReleaseBindings for all Targets when selector is empty", func() {
			target1 := newTarget("target-all-1", map[string]string{"env": "prod"})
			Expect(k8sClient.Create(ctx, target1)).To(Succeed())
			target2 := newTarget("target-all-2", map[string]string{"env": "test"})
			Expect(k8sClient.Create(ctx, target2)).To(Succeed())

			profile := newProfile("profile-all", map[string]string{})
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			Eventually(func() int {
				return len(listOwnedBindings("profile-all"))
			}, eventuallyTimeout).Should(Equal(2))
		})

		It("should update MatchedTargets count in Profile status", func() {
			target := newTarget("target-status", map[string]string{"role": "web"})
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			profile := newProfile("profile-status", map[string]string{"role": "web"})
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			Eventually(func() int {
				p := &solarv1alpha1.Profile{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(profile), p); err != nil {
					return -1
				}

				return p.Status.MatchedTargets
			}, eventuallyTimeout).Should(Equal(1))
		})
	})

	Context("when Target labels change", func() {
		It("should update ReleaseBindings when a Target no longer matches", func() {
			target := newTarget("target-label-change", map[string]string{"env": "prod"})
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			profile := newProfile("profile-label-change", map[string]string{"env": "prod"})
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			// Wait for binding to be created
			Eventually(func() int {
				return len(listOwnedBindings("profile-label-change"))
			}, eventuallyTimeout).Should(Equal(1))

			// Update target labels so it no longer matches
			Eventually(func() error {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return err
				}
				t.Labels = map[string]string{"env": "staging"}

				return k8sClient.Update(ctx, t)
			}).Should(Succeed())

			// ReleaseBinding should be deleted
			Eventually(func() int {
				return len(listOwnedBindings("profile-label-change"))
			}, eventuallyTimeout).Should(Equal(0))
		})
	})

	Context("when Profile is deleted", func() {
		It("should have owner references on ReleaseBindings for garbage collection", func() {
			target := newTarget("target-gc", map[string]string{"tier": "frontend"})
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			profile := newProfile("profile-gc", map[string]string{"tier": "frontend"})
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			// Wait for binding to be created
			Eventually(func() int {
				return len(listOwnedBindings("profile-gc"))
			}, eventuallyTimeout).Should(Equal(1))

			// Verify the owner reference is set correctly for GC
			bindings := listOwnedBindings("profile-gc")
			Expect(bindings[0].OwnerReferences).To(HaveLen(1))
			Expect(bindings[0].OwnerReferences[0].Name).To(Equal("profile-gc"))
			Expect(bindings[0].OwnerReferences[0].Kind).To(Equal("Profile"))
			Expect(*bindings[0].OwnerReferences[0].Controller).To(BeTrue())
		})
	})

	Describe("deletion protection for Release", func() {
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
		)

		It("adds profileFinalizer to Profile and releaseRefFinalizer to Release", func() {
			release := validRelease("dp-prof-release-fin")
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, release, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, release))
			})

			profile := newProfile("dp-profile-fin", map[string]string{"env": "dp-test"})
			profile.Spec.ReleaseRef.Name = release.Name
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, profile, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, profile))
			})

			Eventually(func(g Gomega) {
				updatedProfile := &solarv1alpha1.Profile{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(profile), updatedProfile)).To(Succeed())
				g.Expect(updatedProfile.Finalizers).To(ContainElement(profileFinalizer))

				updatedRelease := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updatedRelease)).To(Succeed())
				g.Expect(updatedRelease.Finalizers).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("blocks Release deletion while a Profile references it", func() {
			release := validRelease("dp-prof-release-blocked")
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			profile := newProfile("dp-profile-blocks-release", map[string]string{"env": "dp-blocked"})
			profile.Spec.ReleaseRef.Name = release.Name
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

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

			// Delete Profile — controller removes releaseRefFinalizer from Release,
			// then removes profileFinalizer, unblocking Release deletion.
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, profile))).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(release), &solarv1alpha1.Release{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("removes releaseRefFinalizer from Release when the last Profile is deleted", func() {
			release := validRelease("dp-prof-release-unprotect")
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, release, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, release))
			})

			profile := newProfile("dp-profile-last", map[string]string{"env": "dp-last"})
			profile.Spec.ReleaseRef.Name = release.Name
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			// Wait for protection finalizer.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Delete the Profile — the controller's deletion handler removes releaseRefFinalizer
			// from Release (no other references), then removes profileFinalizer.
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, profile))).To(Succeed())

			// releaseRefFinalizer should be removed from Release.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).NotTo(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("blocks Profile deletion until owned ReleaseBinding is fully removed from API", func() {
			// The Profile controller must keep profile-finalizer (stay blocked) until every owned
			// ReleaseBinding is completely gone from the API. While any binding still exists, the
			// Release must also remain protected. We use a test-only blocker finalizer on the binding
			// to hold it in the deleting state and assert both invariants for the full Consistently
			// window before releasing control.
			release := validRelease("dp-prof-gc-window-release")
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, release, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, release))
			})

			target := newTarget("dp-prof-gc-window-target", map[string]string{"env": "dp-gc-window"})
			Expect(k8sClient.Create(ctx, target)).To(Succeed())
			DeferCleanup(func() { _ = client.IgnoreNotFound(k8sClient.Delete(ctx, target)) })

			profile := newProfile("dp-prof-gc-window", map[string]string{"env": "dp-gc-window"})
			profile.Spec.ReleaseRef.Name = release.Name
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			// Wait for the owned ReleaseBinding and release-ref to be established.
			var bindingKey types.NamespacedName
			Eventually(func(g Gomega) {
				bindings := listOwnedBindings(profile.Name)
				g.Expect(bindings).To(HaveLen(1))
				bindingKey = client.ObjectKeyFromObject(&bindings[0])

				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Add a test-only blocker finalizer to hold the binding in deleting state.
			// This lets us assert profile and release invariants during the blocked window.
			const testBlocker = "test.solar/blocker"
			binding := &solarv1alpha1.ReleaseBinding{}
			Expect(k8sClient.Get(ctx, bindingKey, binding)).To(Succeed())
			blockerAdded := binding.DeepCopy()
			blockerAdded.Finalizers = append(blockerAdded.Finalizers, testBlocker)
			Expect(k8sClient.Patch(ctx, blockerAdded, client.MergeFrom(binding))).To(Succeed())
			DeferCleanup(func() {
				wipeFinalizers := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, blockerAdded, wipeFinalizers))
			})

			// Delete the Profile.
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, profile))).To(Succeed())

			// Wait for Profile to enter deletion (DeletionTimestamp set).
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Profile{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(profile), updated)).To(Succeed())
				g.Expect(updated.DeletionTimestamp).NotTo(BeNil())
			}, eventuallyTimeout).Should(Succeed())

			// For the full Consistently window: Profile must stay blocked (profile-finalizer present)
			// and Release must remain protected (release-ref present) while binding is alive.
			Consistently(func(g Gomega) {
				updatedProfile := &solarv1alpha1.Profile{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(profile), updatedProfile)).To(Succeed())
				g.Expect(updatedProfile.Finalizers).To(ContainElement(profileFinalizer),
					"Profile must keep profile-finalizer while owned ReleaseBinding still exists")

				updatedRelease := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updatedRelease)).To(Succeed())
				g.Expect(updatedRelease.Finalizers).To(ContainElement(releaseRefFinalizer),
					"Release must keep release-ref while owned ReleaseBinding still exists")
			}, consistentlyDuration).Should(Succeed())

			// Remove the blocker — binding is now fully deleted.
			bindingDeleting := &solarv1alpha1.ReleaseBinding{}
			Expect(k8sClient.Get(ctx, bindingKey, bindingDeleting)).To(Succeed())
			bindingWithoutBlocker := bindingDeleting.DeepCopy()
			bindingWithoutBlocker.Finalizers = slices.DeleteFunc(bindingWithoutBlocker.Finalizers,
				func(s string) bool { return s == testBlocker })
			Expect(k8sClient.Patch(ctx, bindingWithoutBlocker, client.MergeFrom(bindingDeleting))).To(Succeed())

			// Profile must eventually complete deletion (fully removed from API).
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(profile), &solarv1alpha1.Profile{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue())

			// Release must eventually be unprotected.
			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.Release{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), updated)).To(Succeed())
				g.Expect(updated.Finalizers).NotTo(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})
	})

	Context("when a ReferenceGrant enables cross-namespace Target matching", func() {
		It("should create a ReleaseBinding for a Target in another namespace", func() {
			// Create a separate namespace to hold the cross-namespace target.
			otherNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "grant-src-",
				},
			}
			Expect(k8sClient.Create(ctx, otherNs)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, otherNs)

			// Target lives in the other namespace with matching label.
			crossNsTarget := &solarv1alpha1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "target-cross-ns",
					Namespace: otherNs.Name,
					Labels:    map[string]string{"env": "shared"},
				},
				Spec: solarv1alpha1.TargetSpec{},
			}
			Expect(k8sClient.Create(ctx, crossNsTarget)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, crossNsTarget)

			// ReferenceGrant in the target's namespace grants the profile's namespace
			// (ns.Name) access to "targets" resources.
			grant := &solarv1alpha1.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "allow-profile-ns",
					Namespace: otherNs.Name,
				},
				Spec: solarv1alpha1.ReferenceGrantSpec{
					From: []solarv1alpha1.ReferenceGrantFromSubject{
						{Group: "solar.opendefense.cloud", Kind: "Profile", Namespace: ns.Name},
					},
					To: []solarv1alpha1.ReferenceGrantToTarget{
						{Group: "solar.opendefense.cloud", Kind: "Target"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, grant)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, grant)

			// Profile in the test namespace selects targets with label env=shared.
			profile := &solarv1alpha1.Profile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "profile-cross-ns",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ProfileSpec{
					TargetSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "shared"},
					},
					ReleaseRef: corev1.LocalObjectReference{Name: "test-release"},
				},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			// A ReleaseBinding should be created in the profile's namespace for the
			// cross-namespace target, with TargetNamespace set.
			Eventually(func() int {
				return len(listOwnedBindings("profile-cross-ns"))
			}, eventuallyTimeout).Should(Equal(1))

			bindings := listOwnedBindings("profile-cross-ns")
			Expect(bindings[0].Spec.TargetRef.Name).To(Equal("target-cross-ns"))
			Expect(bindings[0].Spec.TargetNamespace).To(Equal(otherNs.Name))
			Expect(bindings[0].Spec.ReleaseRef.Name).To(Equal("test-release"))
		})

		It("should remove the ReleaseBinding when the ReferenceGrant is deleted", func() {
			otherNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "grant-del-",
				},
			}
			Expect(k8sClient.Create(ctx, otherNs)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, otherNs)

			crossNsTarget := &solarv1alpha1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "target-grant-del",
					Namespace: otherNs.Name,
					Labels:    map[string]string{"env": "revoked"},
				},
				Spec: solarv1alpha1.TargetSpec{},
			}
			Expect(k8sClient.Create(ctx, crossNsTarget)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, crossNsTarget)

			grant := &solarv1alpha1.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grant-to-revoke",
					Namespace: otherNs.Name,
				},
				Spec: solarv1alpha1.ReferenceGrantSpec{
					From: []solarv1alpha1.ReferenceGrantFromSubject{
						{Group: "solar.opendefense.cloud", Kind: "Profile", Namespace: ns.Name},
					},
					To: []solarv1alpha1.ReferenceGrantToTarget{
						{Group: "solar.opendefense.cloud", Kind: "Target"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, grant)).To(Succeed())

			profile := &solarv1alpha1.Profile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "profile-grant-del",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ProfileSpec{
					TargetSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "revoked"},
					},
					ReleaseRef: corev1.LocalObjectReference{Name: "test-release"},
				},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			// Wait for binding to appear.
			Eventually(func() int {
				return len(listOwnedBindings("profile-grant-del"))
			}, eventuallyTimeout).Should(Equal(1))

			// Delete the ReferenceGrant — the profile can no longer access the target.
			Expect(k8sClient.Delete(ctx, grant)).To(Succeed())

			// The binding should be removed once the profile reconciles.
			Eventually(func() int {
				return len(listOwnedBindings("profile-grant-del"))
			}, eventuallyTimeout).Should(Equal(0))
		})
	})
})
