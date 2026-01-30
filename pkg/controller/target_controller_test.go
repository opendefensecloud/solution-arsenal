// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opendefense.cloud/kit/envtest"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("TargetController", Ordered, func() {
	var (
		ctx = envtest.Context()
		ns  = setupTest(ctx)
	)

	Context("when reconciling Target", Label("target"), func() {
		It("should create HydratedTarget for Target", func() {
			target := &solarv1alpha1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.TargetSpec{},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())

			hydratedTarget := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(target), hydratedTarget)
			}).Should(Succeed())
			Expect(hydratedTarget).NotTo(BeNil())
		})

		// TODO: implement tests for updates, deletion
	})
})
