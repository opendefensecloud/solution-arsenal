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
})
