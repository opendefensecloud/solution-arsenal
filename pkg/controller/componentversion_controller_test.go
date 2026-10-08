// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ComponentVersionReconciler", Ordered, func() {
	var (
		validCV = func(name string, componentName string) *solarv1alpha1.ComponentVersion {
			return &solarv1alpha1.ComponentVersion{
				Name:      name,
				Namespace: ns.Name,
				Spec: solarv1alpha1.ComponentVersionSpec{
					ComponentName: componentName,
					Tag:           "v1.0.0",
				},
			}
		}
	)

	Describe("self-finalizer", func() {
		It("adds componentVersionFinalizer to a live ComponentVersion", func() {
			cv := validCV("cvf-cv-add", "example.com/cvf")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, cv, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, cv))
			})

			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.ComponentVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(componentVersionFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("removes componentVersionFinalizer when the ComponentVersion is deleted", func() {
			cv := validCV("cvf-cv-del", "example.com/cvf")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.ComponentVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(componentVersionFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			Expect(k8sClient.Delete(ctx, cv)).To(Succeed())

			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), &solarv1alpha1.ComponentVersion{}))
			}, eventuallyTimeout).Should(BeTrue())
		})
	})
})
