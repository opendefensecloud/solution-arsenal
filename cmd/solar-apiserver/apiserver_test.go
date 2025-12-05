// Copyright 2025 BWI GmbH and Artifact Conduit contributors
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

var _ = Describe("CatalogItem", func() {
	var (
		ctx   = envtest.Context()
		ns    = SetupTest(ctx)
		order = &solarv1alpha1.CatalogItem{}
	)

	Context("CatalogItem", func() {
		It("should allow creating an order", func() {
			By("creating a test order")
			order = &solarv1alpha1.CatalogItem{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    ns.Name,
					GenerateName: "test-",
				},
				Spec: solarv1alpha1.CatalogItemSpec{},
			}
			Expect(k8sClient.Create(ctx, order)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(order), order)).To(Succeed())
		})
		It("should allow deleting an order", func() {
			By("deleting a test order")
			Expect(k8sClient.Delete(ctx, order)).To(Succeed())
		})
	})

})
