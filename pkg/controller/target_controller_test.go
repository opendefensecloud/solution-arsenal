// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
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
					Hostname:             "registry.example.com",
					TargetPullSecretName: "registry-pull-secret",
					SolarSecretRef: &corev1.LocalObjectReference{
						Name: "registry-credentials",
					},
				},
			}
		}

		newRegistryBinding = func(name, targetName, registryName string) *solarv1alpha1.RegistryBinding {
			return &solarv1alpha1.RegistryBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RegistryBindingSpec{
					TargetRef:   corev1.LocalObjectReference{Name: targetName},
					RegistryRef: corev1.LocalObjectReference{Name: registryName},
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

		It("should set ReleasesRendered=NoBindings when no ReleaseBindings exist", func() {
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

				return cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == "NoBindings"
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should create a release RenderTask when ReleaseBinding exists", func() {
			registry := newRegistry("test-registry")
			_ = k8sClient.Create(ctx, registry)

			// Create a source registry and RegistryBinding so resources can be resolved
			sourceReg := newRegistry("source-registry")
			sourceReg.Spec.Hostname = "example.com"
			sourceReg.Spec.TargetPullSecretName = "source-pull-secret"
			sourceReg.Spec.SolarSecretRef = nil
			Expect(k8sClient.Create(ctx, sourceReg)).To(Succeed())

			cv := newComponentVersion("my-cv")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			rel := newRelease("my-release")
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			target := newTarget("test-release-rt")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			regBinding := newRegistryBinding("regbinding-1", "test-release-rt", "source-registry")
			Expect(k8sClient.Create(ctx, regBinding)).To(Succeed())

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
			Expect(rt.Spec.SecretRef).NotTo(BeNil())
			Expect(rt.Spec.SecretRef.Name).To(Equal("registry-credentials"))
			Expect(rt.Spec.OwnerKind).To(Equal("Target"))
			Expect(rt.Spec.OwnerName).To(Equal("test-release-rt"))
			// Verify resolved resources carry pullSecretName
			Expect(rt.Spec.RendererConfig.ReleaseConfig.Input.Resources["chart"].PullSecretName).To(Equal("source-pull-secret"))
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

			// Create source registry and RegistryBinding for resource resolution
			sourceReg := newRegistry("source-registry")
			sourceReg.Spec.Hostname = "example.com"
			sourceReg.Spec.SolarSecretRef = nil
			_ = k8sClient.Create(ctx, sourceReg)

			cv := newComponentVersion("my-cv")
			_ = k8sClient.Create(ctx, cv)

			rel1 := newRelease("rel-cleanup-1")
			Expect(k8sClient.Create(ctx, rel1)).To(Succeed())

			rel2 := newRelease("rel-cleanup-2")
			rel2.Spec.ComponentVersionRef.Name = "my-cv"
			Expect(k8sClient.Create(ctx, rel2)).To(Succeed())

			target := newTarget("test-cleanup")
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			// RegistryBindings for resource resolution and render registry
			regBinding := newRegistryBinding("regbinding-cleanup-source", "test-cleanup", "source-registry")
			Expect(k8sClient.Create(ctx, regBinding)).To(Succeed())
			renderRegBinding := newRegistryBinding("regbinding-cleanup-render", "test-cleanup", "test-registry")
			Expect(k8sClient.Create(ctx, renderRegBinding)).To(Succeed())

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

				return err != nil
			}, eventuallyTimeout).Should(BeTrue(), "stale bootstrap RenderTask %s should be deleted", bootstrapV0)

			// Verify the new bootstrap RenderTask (v1) still exists
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: bootstrapV1, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})).To(Succeed())

			// Verify release RenderTasks are NOT cleaned up
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: relRTName, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: relRT2Name, Namespace: ns.Name}, &solarv1alpha1.RenderTask{})).To(Succeed())
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

				return err != nil
			}, eventuallyTimeout).Should(BeTrue())
		})
	})
})
