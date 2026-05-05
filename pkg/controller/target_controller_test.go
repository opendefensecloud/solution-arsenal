// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TargetController", Ordered, func() {
	var (
		newTarget = func(name string) *solarv1alpha1.Target {
			return &solarv1alpha1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.TargetSpec{
					RenderRegistryRef: corev1.LocalObjectReference{Name: "test-registry"},
					Userdata:          runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
				},
			}
		}

		newRegistry = func(name string) *solarv1alpha1.Registry {
			return &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname: "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{
						Name: "registry-credentials",
					},
				},
			}
		}

		newReleaseBinding = func(name, targetName, releaseName string) *solarv1alpha1.ReleaseBinding {
			return &solarv1alpha1.ReleaseBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ReleaseBindingSpec{
					TargetRef:  corev1.LocalObjectReference{Name: targetName},
					ReleaseRef: corev1.LocalObjectReference{Name: releaseName},
				},
			}
		}

		newRelease = func(name string) *solarv1alpha1.Release {
			return &solarv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{Name: "my-cv"},
					UniqueName:          "my-unique-component",
					Values:              runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
					TargetNamespace:     new("my-namespace"),
				},
			}
		}

		newComponentVersion = func(name string) *solarv1alpha1.ComponentVersion {
			return &solarv1alpha1.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.ComponentVersionSpec{
					ComponentRef: corev1.LocalObjectReference{Name: "my-component"},
					Tag:          "v1.0.0",
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"chart": {Repository: "example.com/resources/chart", Tag: "1.0.0"},
					},
					Entrypoint: solarv1alpha1.Entrypoint{
						ResourceName: "chart",
						Type:         solarv1alpha1.EntrypointTypeHelm,
					},
				},
			}
		}
	)

	Context("when reconciling Target", Label("target"), func() {
		It("should add a finalizer to a new Target", func() {
			registry := newRegistry("test-registry")
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			target := newTarget("test-finalizer")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}

				return slices.Contains(t.Finalizers, targetFinalizer)
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should set RegistryResolved=False when Registry does not exist", func() {
			target := newTarget("test-no-registry")
			target.Spec.RenderRegistryRef.Name = "nonexistent-registry"
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}
				cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeRegistryResolved)

				return cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == "NotFound"
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should set RegistryResolved=False when Registry has no SolarSecretRef", func() {
			registry := newRegistry("test-registry-nosecret")
			registry.Spec.SolarSecretRef = nil
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			target := newTarget("test-no-secret")
			target.Spec.RenderRegistryRef.Name = "test-registry-nosecret"
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}
				cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeRegistryResolved)

				return cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == "MissingSolarSecretRef"
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should set ReleasesRendered=NoReleaseBindings when no ReleaseBindings exist", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry) // may already exist

			target := newTarget("test-no-bindings")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}
				cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeReleasesRendered)

				return cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == "NoReleaseBindings"
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should create a release RenderTask when ReleaseBinding exists", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("my-release")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-release-rt")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			binding := newReleaseBinding("binding-1", "test-release-rt", "my-release")
			Expect(k8sClient.Create(ctx, binding)).To(Succeed())

			// Verify a release RenderTask was created
			rtName := releaseRenderTaskName("my-release", "test-release-rt", 1)
			rt := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)
			}, eventuallyTimeout).Should(Succeed())

			Expect(rt.Spec.RendererConfig.Type).To(Equal(solarv1alpha1.RendererConfigTypeRelease))
			Expect(rt.Spec.RendererConfig.ReleaseConfig.TargetNamespace).To(Equal("my-namespace"))
			Expect(rt.Spec.BaseURL).To(Equal("registry.example.com"))
			Expect(rt.Spec.PushSecretRef).NotTo(BeNil())
			Expect(rt.Spec.PushSecretRef.Name).To(Equal("registry-credentials"))
			Expect(rt.Spec.OwnerKind).To(Equal("Target"))
			Expect(rt.Spec.OwnerName).To(Equal("test-release-rt"))
		})
	})

	Context("when bootstrap version changes", Label("target"), func() {
		markRenderTaskSucceeded := func(name, chartURL string) {
			rt := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns.Name}, rt)
			}, eventuallyTimeout).Should(Succeed())

			apimeta.SetStatusCondition(&rt.Status.Conditions, metav1.Condition{
				Type:   ConditionTypeJobSucceeded,
				Status: metav1.ConditionTrue,
				Reason: "JobSucceeded",
			})
			rt.Status.ChartURL = chartURL
			ExpectWithOffset(1, k8sClient.Status().Update(ctx, rt)).To(Succeed())
		}

		It("should clean up stale bootstrap RenderTasks after a new one succeeds", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			_ = k8sClient.Create(ctx, cv)

			rel1 := newRelease("rel-cleanup-1")
			rel1.Spec.UniqueName = "cleanup-component-1"
			Expect(k8sClient.Create(ctx, rel1)).To(Succeed())

			rel2 := newRelease("rel-cleanup-2")
			rel2.Spec.ComponentVersionRef.Name = "my-cv"
			rel2.Spec.UniqueName = "cleanup-component-2"
			Expect(k8sClient.Create(ctx, rel2)).To(Succeed())

			target := newTarget("test-cleanup")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			binding1 := newReleaseBinding("binding-cleanup-1", "test-cleanup", "rel-cleanup-1")
			Expect(k8sClient.Create(ctx, binding1)).To(Succeed())

			// Wait for release RenderTask, then mark it succeeded
			relRTName := releaseRenderTaskName("rel-cleanup-1", "test-cleanup", 1)
			markRenderTaskSucceeded(relRTName, "oci://registry.example.com/"+ns.Name+"/release-rel-cleanup-1:v0.0.0")

			// Wait for the first bootstrap RenderTask (version 0)
			bootstrapV0 := targetRenderTaskName("test-cleanup", 0)
			markRenderTaskSucceeded(bootstrapV0, "oci://registry.example.com/"+ns.Name+"/bootstrap-test-cleanup:v0.0.0")

			// Verify BootstrapReady=True
			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(t.Status.Conditions, ConditionTypeBootstrapReady)
			}, eventuallyTimeout).Should(BeTrue())

			// Add a second release binding — triggers new bootstrap version
			binding2 := newReleaseBinding("binding-cleanup-2", "test-cleanup", "rel-cleanup-2")
			Expect(k8sClient.Create(ctx, binding2)).To(Succeed())

			// Wait for second release RenderTask, then mark it succeeded
			relRT2Name := releaseRenderTaskName("rel-cleanup-2", "test-cleanup", 1)
			markRenderTaskSucceeded(relRT2Name, "oci://registry.example.com/"+ns.Name+"/release-rel-cleanup-2:v0.0.0")

			// Wait for the new bootstrap RenderTask (version 1)
			bootstrapV1 := targetRenderTaskName("test-cleanup", 1)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: bootstrapV1, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
			}, eventuallyTimeout).Should(Succeed())

			// Mark the new bootstrap RenderTask as succeeded
			markRenderTaskSucceeded(bootstrapV1, "oci://registry.example.com/"+ns.Name+"/bootstrap-test-cleanup:v0.0.1")

			// Verify the old bootstrap RenderTask (v0) is cleaned up
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: bootstrapV0, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})

				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue(), "stale bootstrap RenderTask %s should be deleted", bootstrapV0)

			// Verify the new bootstrap RenderTask (v1) still exists
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: bootstrapV1, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})).To(Succeed())

			// Verify release RenderTasks are NOT cleaned up
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: relRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: relRT2Name, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})).To(Succeed())
		})
	})

	Context("release resolver", Label("resolver"), func() {
		It("should only create a RenderTask for the higher-priority release on a uniqueName conflict", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			relHigh := newRelease("rel-resolver-high")
			relHigh.Spec.UniqueName = "shared-component"
			relHigh.Spec.Priority = 5
			Expect(k8sClient.Create(ctx, relHigh)).To(Succeed())

			relLow := newRelease("rel-resolver-low")
			relLow.Spec.UniqueName = "shared-component"
			relLow.Spec.Priority = 1
			Expect(k8sClient.Create(ctx, relLow)).To(Succeed())

			target := newTarget("test-resolver-prio")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Expect(k8sClient.Create(ctx, newReleaseBinding("binding-resolver-high", "test-resolver-prio", "rel-resolver-high"))).To(Succeed())
			Expect(k8sClient.Create(ctx, newReleaseBinding("binding-resolver-low", "test-resolver-prio", "rel-resolver-low"))).To(Succeed())

			// High-priority release gets a RenderTask
			highRTName := releaseRenderTaskName("rel-resolver-high", "test-resolver-prio", 1)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: highRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
			}, eventuallyTimeout).Should(Succeed())

			// Low-priority release must not get a RenderTask
			lowRTName := releaseRenderTaskName("rel-resolver-low", "test-resolver-prio", 1)
			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: lowRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
				return apierrors.IsNotFound(err)
			}, 2*time.Second, 100*time.Millisecond).Should(BeTrue())

			// Target should reflect the conflict resolution
			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}
				cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeReleasesResolved)

				return cond != nil && cond.Status == metav1.ConditionTrue && cond.Reason == "Resolved" &&
					strings.Contains(cond.Message, "binding-resolver-low")
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should use namespace-qualified bindingKey as tiebreaker when priorities are equal", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			relA := newRelease("rel-tiebreak-a")
			relA.Spec.UniqueName = "tied-component"
			relA.Spec.Priority = 0
			Expect(k8sClient.Create(ctx, relA)).To(Succeed())

			relZ := newRelease("rel-tiebreak-z")
			relZ.Spec.UniqueName = "tied-component"
			relZ.Spec.Priority = 0
			Expect(k8sClient.Create(ctx, relZ)).To(Succeed())

			target := newTarget("test-resolver-tiebreak")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			// "binding-alpha" < "binding-zeta" alphabetically → rel-tiebreak-a wins
			Expect(k8sClient.Create(ctx, newReleaseBinding("binding-alpha", "test-resolver-tiebreak", "rel-tiebreak-a"))).To(Succeed())
			Expect(k8sClient.Create(ctx, newReleaseBinding("binding-zeta", "test-resolver-tiebreak", "rel-tiebreak-z"))).To(Succeed())

			alphaRTName := releaseRenderTaskName("rel-tiebreak-a", "test-resolver-tiebreak", 1)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: alphaRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
			}, eventuallyTimeout).Should(Succeed())

			zetaRTName := releaseRenderTaskName("rel-tiebreak-z", "test-resolver-tiebreak", 1)
			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: zetaRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
				return apierrors.IsNotFound(err)
			}, 2*time.Second, 100*time.Millisecond).Should(BeTrue())

			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}
				cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeReleasesResolved)

				return cond != nil && cond.Status == metav1.ConditionTrue && cond.Reason == "Resolved" &&
					strings.Contains(cond.Message, "binding-zeta")
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should block a release whose anti-affinity matches another accepted release", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			// istio: high priority, carries the "service-mesh" label
			istio := newRelease("rel-istio")
			istio.Labels = map[string]string{"solar.opendefense.cloud/category": "service-mesh"}
			istio.Spec.UniqueName = "istio"
			istio.Spec.Priority = 10
			Expect(k8sClient.Create(ctx, istio)).To(Succeed())

			// linkerd: lower priority, declares anti-affinity against any service-mesh release
			linkerd := newRelease("rel-linkerd")
			linkerd.Spec.UniqueName = "linkerd"
			linkerd.Spec.Priority = 5
			linkerd.Spec.AntiAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{"solar.opendefense.cloud/category": "service-mesh"},
			}
			Expect(k8sClient.Create(ctx, linkerd)).To(Succeed())

			target := newTarget("test-resolver-antiaffinity")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Expect(k8sClient.Create(ctx, newReleaseBinding("binding-istio", "test-resolver-antiaffinity", "rel-istio"))).To(Succeed())
			Expect(k8sClient.Create(ctx, newReleaseBinding("binding-linkerd", "test-resolver-antiaffinity", "rel-linkerd"))).To(Succeed())

			// istio gets a RenderTask
			istioRTName := releaseRenderTaskName("rel-istio", "test-resolver-antiaffinity", 1)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: istioRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
			}, eventuallyTimeout).Should(Succeed())

			// linkerd is blocked by anti-affinity
			linkerdRTName := releaseRenderTaskName("rel-linkerd", "test-resolver-antiaffinity", 1)
			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: linkerdRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
				return apierrors.IsNotFound(err)
			}, 2*time.Second, 100*time.Millisecond).Should(BeTrue())

			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}
				cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeReleasesResolved)

				return cond != nil && cond.Status == metav1.ConditionTrue && cond.Reason == "Resolved"
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should set ReleasesResolved=NoConflicts when there are no conflicts", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("rel-no-conflict")
			rel.Spec.UniqueName = "unique-component"
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-resolver-no-conflict")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Expect(k8sClient.Create(ctx, newReleaseBinding("binding-no-conflict", "test-resolver-no-conflict", "rel-no-conflict"))).To(Succeed())

			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}
				cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeReleasesResolved)

				return cond != nil && cond.Status == metav1.ConditionTrue && cond.Reason == "NoConflicts"
			}, eventuallyTimeout).Should(BeTrue())
		})
	})

	Context("when Target is deleted", Label("target"), func() {
		It("should remove the finalizer and allow deletion", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			target := newTarget("test-delete")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			// Wait for finalizer to be added
			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}

				return slices.Contains(t.Finalizers, targetFinalizer)
			}, eventuallyTimeout).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, target)).To(Succeed())

			// Verify Target is eventually deleted
			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t)

				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue())
		})
	})
})

var _ = Describe("resolveReleaseConflicts", func() {
	makeRI := func(bindingKey, name, uniqueName string, priority int32, relLabels map[string]string, antiAffinity *metav1.LabelSelector, cv *solarv1alpha1.ComponentVersion) releaseInfo {
		return releaseInfo{
			bindingKey: bindingKey,
			name:       name,
			cv:         cv,
			release: &solarv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: relLabels,
				},
				Spec: solarv1alpha1.ReleaseSpec{
					UniqueName:   uniqueName,
					Priority:     priority,
					AntiAffinity: antiAffinity,
				},
			},
		}
	}

	makeCV := func(componentName string) *solarv1alpha1.ComponentVersion {
		return &solarv1alpha1.ComponentVersion{
			Spec: solarv1alpha1.ComponentVersionSpec{
				ComponentRef: corev1.LocalObjectReference{Name: componentName},
			},
		}
	}

	It("returns nil for nil input", func() {
		accepted, skipped := resolveReleaseConflicts(nil)
		Expect(accepted).To(BeNil())
		Expect(skipped).To(BeNil())
	})

	It("deduplicates releases with empty uniqueName using the component name from the CV", func() {
		low := makeRI("ns/binding-low", "rel-low", "", 1, nil, nil, makeCV("kyverno"))
		high := makeRI("ns/binding-high", "rel-high", "", 5, nil, nil, makeCV("kyverno"))
		accepted, skipped := resolveReleaseConflicts([]releaseInfo{low, high})
		Expect(accepted).To(HaveLen(1))
		Expect(accepted[0].name).To(Equal("rel-high"))
		Expect(skipped).To(HaveLen(1))
		Expect(skipped[0]).To(ContainSubstring("ns/binding-low"))
		Expect(skipped[0]).To(ContainSubstring("kyverno"))
	})

	Context("uniqueName deduplication", func() {
		It("keeps the release with the higher priority", func() {
			low := makeRI("ns/binding-low", "rel-low", "kyverno", 1, nil, nil, nil)
			high := makeRI("ns/binding-high", "rel-high", "kyverno", 5, nil, nil, nil)
			accepted, skipped := resolveReleaseConflicts([]releaseInfo{low, high})
			Expect(accepted).To(HaveLen(1))
			Expect(accepted[0].name).To(Equal("rel-high"))
			Expect(skipped).To(HaveLen(1))
			Expect(skipped[0]).To(ContainSubstring("ns/binding-low"))
			Expect(skipped[0]).To(ContainSubstring("kyverno"))
		})

		It("uses namespace-qualified bindingKey as tiebreaker for equal priority", func() {
			a := makeRI("ns-a/binding-alpha", "rel-a", "kyverno", 0, nil, nil, nil)
			b := makeRI("ns-z/binding-zeta", "rel-b", "kyverno", 0, nil, nil, nil)
			// pass b before a to verify sort is not input-order-dependent
			accepted, skipped := resolveReleaseConflicts([]releaseInfo{b, a})
			Expect(accepted).To(HaveLen(1))
			Expect(accepted[0].bindingKey).To(Equal("ns-a/binding-alpha"))
			Expect(skipped).To(HaveLen(1))
			Expect(skipped[0]).To(ContainSubstring("ns-z/binding-zeta"))
		})
	})

	Context("anti-affinity", func() {
		It("blocks a release whose anti-affinity matches an already-accepted release", func() {
			// istio comes first (higher priority), gets accepted
			istio := makeRI("ns/binding-istio", "istio", "istio", 10,
				map[string]string{"solar.opendefense.cloud/category": "service-mesh"}, nil, nil)
			// linkerd declares anti-affinity against any service-mesh release
			linkerd := makeRI("ns/binding-linkerd", "linkerd", "linkerd", 5, nil,
				&metav1.LabelSelector{
					MatchLabels: map[string]string{"solar.opendefense.cloud/category": "service-mesh"},
				}, nil)
			accepted, skipped := resolveReleaseConflicts([]releaseInfo{istio, linkerd})
			Expect(accepted).To(HaveLen(1))
			Expect(accepted[0].name).To(Equal("istio"))
			Expect(skipped).To(HaveLen(1))
			Expect(skipped[0]).To(ContainSubstring("ns/binding-linkerd"))
			Expect(skipped[0]).To(ContainSubstring("anti-affinity"))
		})

		It("blocks a lower-priority release when a higher-priority release declares anti-affinity against it", func() {
			// A has higher priority and declares anti-affinity against releases labelled "service-mesh".
			// B has that label but no anti-affinity of its own.
			// A is accepted first; B should be blocked by A's anti-affinity.
			a := makeRI("ns/binding-a", "rel-a", "", 10, nil,
				&metav1.LabelSelector{
					MatchLabels: map[string]string{"solar.opendefense.cloud/category": "service-mesh"},
				}, makeCV("rel-a"))
			b := makeRI("ns/binding-b", "rel-b", "", 5,
				map[string]string{"solar.opendefense.cloud/category": "service-mesh"}, nil, makeCV("rel-b"))
			accepted, skipped := resolveReleaseConflicts([]releaseInfo{a, b})
			Expect(accepted).To(HaveLen(1))
			Expect(accepted[0].name).To(Equal("rel-a"))
			Expect(skipped).To(HaveLen(1))
			Expect(skipped[0]).To(ContainSubstring("ns/binding-b"))
			Expect(skipped[0]).To(ContainSubstring("anti-affinity"))
		})

		It("accepts a release when no other release matches its anti-affinity", func() {
			ri := makeRI("ns/binding-a", "rel-a", "", 0, nil,
				&metav1.LabelSelector{
					MatchLabels: map[string]string{"solar.opendefense.cloud/category": "service-mesh"},
				}, makeCV("rel-a"))
			accepted, skipped := resolveReleaseConflicts([]releaseInfo{ri})
			Expect(accepted).To(HaveLen(1))
			Expect(skipped).To(BeEmpty())
		})

		It("skips a release with an invalid anti-affinity selector", func() {
			ri := makeRI("ns/binding-invalid", "rel-invalid", "", 0, nil,
				&metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "k", Operator: "InvalidOp", Values: []string{"v"}},
					},
				}, makeCV("rel-invalid"))
			accepted, skipped := resolveReleaseConflicts([]releaseInfo{ri})
			Expect(accepted).To(BeEmpty())
			Expect(skipped).To(HaveLen(1))
			Expect(skipped[0]).To(ContainSubstring("ns/binding-invalid"))
			Expect(skipped[0]).To(ContainSubstring("invalid"))
		})
	})
})
