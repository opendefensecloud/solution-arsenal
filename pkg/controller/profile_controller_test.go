// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
})
