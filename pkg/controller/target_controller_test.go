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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
			rtName := releaseRenderTaskName(ns.Name, "my-release", "test-release-rt", 1)
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

	Context("RegistryBinding pull secret resolution", Label("target"), func() {
		It("should populate PullSecretName in the release RenderTask when a RegistryBinding exists", func() {
			// Create a source registry with a targetPullSecretName
			sourceRegistry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-registry",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:             "example.com",
					TargetPullSecretName: "target-pull-creds",
				},
			}
			Expect(k8sClient.Create(ctx, sourceRegistry)).To(Succeed())

			// Create the render registry (for pushing)
			renderRegistry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, renderRegistry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("my-release")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-pullsecret")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			// Create a RegistryBinding linking the target to the source registry
			rb := &solarv1alpha1.RegistryBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rb-source",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistryBindingSpec{
					TargetRef:   corev1.LocalObjectReference{Name: "test-pullsecret"},
					RegistryRef: corev1.LocalObjectReference{Name: "source-registry"},
				},
			}
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())

			releaseBinding := newReleaseBinding("binding-pullsecret", "test-pullsecret", "my-release")
			Expect(k8sClient.Create(ctx, releaseBinding)).To(Succeed())

			// Verify the release RenderTask has the pull secret populated
			rtName := releaseRenderTaskName(ns.Name, "my-release", "test-pullsecret", 1)
			rt := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)
			}, eventuallyTimeout).Should(Succeed())

			Expect(rt.Spec.RendererConfig.Type).To(Equal(solarv1alpha1.RendererConfigTypeRelease))
			releaseResources := rt.Spec.RendererConfig.ReleaseConfig.Input.Resources
			Expect(releaseResources).To(HaveKey("chart"))
			Expect(releaseResources["chart"].PullSecretName).To(Equal("target-pull-creds"))
		})

		It("should leave PullSecretName empty when no RegistryBinding matches (relaxed mode)", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("my-release")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-no-rb")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			// No RegistryBinding created — relaxed mode should still succeed
			releaseBinding := newReleaseBinding("binding-no-rb", "test-no-rb", "my-release")
			Expect(k8sClient.Create(ctx, releaseBinding)).To(Succeed())

			rtName := releaseRenderTaskName(ns.Name, "my-release", "test-no-rb", 1)
			rt := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)
			}, eventuallyTimeout).Should(Succeed())

			releaseResources := rt.Spec.RendererConfig.ReleaseConfig.Input.Resources
			Expect(releaseResources).To(HaveKey("chart"))
			Expect(releaseResources["chart"].PullSecretName).To(BeEmpty())
		})

		It("should recreate release RenderTask when RegistryBinding is added after initial creation (spec drift)", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("my-release")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-drift")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			// Create ReleaseBinding without a RegistryBinding — RT will
			// have empty pull secrets in relaxed mode.
			releaseBinding := newReleaseBinding("binding-drift", "test-drift", "my-release")
			Expect(k8sClient.Create(ctx, releaseBinding)).To(Succeed())

			rtName := releaseRenderTaskName(ns.Name, "my-release", "test-drift", 1)
			rt := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)
			}, eventuallyTimeout).Should(Succeed())

			// Verify initial RT has empty pull secret
			Expect(rt.Spec.RendererConfig.ReleaseConfig.Input.Resources["chart"].PullSecretName).To(BeEmpty())
			initialUID := rt.UID

			// Now create a source registry and RegistryBinding
			sourceRegistry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "drift-source-registry",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:             "example.com",
					TargetPullSecretName: "drift-pull-creds",
				},
			}
			Expect(k8sClient.Create(ctx, sourceRegistry)).To(Succeed())

			rb := &solarv1alpha1.RegistryBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rb-drift",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistryBindingSpec{
					TargetRef:   corev1.LocalObjectReference{Name: "test-drift"},
					RegistryRef: corev1.LocalObjectReference{Name: "drift-source-registry"},
				},
			}
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())

			// The controller should detect spec drift and recreate the RT
			// with the correct pull secret.
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(rt.UID).NotTo(Equal(initialUID), "RT should have been recreated (new UID)")
				resources := rt.Spec.RendererConfig.ReleaseConfig.Input.Resources
				g.Expect(resources).To(HaveKey("chart"))
				g.Expect(resources["chart"].PullSecretName).To(Equal("drift-pull-creds"))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("should not create a RenderTask when RegistryBindings conflict on the same hostname", func() {
			// Two registries with the SAME hostname but DIFFERENT pull secrets.
			reg1 := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "conflict-reg-1", Namespace: ns.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:             "conflict.example.com",
					TargetPullSecretName: "secret-a",
				},
			}
			reg2 := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "conflict-reg-2", Namespace: ns.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:             "conflict.example.com",
					TargetPullSecretName: "secret-b",
				},
			}
			Expect(k8sClient.Create(ctx, reg1)).To(Succeed())
			Expect(k8sClient.Create(ctx, reg2)).To(Succeed())

			renderRegistry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, renderRegistry)

			target := newTarget("test-conflict")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			rb1 := &solarv1alpha1.RegistryBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "conflict-rb-1", Namespace: ns.Name},
				Spec: solarv1alpha1.RegistryBindingSpec{
					TargetRef:   corev1.LocalObjectReference{Name: "test-conflict"},
					RegistryRef: corev1.LocalObjectReference{Name: "conflict-reg-1"},
				},
			}
			rb2 := &solarv1alpha1.RegistryBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "conflict-rb-2", Namespace: ns.Name},
				Spec: solarv1alpha1.RegistryBindingSpec{
					TargetRef:   corev1.LocalObjectReference{Name: "test-conflict"},
					RegistryRef: corev1.LocalObjectReference{Name: "conflict-reg-2"},
				},
			}
			Expect(k8sClient.Create(ctx, rb1)).To(Succeed())
			Expect(k8sClient.Create(ctx, rb2)).To(Succeed())

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("my-release")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			releaseBinding := newReleaseBinding("binding-conflict", "test-conflict", "my-release")
			Expect(k8sClient.Create(ctx, releaseBinding)).To(Succeed())

			// The conflicting RegistryBindings should prevent RenderTask creation.
			// Verify no release RenderTask appears for this target.
			rtName := releaseRenderTaskName(ns.Name, "my-release", "test-conflict", 1)
			Consistently(func() bool {
				rt := &solarv1alpha1.RenderTask{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)

				return apierrors.IsNotFound(err)
			}, "3s", "500ms").Should(BeTrue(), "RenderTask should not be created when RegistryBindings conflict")
		})

		It("should recreate release RenderTask without PullSecretName when RegistryBinding is deleted", func() {
			// Create source registry with a pull secret
			sourceRegistry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "del-source-registry",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:             "example.com",
					TargetPullSecretName: "del-pull-creds",
				},
			}
			Expect(k8sClient.Create(ctx, sourceRegistry)).To(Succeed())

			renderRegistry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, renderRegistry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("my-release")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-del-rb")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			rb := &solarv1alpha1.RegistryBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rb-del",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistryBindingSpec{
					TargetRef:   corev1.LocalObjectReference{Name: "test-del-rb"},
					RegistryRef: corev1.LocalObjectReference{Name: "del-source-registry"},
				},
			}
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())

			releaseBinding := newReleaseBinding("binding-del-rb", "test-del-rb", "my-release")
			Expect(k8sClient.Create(ctx, releaseBinding)).To(Succeed())

			// Wait for the RT with pull secret populated
			rtName := releaseRenderTaskName(ns.Name, "my-release", "test-del-rb", 1)
			rt := &solarv1alpha1.RenderTask{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)).To(Succeed())
				resources := rt.Spec.RendererConfig.ReleaseConfig.Input.Resources
				g.Expect(resources).To(HaveKey("chart"))
				g.Expect(resources["chart"].PullSecretName).To(Equal("del-pull-creds"))
			}, eventuallyTimeout).Should(Succeed())

			initialUID := rt.UID

			// Delete the RegistryBinding
			Expect(k8sClient.Delete(ctx, rb)).To(Succeed())

			// The controller should detect spec drift and recreate the RT
			// without the pull secret.
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(rt.UID).NotTo(Equal(initialUID), "RT should have been recreated (new UID)")
				resources := rt.Spec.RendererConfig.ReleaseConfig.Input.Resources
				g.Expect(resources).To(HaveKey("chart"))
				g.Expect(resources["chart"].PullSecretName).To(BeEmpty())
			}, eventuallyTimeout).Should(Succeed())
		})

		It("should not create a RenderTask when RegistryBinding references a non-existent Registry", func() {
			renderRegistry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, renderRegistry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("my-release")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-bad-ref")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			// RegistryBinding pointing to a Registry that doesn't exist
			rb := &solarv1alpha1.RegistryBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rb-bad-ref",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistryBindingSpec{
					TargetRef:   corev1.LocalObjectReference{Name: "test-bad-ref"},
					RegistryRef: corev1.LocalObjectReference{Name: "nonexistent-source-registry"},
				},
			}
			Expect(k8sClient.Create(ctx, rb)).To(Succeed())

			releaseBinding := newReleaseBinding("binding-bad-ref", "test-bad-ref", "my-release")
			Expect(k8sClient.Create(ctx, releaseBinding)).To(Succeed())

			// Verify no release RenderTask appears
			rtName := releaseRenderTaskName(ns.Name, "my-release", "test-bad-ref", 1)
			Consistently(func() bool {
				rt := &solarv1alpha1.RenderTask{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)

				return apierrors.IsNotFound(err)
			}, "3s", "500ms").Should(BeTrue(), "RenderTask should not be created when RegistryBinding references non-existent Registry")

			// Verify the Target has the RegistryBindingConflict condition
			Eventually(func(g Gomega) {
				t := &solarv1alpha1.Target{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t)).To(Succeed())
				cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeReleasesRendered)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("RegistryBindingConflict"))
				g.Expect(cond.Message).To(ContainSubstring("nonexistent-source-registry"))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("should set MissingRegistryBinding condition in strict mode when no binding matches", func() {
			renderRegistry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, renderRegistry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("my-release")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-strict")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			// Enable strict mode
			targetReconciler.RegistryBindingStrict = true
			defer func() { targetReconciler.RegistryBindingStrict = false }()

			// No RegistryBinding — in strict mode this should error
			releaseBinding := newReleaseBinding("binding-strict", "test-strict", "my-release")
			Expect(k8sClient.Create(ctx, releaseBinding)).To(Succeed())

			// Verify no release RenderTask appears
			rtName := releaseRenderTaskName(ns.Name, "my-release", "test-strict", 1)
			Consistently(func() bool {
				rt := &solarv1alpha1.RenderTask{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)

				return apierrors.IsNotFound(err)
			}, "3s", "500ms").Should(BeTrue(), "RenderTask should not be created in strict mode without RegistryBinding")

			// Verify the Target has the MissingRegistryBinding condition
			Eventually(func(g Gomega) {
				t := &solarv1alpha1.Target{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t)).To(Succeed())
				cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeReleasesRendered)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("MissingRegistryBinding"))
				g.Expect(cond.Message).To(ContainSubstring("example.com"))
			}, eventuallyTimeout).Should(Succeed())
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
			relRTName := releaseRenderTaskName(ns.Name, "rel-cleanup-1", "test-cleanup", 1)
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
			relRT2Name := releaseRenderTaskName(ns.Name, "rel-cleanup-2", "test-cleanup", 1)
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

		It("should re-render bootstrap without the removed release after a ReleaseBinding is deleted", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			_ = k8sClient.Create(ctx, cv)

			rel1 := newRelease("rel-rebind-del-1")
			rel1.Spec.UniqueName = "rebind-component-1"
			Expect(k8sClient.Create(ctx, rel1)).To(Succeed())

			rel2 := newRelease("rel-rebind-del-2")
			rel2.Spec.UniqueName = "rebind-component-2"
			Expect(k8sClient.Create(ctx, rel2)).To(Succeed())

			target := newTarget("test-rebind-del")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			binding1 := newReleaseBinding("rb-rebind-del-1", "test-rebind-del", "rel-rebind-del-1")
			Expect(k8sClient.Create(ctx, binding1)).To(Succeed())

			binding2 := newReleaseBinding("rb-rebind-del-2", "test-rebind-del", "rel-rebind-del-2")
			Expect(k8sClient.Create(ctx, binding2)).To(Succeed())

			// Succeed both release RenderTasks.
			relRT1Name := releaseRenderTaskName(ns.Name, "rel-rebind-del-1", "test-rebind-del", 1)
			relRT2Name := releaseRenderTaskName(ns.Name, "rel-rebind-del-2", "test-rebind-del", 1)
			relChartURL1 := "oci://registry.example.com/" + ns.Name + "/release-rel-rebind-del-1:v0.0.0"
			relChartURL2 := "oci://registry.example.com/" + ns.Name + "/release-rel-rebind-del-2:v0.0.0"
			markRenderTaskSucceeded(relRT1Name, relChartURL1)
			markRenderTaskSucceeded(relRT2Name, relChartURL2)

			// Wait for bootstrap v0 and succeed it (both releases present).
			bootstrapV0 := targetRenderTaskName("test-rebind-del", 0)
			markRenderTaskSucceeded(bootstrapV0, "oci://registry.example.com/"+ns.Name+"/bootstrap-test-rebind-del:v0.0.0")

			Eventually(func() bool {
				t := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(t.Status.Conditions, ConditionTypeBootstrapReady)
			}, eventuallyTimeout).Should(BeTrue(), "BootstrapReady should be True before removing the binding")

			// Verify bootstrap v0 contains both releases in its input.
			rt0 := &solarv1alpha1.RenderTask{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: bootstrapV0, Namespace: ns.Name}, rt0)).To(Succeed())
			Expect(rt0.Spec.RendererConfig.BootstrapConfig.Input.Releases).To(HaveKey("rel-rebind-del-1"))
			Expect(rt0.Spec.RendererConfig.BootstrapConfig.Input.Releases).To(HaveKey("rel-rebind-del-2"))

			// Delete binding2, this should trigger a new bootstrap version without rel-rebind-del-2.
			Expect(k8sClient.Delete(ctx, binding2)).To(Succeed())

			// Bootstrap v1 must be created and should only contain rel-rebind-del-1.
			bootstrapV1 := targetRenderTaskName("test-rebind-del", 1)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: bootstrapV1, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
			}, eventuallyTimeout).Should(Succeed(), "new bootstrap RenderTask (v1) should be created after binding deletion")

			rt1 := &solarv1alpha1.RenderTask{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: bootstrapV1, Namespace: ns.Name}, rt1)).To(Succeed())
			Expect(rt1.Spec.RendererConfig.BootstrapConfig.Input.Releases).To(HaveKey("rel-rebind-del-1"),
				"bootstrap v1 should still include rel-rebind-del-1")
			Expect(rt1.Spec.RendererConfig.BootstrapConfig.Input.Releases).NotTo(HaveKey("rel-rebind-del-2"),
				"bootstrap v1 must NOT include the removed rel-rebind-del-2")

			// Mark the new bootstrap as succeeded -> this triggers stale RenderTask cleanup.
			markRenderTaskSucceeded(bootstrapV1, "oci://registry.example.com/"+ns.Name+"/bootstrap-test-rebind-del:v0.0.1")

			// The release RenderTask for the removed release must be deleted.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: relRT2Name, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})

				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue(), "stale release RenderTask for deleted binding must be cleaned up")

			// The release RenderTask for the remaining release must still exist.
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: relRT1Name, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})).To(Succeed())
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
			highRTName := releaseRenderTaskName(ns.Name, "rel-resolver-high", "test-resolver-prio", 1)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: highRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
			}, eventuallyTimeout).Should(Succeed())

			// Low-priority release must not get a RenderTask
			lowRTName := releaseRenderTaskName(ns.Name, "rel-resolver-low", "test-resolver-prio", 1)
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

			alphaRTName := releaseRenderTaskName(ns.Name, "rel-tiebreak-a", "test-resolver-tiebreak", 1)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: alphaRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
			}, eventuallyTimeout).Should(Succeed())

			zetaRTName := releaseRenderTaskName(ns.Name, "rel-tiebreak-z", "test-resolver-tiebreak", 1)
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
			istioRTName := releaseRenderTaskName(ns.Name, "rel-istio", "test-resolver-antiaffinity", 1)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: istioRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})
			}, eventuallyTimeout).Should(Succeed())

			// linkerd is blocked by anti-affinity
			linkerdRTName := releaseRenderTaskName(ns.Name, "rel-linkerd", "test-resolver-antiaffinity", 1)
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

	Context("RenderArtifact and RenderBinding lifecycle", Label("renderartifact"), func() {
		// markRenderTaskSucceeded sets JobSucceeded on a named RenderTask and
		// sets Status.ChartURL so the target controller can proceed.
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

		It("should create a RenderArtifact and RenderBinding when a release RenderTask succeeds", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("rel-art-binding")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-art-binding")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Expect(k8sClient.Create(ctx, newReleaseBinding("rb-art-binding", "test-art-binding", "rel-art-binding"))).To(Succeed())

			relRTName := releaseRenderTaskName(ns.Name, "rel-art-binding", "test-art-binding", 1)

			// Wait for the RenderTask to be created, then read its actual spec coordinates
			// so we don't need to reproduce the tag formula (which depends on Generation).
			rt := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: relRTName, Namespace: ns.Name}, rt)
			}, eventuallyTimeout).Should(Succeed())

			actualBaseURL := rt.Spec.BaseURL
			actualRepo := rt.Spec.Repository
			actualTag := rt.Spec.Tag
			expectedArtName := renderArtifactName(ns.Name, actualBaseURL, actualRepo, actualTag)
			expectedBindingName := renderBindingName(expectedArtName, "test-art-binding")

			markRenderTaskSucceeded(relRTName, "oci://"+actualBaseURL+"/"+actualRepo+":"+actualTag)

			// Both artifact and binding must be created; the binding keeps the artifact alive.
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: expectedArtName, Namespace: ns.Name}, &solarv1alpha1.RenderArtifact{})).
					To(Succeed(), "RenderArtifact should exist")
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: expectedBindingName, Namespace: ns.Name}, &solarv1alpha1.RenderBinding{})).
					To(Succeed(), "RenderBinding should exist")
			}, eventuallyTimeout).Should(Succeed())

			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: expectedArtName, Namespace: ns.Name}, &solarv1alpha1.RenderArtifact{})
				return err == nil
			}, consistentlyDuration).Should(BeTrue(), "RenderArtifact must be kept alive while its RenderBinding exists")
		})

		It("should propagate RegistryFlavor from the Registry to the RenderArtifact", func() {
			reg := newRegistry("test-registry-flavor")
			reg.Spec.Flavor = "zot"
			reg.Spec.SolarSecretRef = &corev1.LocalObjectReference{Name: "registry-credentials"}
			Expect(k8sClient.Create(ctx, reg)).To(Succeed())

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("rel-flavor")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-flavor-propagation")
			target.Spec.RenderRegistryRef.Name = "test-registry-flavor"
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Expect(k8sClient.Create(ctx, newReleaseBinding("rb-flavor", "test-flavor-propagation", "rel-flavor"))).To(Succeed())

			relRTName := releaseRenderTaskName(ns.Name, "rel-flavor", "test-flavor-propagation", 1)

			rt := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: relRTName, Namespace: ns.Name}, rt)
			}, eventuallyTimeout).Should(Succeed())

			actualBaseURL := rt.Spec.BaseURL
			actualRepo := rt.Spec.Repository
			actualTag := rt.Spec.Tag
			expectedArtName := renderArtifactName(ns.Name, actualBaseURL, actualRepo, actualTag)

			markRenderTaskSucceeded(relRTName, "oci://"+actualBaseURL+"/"+actualRepo+":"+actualTag)

			Eventually(func(g Gomega) {
				art := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: expectedArtName, Namespace: ns.Name}, art)).To(Succeed())
				g.Expect(art.Spec.RegistryFlavor).To(Equal("zot"))
			}, eventuallyTimeout).Should(Succeed(), "RenderArtifact should carry the Registry's Flavor")
		})

		It("should delete owned RenderBindings when the Target is deleted", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			cv := newComponentVersion("my-cv")
			_ = k8sClient.Create(ctx, cv)

			rel := newRelease("rel-bind-del")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-bind-deletion")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			Expect(k8sClient.Create(ctx, newReleaseBinding("rb-bind-del", "test-bind-deletion", "rel-bind-del"))).To(Succeed())

			relRTName := releaseRenderTaskName(ns.Name, "rel-bind-del", "test-bind-deletion", 1)

			rt := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: relRTName, Namespace: ns.Name}, rt)
			}, eventuallyTimeout).Should(Succeed())

			actualBaseURL := rt.Spec.BaseURL
			actualRepo := rt.Spec.Repository
			actualTag := rt.Spec.Tag
			expectedArtName := renderArtifactName(ns.Name, actualBaseURL, actualRepo, actualTag)
			expectedBindingName := renderBindingName(expectedArtName, "test-bind-deletion")

			markRenderTaskSucceeded(relRTName, "oci://"+actualBaseURL+"/"+actualRepo+":"+actualTag)

			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: expectedBindingName, Namespace: ns.Name}, &solarv1alpha1.RenderBinding{})
			}, eventuallyTimeout).Should(Succeed(), "RenderBinding should exist before target deletion")

			// Delete the Target — the target controller's finalizer path must delete owned RenderBindings.
			Expect(k8sClient.Delete(ctx, target)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: expectedBindingName, Namespace: ns.Name}, &solarv1alpha1.RenderBinding{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue(), "RenderBinding should be deleted when the owning Target is deleted")
		})
	})
})

var _ = Describe("TargetController cross-namespace ReleaseBinding", Ordered, func() {
	var providerNs *corev1.Namespace

	BeforeAll(func() {
		providerNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "provider-"},
		}
		Expect(k8sClient.Create(ctx, providerNs)).To(Succeed())
	})

	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, providerNs)).To(Succeed())
	})

	newProviderRelease := func(name string) *solarv1alpha1.Release {
		return &solarv1alpha1.Release{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: providerNs.Name},
			Spec: solarv1alpha1.ReleaseSpec{
				ComponentVersionRef: corev1.LocalObjectReference{Name: "provider-cv"},
				UniqueName:          "provider-component",
				Values:              runtime.RawExtension{Raw: []byte(`{}`)},
				TargetNamespace:     new("app-namespace"),
			},
		}
	}

	newProviderCV := func() *solarv1alpha1.ComponentVersion {
		return &solarv1alpha1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{Name: "provider-cv", Namespace: providerNs.Name},
			Spec: solarv1alpha1.ComponentVersionSpec{
				ComponentRef: corev1.LocalObjectReference{Name: "provider-comp"},
				Tag:          "v1.0.0",
				Resources: map[string]solarv1alpha1.ResourceAccess{
					"chart": {Repository: "example.com/chart", Tag: "1.0.0"},
				},
				Entrypoint: solarv1alpha1.Entrypoint{
					ResourceName: "chart",
					Type:         solarv1alpha1.EntrypointTypeHelm,
				},
			},
		}
	}

	newCrossNsBinding := func(name, targetName, releaseName, targetNamespace string) *solarv1alpha1.ReleaseBinding {
		return &solarv1alpha1.ReleaseBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: providerNs.Name},
			Spec: solarv1alpha1.ReleaseBindingSpec{
				TargetRef:       corev1.LocalObjectReference{Name: targetName},
				TargetNamespace: targetNamespace,
				ReleaseRef:      corev1.LocalObjectReference{Name: releaseName},
			},
		}
	}

	newReferenceGrant := func(name string) *solarv1alpha1.ReferenceGrant {
		return &solarv1alpha1.ReferenceGrant{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns.Name},
			Spec: solarv1alpha1.ReferenceGrantSpec{
				From: []solarv1alpha1.ReferenceGrantFromSubject{
					{Group: solarGroup, Kind: "ReleaseBinding", Namespace: providerNs.Name},
				},
				To: []solarv1alpha1.ReferenceGrantToTarget{
					{Group: solarGroup, Kind: "Target"},
				},
			},
		}
	}

	It("should create a RenderTask from a cross-namespace ReleaseBinding when a ReferenceGrant permits it", func() {
		registry := &solarv1alpha1.Registry{
			ObjectMeta: metav1.ObjectMeta{Name: "xns-registry", Namespace: ns.Name},
			Spec: solarv1alpha1.RegistrySpec{
				Hostname:       "registry.example.com",
				SolarSecretRef: &corev1.LocalObjectReference{Name: "registry-credentials"},
			},
		}
		Expect(k8sClient.Create(ctx, registry)).To(Succeed())

		cv := newProviderCV()
		Expect(k8sClient.Create(ctx, cv)).To(Succeed())

		rel := newProviderRelease("xns-release")
		Expect(k8sClient.Create(ctx, rel)).To(Succeed())

		target := &solarv1alpha1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "xns-target", Namespace: ns.Name},
			Spec: solarv1alpha1.TargetSpec{
				RenderRegistryRef: corev1.LocalObjectReference{Name: "xns-registry"},
				Userdata:          runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())

		grant := newReferenceGrant("xns-grant")
		Expect(k8sClient.Create(ctx, grant)).To(Succeed())

		binding := newCrossNsBinding("xns-binding", "xns-target", "xns-release", ns.Name)
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())

		rtName := releaseRenderTaskName(providerNs.Name, "xns-release", "xns-target", 1)
		rt := &solarv1alpha1.RenderTask{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)
		}, eventuallyTimeout).Should(Succeed(), "expected RenderTask from cross-namespace ReleaseBinding")
	})

	It("should NOT create a RenderTask from a cross-namespace ReleaseBinding without a ReferenceGrant", func() {
		registry := &solarv1alpha1.Registry{
			ObjectMeta: metav1.ObjectMeta{Name: "xns2-registry", Namespace: ns.Name},
			Spec: solarv1alpha1.RegistrySpec{
				Hostname:       "registry.example.com",
				SolarSecretRef: &corev1.LocalObjectReference{Name: "registry-credentials"},
			},
		}
		Expect(k8sClient.Create(ctx, registry)).To(Succeed())

		cv := &solarv1alpha1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{Name: "provider-cv2", Namespace: providerNs.Name},
			Spec:       newProviderCV().Spec,
		}
		Expect(k8sClient.Create(ctx, cv)).To(Succeed())

		rel := newProviderRelease("xns2-release")
		rel.Spec.ComponentVersionRef.Name = cv.Name
		Expect(k8sClient.Create(ctx, rel)).To(Succeed())

		target := &solarv1alpha1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "xns2-target", Namespace: ns.Name},
			Spec: solarv1alpha1.TargetSpec{
				RenderRegistryRef: corev1.LocalObjectReference{Name: "xns2-registry"},
				Userdata:          runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())

		// No ReferenceGrant created
		binding := newCrossNsBinding("xns2-binding", "xns2-target", "xns2-release", ns.Name)
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())

		rtName := releaseRenderTaskName(providerNs.Name, "xns2-release", "xns2-target", 1)
		Consistently(func() bool {
			rt := &solarv1alpha1.RenderTask{}
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt))
		}, 3*time.Second).Should(BeTrue(), "expected no RenderTask without a ReferenceGrant")

		Eventually(func() bool {
			t := &solarv1alpha1.Target{}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), t); err != nil {
				return false
			}
			cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeReleasesRendered)

			return cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == "NoReleaseBindings"
		}, eventuallyTimeout).Should(BeTrue(), "expected ReleasesRendered=False/NoReleaseBindings without a grant")
	})

	It("should create exactly one RenderTask when two grants cover the same provider namespace", func() {
		registry := &solarv1alpha1.Registry{
			ObjectMeta: metav1.ObjectMeta{Name: "xns3-registry", Namespace: ns.Name},
			Spec: solarv1alpha1.RegistrySpec{
				Hostname:       "registry.example.com",
				SolarSecretRef: &corev1.LocalObjectReference{Name: "registry-credentials"},
			},
		}
		Expect(k8sClient.Create(ctx, registry)).To(Succeed())

		cv := &solarv1alpha1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{Name: "provider-cv3", Namespace: providerNs.Name},
			Spec:       newProviderCV().Spec,
		}
		Expect(k8sClient.Create(ctx, cv)).To(Succeed())

		rel := newProviderRelease("xns3-release")
		rel.Spec.ComponentVersionRef.Name = cv.Name
		Expect(k8sClient.Create(ctx, rel)).To(Succeed())

		target := &solarv1alpha1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "xns3-target", Namespace: ns.Name},
			Spec: solarv1alpha1.TargetSpec{
				RenderRegistryRef: corev1.LocalObjectReference{Name: "xns3-registry"},
				Userdata:          runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())

		// Two grants, both covering the same providerNs — collectCrossNamespaceReleaseBindings
		// must deduplicate so the binding is not processed twice.
		grant1 := newReferenceGrant("xns3-grant-1")
		Expect(k8sClient.Create(ctx, grant1)).To(Succeed())
		grant2 := newReferenceGrant("xns3-grant-2")
		Expect(k8sClient.Create(ctx, grant2)).To(Succeed())

		binding := newCrossNsBinding("xns3-binding", "xns3-target", "xns3-release", ns.Name)
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())

		Eventually(func() ([]solarv1alpha1.RenderTask, error) {
			rtList := &solarv1alpha1.RenderTaskList{}
			if err := k8sClient.List(ctx, rtList, client.InNamespace(ns.Name)); err != nil {
				return nil, err
			}
			var matched []solarv1alpha1.RenderTask
			for _, rt := range rtList.Items {
				if strings.HasPrefix(rt.Name, "render-rel-xns3-release-") {
					matched = append(matched, rt)
				}
			}

			return matched, nil
		}, eventuallyTimeout).Should(HaveLen(1), "expected exactly one RenderTask for xns3-release despite overlapping grants")
	})

	It("should not assign a cross-namespace ReleaseBinding to a same-named Target in the provider namespace", func() {
		// Two Targets with the same name exist: one in ns (consumer) and one in providerNs.
		// The cross-namespace ReleaseBinding points to the consumer Target.
		// Without the targetNamespace index filter the provider-ns Target would incorrectly
		// pick up the cross-namespace binding via the same-namespace list.
		consumerRegistry := &solarv1alpha1.Registry{
			ObjectMeta: metav1.ObjectMeta{Name: "xns4-registry", Namespace: ns.Name},
			Spec: solarv1alpha1.RegistrySpec{
				Hostname:       "registry.example.com",
				SolarSecretRef: &corev1.LocalObjectReference{Name: "registry-credentials"},
			},
		}
		Expect(k8sClient.Create(ctx, consumerRegistry)).To(Succeed())

		// Give the provider-ns Target its own Registry so it clears registry resolution
		// and reaches the ReleaseBinding lookup — where the targetNamespace filter is
		// what prevents it from picking up the cross-namespace binding.
		providerRegistry := &solarv1alpha1.Registry{
			ObjectMeta: metav1.ObjectMeta{Name: "xns4-provider-registry", Namespace: providerNs.Name},
			Spec: solarv1alpha1.RegistrySpec{
				Hostname:       "registry.example.com",
				SolarSecretRef: &corev1.LocalObjectReference{Name: "registry-credentials"},
			},
		}
		Expect(k8sClient.Create(ctx, providerRegistry)).To(Succeed())

		cv := &solarv1alpha1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{Name: "provider-cv4", Namespace: providerNs.Name},
			Spec:       newProviderCV().Spec,
		}
		Expect(k8sClient.Create(ctx, cv)).To(Succeed())

		rel := newProviderRelease("xns4-release")
		rel.Spec.ComponentVersionRef.Name = cv.Name
		Expect(k8sClient.Create(ctx, rel)).To(Succeed())

		// Consumer target in ns — this is the intended target of the cross-namespace binding.
		consumerTarget := &solarv1alpha1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "xns4-target", Namespace: ns.Name},
			Spec: solarv1alpha1.TargetSpec{
				RenderRegistryRef: corev1.LocalObjectReference{Name: "xns4-registry"},
				Userdata:          runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		Expect(k8sClient.Create(ctx, consumerTarget)).To(Succeed())

		// Provider target with the SAME name in providerNs — must not receive the binding.
		// References its own registry so registry resolution succeeds and the test verifies
		// the targetNamespace filter is what prevents the spurious RenderTask.
		providerTarget := &solarv1alpha1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "xns4-target", Namespace: providerNs.Name},
			Spec: solarv1alpha1.TargetSpec{
				RenderRegistryRef: corev1.LocalObjectReference{Name: "xns4-provider-registry"},
				Userdata:          runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		Expect(k8sClient.Create(ctx, providerTarget)).To(Succeed())

		grant := newReferenceGrant("xns4-grant")
		Expect(k8sClient.Create(ctx, grant)).To(Succeed())

		// Cross-namespace binding: lives in providerNs, targets consumer "xns4-target" in ns.
		binding := newCrossNsBinding("xns4-binding", "xns4-target", "xns4-release", ns.Name)
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())

		rtName := releaseRenderTaskName(providerNs.Name, "xns4-release", "xns4-target", 1)

		// Consumer target in ns must eventually get its RenderTask.
		Eventually(func() error {
			rt := &solarv1alpha1.RenderTask{}
			return k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: ns.Name}, rt)
		}, eventuallyTimeout).Should(Succeed(), "expected RenderTask for consumer target in ns")

		// Provider-ns target with the same name must never get a RenderTask from this binding.
		Consistently(func() bool {
			rt := &solarv1alpha1.RenderTask{}
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: rtName, Namespace: providerNs.Name}, rt))
		}, 3*time.Second).Should(BeTrue(), "expected no RenderTask for provider-ns target with same name")
	})
})

var _ = Describe("mapReferenceGrantToTargets", func() {
	It("enqueues the Target namespace from a cross-namespace ReleaseBinding on ComponentVersion grant change", func() {
		providerNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "cv-grant-provider-"}}
		Expect(k8sClient.Create(ctx, providerNs)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, providerNs)).To(Succeed()) })

		targetNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "cv-grant-target-"}}
		Expect(k8sClient.Create(ctx, targetNs)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, targetNs)).To(Succeed()) })

		binding := &solarv1alpha1.ReleaseBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "cv-grant-binding", Namespace: providerNs.Name},
			Spec: solarv1alpha1.ReleaseBindingSpec{
				TargetRef:       corev1.LocalObjectReference{Name: "my-target"},
				TargetNamespace: targetNs.Name,
				ReleaseRef:      corev1.LocalObjectReference{Name: "my-release"},
			},
		}
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())

		grant := &solarv1alpha1.ReferenceGrant{
			ObjectMeta: metav1.ObjectMeta{Name: "cv-grant", Namespace: providerNs.Name},
			Spec: solarv1alpha1.ReferenceGrantSpec{
				From: []solarv1alpha1.ReferenceGrantFromSubject{
					{Group: solarGroup, Kind: "Release", Namespace: providerNs.Name},
				},
				To: []solarv1alpha1.ReferenceGrantToTarget{
					{Group: solarGroup, Kind: "ComponentVersion"},
				},
			},
		}

		requests := targetReconciler.mapReferenceGrantToTargets(ctx, grant)
		Expect(requests).To(ConsistOf(reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "my-target", Namespace: targetNs.Name},
		}))
	})
})

var _ = Describe("registryHost", func() {
	DescribeTable("extracts the host from a repository string",
		func(repository, expected string) {
			Expect(registryHost(repository)).To(Equal(expected))
		},
		Entry("simple host/repo", "registry.example.com/foo/bar", "registry.example.com"),
		Entry("host with port", "registry.example.com:5000/foo/bar", "registry.example.com:5000"),
		Entry("oci:// prefix", "oci://registry.example.com/foo/bar", "registry.example.com"),
		Entry("oci:// prefix with port", "oci://registry.example.com:5000/charts/my-chart", "registry.example.com:5000"),
		Entry("bare host (no path)", "registry.example.com", "registry.example.com"),
		Entry("bare host with oci://", "oci://registry.example.com", "registry.example.com"),
		Entry("deeply nested path", "ghcr.io/org/sub/repo/chart", "ghcr.io"),
		Entry("uppercase host normalised", "Registry.Example.COM:5000/foo/bar", "registry.example.com:5000"),
	)
})

var _ = Describe("pullSecretsTag", func() {
	It("is deterministic for the same input", func() {
		resolved := map[string]solarv1alpha1.ResolvedResourceAccess{
			"chart": {PullSecretName: "regcred"},
			"image": {PullSecretName: "other"},
		}
		Expect(pullSecretsTag(resolved)).To(Equal(pullSecretsTag(resolved)))
	})

	It("changes when pull secrets change", func() {
		a := map[string]solarv1alpha1.ResolvedResourceAccess{
			"chart": {PullSecretName: "regcred"},
		}
		b := map[string]solarv1alpha1.ResolvedResourceAccess{
			"chart": {PullSecretName: "other"},
		}
		Expect(pullSecretsTag(a)).NotTo(Equal(pullSecretsTag(b)))
	})

	It("changes between empty and non-empty pull secrets", func() {
		empty := map[string]solarv1alpha1.ResolvedResourceAccess{
			"chart": {PullSecretName: ""},
		}
		nonEmpty := map[string]solarv1alpha1.ResolvedResourceAccess{
			"chart": {PullSecretName: "regcred"},
		}
		Expect(pullSecretsTag(empty)).NotTo(Equal(pullSecretsTag(nonEmpty)))
	})

	It("returns a consistent hash for empty resources", func() {
		empty := map[string]solarv1alpha1.ResolvedResourceAccess{}
		Expect(pullSecretsTag(empty)).To(Equal(pullSecretsTag(empty)))
	})
})

var _ = Describe("resolveResources", func() {
	resources := map[string]solarv1alpha1.ResourceAccess{
		"chart": {
			Repository: "registry.example.com/charts/my-chart",
			Tag:        "1.0.0",
			Insecure:   true,
		},
		"image": {
			Repository: "docker.io/library/nginx",
			Tag:        "1.25",
		},
	}

	Context("relaxed mode (strict=false)", func() {
		It("should populate PullSecretName when a matching host is found", func() {
			lookup := map[string]string{
				"registry.example.com": "my-pull-secret",
			}
			resolved, err := resolveResources(resources, lookup, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveLen(2))
			Expect(resolved["chart"].PullSecretName).To(Equal("my-pull-secret"))
			Expect(resolved["chart"].Repository).To(Equal("registry.example.com/charts/my-chart"))
			Expect(resolved["chart"].Tag).To(Equal("1.0.0"))
			Expect(resolved["chart"].Insecure).To(BeTrue())
		})

		It("should leave PullSecretName empty when no matching host is found", func() {
			lookup := map[string]string{
				"registry.example.com": "my-pull-secret",
			}
			resolved, err := resolveResources(resources, lookup, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["image"].PullSecretName).To(BeEmpty())
		})

		It("should succeed with empty lookup", func() {
			resolved, err := resolveResources(resources, map[string]string{}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveLen(2))
			Expect(resolved["chart"].PullSecretName).To(BeEmpty())
			Expect(resolved["image"].PullSecretName).To(BeEmpty())
		})

		It("should preserve Helm metadata", func() {
			valTpl := "image: {{ .resources.chart.tag }}"
			res := map[string]solarv1alpha1.ResourceAccess{
				"chart": {
					Repository: "registry.example.com/charts/my-chart",
					Tag:        "1.0.0",
					Helm: &solarv1alpha1.HelmResourceMetadata{
						Name:           "my-chart",
						Version:        "1.0.0",
						ValuesTemplate: &valTpl,
					},
				},
			}
			resolved, err := resolveResources(res, map[string]string{}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["chart"].Helm).NotTo(BeNil())
			Expect(resolved["chart"].Helm.Name).To(Equal("my-chart"))
			Expect(resolved["chart"].Helm.ValuesTemplate).To(Equal(&valTpl))
		})
	})

	Context("strict mode (strict=true)", func() {
		It("should succeed when all hosts have matching bindings", func() {
			lookup := map[string]string{
				"registry.example.com": "my-pull-secret",
				"docker.io":            "docker-secret",
			}
			resolved, err := resolveResources(resources, lookup, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["chart"].PullSecretName).To(Equal("my-pull-secret"))
			Expect(resolved["image"].PullSecretName).To(Equal("docker-secret"))
		})

		It("should return error when a host has no matching binding", func() {
			lookup := map[string]string{
				"registry.example.com": "my-pull-secret",
				// docker.io is missing
			}
			_, err := resolveResources(resources, lookup, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no RegistryBinding for host"))
			Expect(err.Error()).To(ContainSubstring("docker.io"))
		})

		It("should return error with empty lookup", func() {
			_, err := resolveResources(resources, map[string]string{}, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no RegistryBinding for host"))
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
