// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opendefense.cloud/kit/envtest"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/test"
	"go.opendefense.cloud/solar/test/registry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DiscoveryController", Ordered, func() {
	var (
		ctx         = envtest.Context()
		ns          = setupTest(ctx)
		testServer  *httptest.Server
		registryURL string
	)

	BeforeAll(func() {
		reg := registry.New()
		testServer = httptest.NewServer(reg.HandleFunc())

		testServerUrl, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		registryURL = testServerUrl.Host

		_, err = test.Run(exec.Command("./bin/ocm", "transfer", "ctf", "./test/fixtures/helmdemo-ctf", fmt.Sprintf("http://%s/test", registryURL)))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		testServer.Close()
	})

	Context("when reconciling Discoveries", func() {
		It("should create a Pod for a discovery resource", func() {
			d := &solarv1alpha1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-discovery",
					Namespace:    ns.Name,
				},
				Spec: solarv1alpha1.DiscoverySpec{
					Registry: solarv1alpha1.Registry{
						RegistryURL: registryURL,
					},
				},
			}
			Expect(k8sClient.Create(ctx, d)).To(Succeed())

			// Verify artifact workflows were created
			pods := &corev1.PodList{}
			Eventually(func() int {
				err := k8sClient.List(ctx, pods, client.InNamespace(ns.Name))
				if err != nil {
					return 0
				}
				return len(pods.Items)
			}).Should(Equal(1))

			// Verify status contains version of discovery
			Eventually(func() string {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(d), d); err != nil {
					return ""
				}
				return d.Status.PodDiscoveryVersion
			}).Should(Equal(d.GetResourceVersion()))

		})
	})
})
