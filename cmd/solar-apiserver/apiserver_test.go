// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opendefense.cloud/kit/envtest"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Component", func() {
	var (
		ctx  = envtest.Context()
		ns   = SetupTest(ctx)
		comp = &solarv1alpha1.Component{}
	)

	Context("Component", func() {
		It("should allow creating an order", func() {
			By("creating a test order")
			comp = &solarv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    ns.Name,
					GenerateName: "test-",
				},
				Spec: solarv1alpha1.ComponentSpec{},
			}
			Expect(k8sClient.Create(ctx, comp)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), comp)).To(Succeed())
		})
		It("should allow deleting an order", func() {
			By("deleting a test order")
			Expect(k8sClient.Delete(ctx, comp)).To(Succeed())
		})
	})

})
