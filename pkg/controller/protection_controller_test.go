// Copyright BWI GmbH and Solution Arsenal contributors
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

// A protection finalizer must follow the reference: switching a referrer to another
// object releases the one it left, so that object can be deleted (#948).
var _ = Describe("protection finalizers follow switched references", func() {
	var (
		// create creates obj and removes it, finalizers included, when the spec ends.
		create = func(obj client.Object) {
			GinkgoHelper()
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, obj, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, obj))
			})
		}

		finalizersOf = func(g Gomega, obj client.Object) []string {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())

			return obj.GetFinalizers()
		}

		// update re-reads obj, applies mutate and writes it back, retrying on conflicts.
		update = func(obj client.Object, mutate func()) {
			GinkgoHelper()
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
				mutate()
				g.Expect(k8sClient.Update(ctx, obj)).To(Succeed())
			}, eventuallyTimeout).Should(Succeed())
		}

		gone = func(obj client.Object) func() bool {
			return func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj))
			}
		}

		componentVersion = func(name string) *solarv1alpha1.ComponentVersion {
			return &solarv1alpha1.ComponentVersion{
				Name:      name,
				Namespace: ns.Name,
				Spec: solarv1alpha1.ComponentVersionSpec{
					ComponentName: name,
					Tag:           "v1.0.0",
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"chart": {Repository: "example.com/charts/" + name, Tag: "1.0.0"},
					},
					Entrypoint: solarv1alpha1.Entrypoint{ResourceName: "chart", Type: solarv1alpha1.EntrypointTypeHelm},
				},
			}
		}

		release = func(name, cvName string) *solarv1alpha1.Release {
			return &solarv1alpha1.Release{
				Name:      name,
				Namespace: ns.Name,
				Spec: solarv1alpha1.ReleaseSpec{
					ComponentVersionRef: solarv1alpha1.ObjectReference{Name: cvName},
					UniqueName:          name,
				},
			}
		}

		registry = func(name string) *solarv1alpha1.Registry {
			return &solarv1alpha1.Registry{
				Name:      name,
				Namespace: ns.Name,
				Spec:      solarv1alpha1.RegistrySpec{Hostname: "registry.example.com"},
			}
		}
	)

	Describe("ComponentVersion, referenced by Release", func() {
		It("moves the protection when a Release switches, and the old ComponentVersion can be deleted", func() {
			a, b := componentVersion("pf-cv-switch-a"), componentVersion("pf-cv-switch-b")
			create(a)
			create(b)
			rel := release("pf-rel-switch", a.Name)
			create(rel)
			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, a)).To(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			update(rel, func() { rel.Spec.ComponentVersionRef.Name = b.Name })

			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, a)).NotTo(ContainElement(componentVersionRefFinalizer))
				g.Expect(finalizersOf(g, b)).To(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			Expect(k8sClient.Delete(ctx, a)).To(Succeed())
			Eventually(gone(a), eventuallyTimeout).Should(BeTrue())
		})

		It("completes a pending deletion once the Release switches away", func() {
			a, b := componentVersion("pf-cv-pending-a"), componentVersion("pf-cv-pending-b")
			create(a)
			create(b)
			rel := release("pf-rel-pending", a.Name)
			create(rel)
			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, a)).To(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			Expect(k8sClient.Delete(ctx, a)).To(Succeed())
			Consistently(gone(a), consistentlyDuration).Should(BeFalse())

			update(rel, func() { rel.Spec.ComponentVersionRef.Name = b.Name })
			Eventually(gone(a), eventuallyTimeout).Should(BeTrue())
		})

		It("keeps the protection while another Release still references it", func() {
			a, b := componentVersion("pf-cv-shared-a"), componentVersion("pf-cv-shared-b")
			create(a)
			create(b)
			stays, moves := release("pf-rel-shared-stays", a.Name), release("pf-rel-shared-moves", a.Name)
			create(stays)
			create(moves)
			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, a)).To(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			update(moves, func() { moves.Spec.ComponentVersionRef.Name = b.Name })
			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, b)).To(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			Consistently(func(g Gomega) {
				g.Expect(finalizersOf(g, a)).To(ContainElement(componentVersionRefFinalizer))
			}, consistentlyDuration).Should(Succeed())
		})

		It("leaves neither ComponentVersion protected after a switch and the Release's deletion", func() {
			a, b := componentVersion("pf-cv-then-delete-a"), componentVersion("pf-cv-then-delete-b")
			create(a)
			create(b)
			rel := release("pf-rel-then-delete", a.Name)
			create(rel)
			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, a)).To(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			update(rel, func() { rel.Spec.ComponentVersionRef.Name = b.Name })
			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, b)).To(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
			Expect(k8sClient.Delete(ctx, rel)).To(Succeed())
			Eventually(gone(rel), eventuallyTimeout).Should(BeTrue())

			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, a)).NotTo(ContainElement(componentVersionRefFinalizer))
				g.Expect(finalizersOf(g, b)).NotTo(ContainElement(componentVersionRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("releases a protection no Release accounts for", func() {
			// The state the bug left behind: protected, but nothing references it.
			leaked := componentVersion("pf-cv-leaked")
			leaked.Finalizers = []string{componentVersionRefFinalizer}
			create(leaked)

			Expect(k8sClient.Delete(ctx, leaked)).To(Succeed())
			Eventually(gone(leaked), eventuallyTimeout).Should(BeTrue())
		})
	})

	Describe("Release, referenced by Profile and ReleaseBinding", func() {
		It("moves the protection when a Profile switches", func() {
			r1, r2 := release("pf-prof-r1", "my-cv"), release("pf-prof-r2", "my-cv")
			create(r1)
			create(r2)
			// No Target matches, so the Profile owns no ReleaseBindings that would
			// keep referencing r1.
			profile := &solarv1alpha1.Profile{
				Name:      "pf-prof-switch",
				Namespace: ns.Name,
				Spec: solarv1alpha1.ProfileSpec{
					TargetSelector: metav1.LabelSelector{MatchLabels: map[string]string{"pf": "no-such-target"}},
					ReleaseRef:     corev1.LocalObjectReference{Name: r1.Name},
				},
			}
			create(profile)
			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, r1)).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			update(profile, func() { profile.Spec.ReleaseRef.Name = r2.Name })

			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, r1)).NotTo(ContainElement(releaseRefFinalizer))
				g.Expect(finalizersOf(g, r2)).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("moves the protection when a ReleaseBinding switches", func() {
			r1, r2 := release("pf-rb-r1", "my-cv"), release("pf-rb-r2", "my-cv")
			create(r1)
			create(r2)
			rb := &solarv1alpha1.ReleaseBinding{
				Name:      "pf-rb-switch",
				Namespace: ns.Name,
				Spec: solarv1alpha1.ReleaseBindingSpec{
					TargetRef:  solarv1alpha1.ObjectReference{Name: "my-target"},
					ReleaseRef: corev1.LocalObjectReference{Name: r1.Name},
				},
			}
			create(rb)
			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, r1)).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			update(rb, func() { rb.Spec.ReleaseRef.Name = r2.Name })

			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, r1)).NotTo(ContainElement(releaseRefFinalizer))
				g.Expect(finalizersOf(g, r2)).To(ContainElement(releaseRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})
	})

	Describe("Registry, referenced by Target and RegistryBinding", func() {
		It("moves the protection when a Target switches", func() {
			r1, r2 := registry("pf-tgt-reg1"), registry("pf-tgt-reg2")
			create(r1)
			create(r2)
			target := &solarv1alpha1.Target{
				Name:      "pf-tgt-switch",
				Namespace: ns.Name,
				Spec:      solarv1alpha1.TargetSpec{RenderRegistryRef: solarv1alpha1.ObjectReference{Name: r1.Name}},
			}
			create(target)
			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, r1)).To(ContainElement(registryRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			update(target, func() { target.Spec.RenderRegistryRef.Name = r2.Name })

			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, r1)).NotTo(ContainElement(registryRefFinalizer))
				g.Expect(finalizersOf(g, r2)).To(ContainElement(registryRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("moves the protection when a RegistryBinding switches", func() {
			r1, r2 := registry("pf-rgb-reg1"), registry("pf-rgb-reg2")
			create(r1)
			create(r2)
			rb := &solarv1alpha1.RegistryBinding{
				Name:      "pf-rgb-switch",
				Namespace: ns.Name,
				Spec: solarv1alpha1.RegistryBindingSpec{
					TargetRef:   solarv1alpha1.ObjectReference{Name: "my-target"},
					RegistryRef: corev1.LocalObjectReference{Name: r1.Name},
				},
			}
			create(rb)
			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, r1)).To(ContainElement(registryRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			update(rb, func() { rb.Spec.RegistryRef.Name = r2.Name })

			Eventually(func(g Gomega) {
				g.Expect(finalizersOf(g, r1)).NotTo(ContainElement(registryRefFinalizer))
				g.Expect(finalizersOf(g, r2)).To(ContainElement(registryRefFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})
	})
})
