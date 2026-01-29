// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"go.opendefense.cloud/kit/envtest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CatalogItemController", func() {
	var (
		ctx = envtest.Context()
		ns  = setupTest(ctx)
	)

	Context("when reconciling CatalogItems", func() {
		It("should create CatalogItem", func() {
			ci := &solarv1alpha1.CatalogItem{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    ns.Name,
					GenerateName: "test-",
				},
				Spec: solarv1alpha1.CatalogItemSpec{},
			}
			Expect(k8sClient.Create(ctx, ci)).To(Succeed())
		})
	})
})
