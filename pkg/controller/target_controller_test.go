// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"go.opendefense.cloud/kit/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TargetController", Ordered, func() {
	var (
		ctx = envtest.Context()
		ns  = setupTest(ctx)
	)

	Context("when reconciling Target", Label("target"), func() {
		It("should create HydratedTarget for Target", func() {
			target := newTargetWithEmptySpec("test-target", ns.Name, nil)
			target.Spec = solarv1alpha1.TargetSpec{
				Userdata: runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
				Releases: map[string]corev1.LocalObjectReference{
					"example-release": {Name: "initial-release-name"},
				},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			hydratedTarget := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(target), hydratedTarget)
			}).Should(Succeed())
			Expect(hydratedTarget).NotTo(BeNil())
		})
	})

	Context("when Target is deleted", Label("target"), func() {
		It("should clean up HydratedTarget", func() {
			target := newTargetWithEmptySpec("test-target-to-delete", ns.Name, nil)
			target.Spec = solarv1alpha1.TargetSpec{
				Userdata: runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
				Releases: map[string]corev1.LocalObjectReference{
					"example-release": {Name: "initial-release-name"},
				},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			hydratedTarget := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(target), hydratedTarget)
			}).Should(Succeed())
			Expect(hydratedTarget).NotTo(BeNil())

			Expect(k8sClient.Delete(ctx, target)).To(Succeed())

			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(target), hydratedTarget)
			}).ShouldNot(Succeed())
		})
	})

	Context("when Target is updated", Label("target"), func() {
		It("should update HydratedTarget", func() {
			target := newTargetWithEmptySpec("test-target-to-update", ns.Name, nil)
			target.Spec = solarv1alpha1.TargetSpec{
				Userdata: runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
				Releases: map[string]corev1.LocalObjectReference{
					"example-release": {Name: "initial-release-name"},
				},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			hydratedTarget := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(target), hydratedTarget)
			}).Should(Succeed())
			Expect(hydratedTarget.Spec.Releases).To(Equal(target.Spec.Releases))

			// Get fresh version of Target and update example-release
			latestTarget := &solarv1alpha1.Target{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(target), latestTarget)).To(Succeed())
			latestTarget.Spec.Releases["example-release"] = corev1.LocalObjectReference{Name: "updated-release-name"}
			Expect(k8sClient.Update(ctx, latestTarget)).To(Succeed())

			// Verify HydratedTarget has been updated by the controller
			Eventually(func() bool {
				ht := &solarv1alpha1.HydratedTarget{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), ht)
				if err != nil {
					return false
				}
				if release, exists := ht.Spec.Releases["example-release"]; exists {
					return release.Name == "updated-release-name"
				}

				return false
			}).Should(BeTrue(), "HydratedTarget was not updated with new release name")
		})
		It("should update the Profiles of the HydratedTarget", func() {
			// Create a profile and two targets with labels so that target 1 does not match
			// the profile and target 2 matches the profile
			profile := newProfile("profile", ns.Name, map[string]string{"wave": "2"})
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			target1 := newTargetWithEmptySpec("target-1", ns.Name, map[string]string{"wave": "1"})
			Expect(k8sClient.Create(ctx, target1)).To(Succeed())
			target2 := newTargetWithEmptySpec("target-2", ns.Name, map[string]string{"wave": "2"})
			Expect(k8sClient.Create(ctx, target2)).To(Succeed())

			expectProfilesInHydratedTarget(ctx, target1)
			expectProfilesInHydratedTarget(ctx, target2, profile)

			// Update the labels of both targets so that target 1 now matches the profile
			// and target 2 doesn't match the profile anymore
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(target1), target1)).To(Succeed())
			target1.ObjectMeta.Labels = map[string]string{"wave": "2"}
			Expect(k8sClient.Update(ctx, target1)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(target2), target2)).To(Succeed())
			target2.ObjectMeta.Labels = map[string]string{"wave": "3"}
			Expect(k8sClient.Update(ctx, target2)).To(Succeed())

			expectProfilesInHydratedTarget(ctx, target1, profile)
			expectProfilesInHydratedTarget(ctx, target2)
		})
	})

	Context("when Profile is created", Label("target"), func() {
		It("should update HydratedTarget of Targets matching the Profile", func() {
			target1 := newTargetWithEmptySpec("target-1", ns.Name, map[string]string{
				"env":    "prod",
				"region": "north",
			})
			Expect(k8sClient.Create(ctx, target1)).To(Succeed())
			target2 := newTargetWithEmptySpec("target-2", ns.Name, map[string]string{"env": "prod"})
			Expect(k8sClient.Create(ctx, target2)).To(Succeed())
			target3 := newTargetWithEmptySpec("target-3", ns.Name, map[string]string{"env": "test"})
			Expect(k8sClient.Create(ctx, target3)).To(Succeed())

			expectProfilesInHydratedTarget(ctx, target1)
			expectProfilesInHydratedTarget(ctx, target2)
			expectProfilesInHydratedTarget(ctx, target3)

			// Create two profiles so that target 1 matches two profiles, target 2 matches one profile,
			// and target 3 doesn't match any profile
			profile1 := newProfile("profile-1", ns.Name, map[string]string{"env": "prod"})
			Expect(k8sClient.Create(ctx, profile1)).To(Succeed())
			profile2 := newProfile("profile-2", ns.Name, map[string]string{"region": "north"})
			Expect(k8sClient.Create(ctx, profile2)).To(Succeed())

			expectProfilesInHydratedTarget(ctx, target1, profile1, profile2)
			expectProfilesInHydratedTarget(ctx, target2, profile1)
			expectProfilesInHydratedTarget(ctx, target3)

		})
		It("should update HydratedTarget for all Targets when the Profile doesn't define a target selector", func() {
			target1 := newTargetWithEmptySpec("target-1", ns.Name, map[string]string{})
			Expect(k8sClient.Create(ctx, target1)).To(Succeed())
			target2 := newTargetWithEmptySpec("target-2", ns.Name, map[string]string{})
			Expect(k8sClient.Create(ctx, target2)).To(Succeed())

			// Create a profile with no target selector so that it matches all targets
			profile := newProfile("profile", ns.Name, map[string]string{})
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			expectProfilesInHydratedTarget(ctx, target1, profile)
			expectProfilesInHydratedTarget(ctx, target2, profile)
		})
	})

	Context("when Profile is updated", Label("target"), func() {
		It("should update HydratedTarget of matching Target", func() {
			// Create a profile and a target so that they don't match
			profile := newProfile("profile", ns.Name, map[string]string{"wave": "2"})
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			target := newTargetWithEmptySpec("target", ns.Name, map[string]string{"wave": "1"})
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			expectProfilesInHydratedTarget(ctx, target)

			// Update the profile so that it matches the target
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(profile), profile)
			}).Should(Succeed())
			profile.Spec.TargetSelector = metav1.LabelSelector{
				MatchLabels: map[string]string{"wave": "1"}}
			Expect(k8sClient.Update(ctx, profile)).To(Succeed())

			expectProfilesInHydratedTarget(ctx, target, profile)
		})
	})

	Context("when Profile is deleted", Label("target"), func() {
		It("should update HydratedTarget of matchting Target", func() {
			// Create a profile a target so that they match
			profile := newProfile("profile", ns.Name, map[string]string{"wave": "1"})
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			target := newTargetWithEmptySpec("target", ns.Name, map[string]string{"wave": "1"})
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			expectProfilesInHydratedTarget(ctx, target, profile)

			// Delete the profile
			Eventually(func() error {
				return k8sClient.Delete(ctx, profile)
			}).Should(Succeed())

			expectProfilesInHydratedTarget(ctx, target)
		})
	})

	Context("Profile Predicate", Label("target"), func() {
		It("should trigger when a profile has been created", func() {
			predicate := profileSelectionPredicate()

			ev := event.CreateEvent{Object: &solarv1alpha1.Profile{}}

			Expect(predicate.Create(ev)).To(BeTrue())
		})
		It("should trigger when a profile has been deleted", func() {
			predicate := profileSelectionPredicate()

			ev := event.DeleteEvent{Object: &solarv1alpha1.Profile{}}

			Expect(predicate.Delete(ev)).To(BeTrue())
		})
		It("should trigger when the target selector of a profile has been updated", func() {
			predicate := profileSelectionPredicate()

			oldProfile := newProfile("profile", ns.Name, map[string]string{"wave": "1"})
			newProfile := oldProfile.DeepCopy()
			newProfile.Spec = solarv1alpha1.ProfileSpec{
				TargetSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"wave": "2"},
				},
			}

			ev := event.UpdateEvent{ObjectOld: oldProfile, ObjectNew: newProfile}

			Expect(predicate.Update(ev)).To(BeTrue())
		})
		It("should not trigger when other than the target selector of a profile has been updated", func() {
			predicate := profileSelectionPredicate()

			oldProfile := newProfile("profile", ns.Name, map[string]string{"wave": "1"})

			newProfile := oldProfile.DeepCopy()
			newProfile.ObjectMeta.Name = "profile-updated"

			ev := event.UpdateEvent{ObjectOld: oldProfile, ObjectNew: newProfile}

			Expect(predicate.Update(ev)).To(BeFalse())
		})
	})

})

func newTargetWithEmptySpec(name, namespace string, labels map[string]string) *solarv1alpha1.Target {
	return &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: solarv1alpha1.TargetSpec{},
	}
}

func newProfile(name, namespace string, matchLabels map[string]string) *solarv1alpha1.Profile {
	return &solarv1alpha1.Profile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: solarv1alpha1.ProfileSpec{
			TargetSelector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
		},
	}
}

func expectProfilesInHydratedTarget(ctx context.Context, target *solarv1alpha1.Target, expectedProfiles ...*solarv1alpha1.Profile) {
	GinkgoHelper()
	ht := &solarv1alpha1.HydratedTarget{}
	Eventually(func(g Gomega) {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), ht)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ht.Spec.Profiles).To(HaveLen(len(expectedProfiles)))
		for _, p := range expectedProfiles {
			g.Expect(ht.Spec.Profiles).To(HaveKeyWithValue(p.Name, corev1.LocalObjectReference{
				Name: p.Name,
			}))
		}
	}).Should(Succeed())
}
