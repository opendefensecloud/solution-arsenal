// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opendefense.cloud/kit/envtest"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("DiscoveryController", func() {
	var (
		ctx = envtest.Context()
		ns  = setupTest(ctx)
	)

	Context("when reconciling Discoverys", func() {
		It("should create Discovery", func() {
			ci := &solarv1alpha1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    ns.Name,
					GenerateName: "test-",
				},
				Spec: solarv1alpha1.DiscoverySpec{},
			}
			Expect(k8sClient.Create(ctx, ci)).To(Succeed())
		})
	})
})
