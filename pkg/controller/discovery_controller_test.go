// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
	"go.opendefense.cloud/kit/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/test"
	testregistry "go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DiscoveryController", Ordered, func() {
	var (
		ctx         = envtest.Context()
		ns          = setupTest(ctx)
		testServer  *httptest.Server
		registryURL string
	)

	BeforeAll(func() {
		reg := testregistry.New(registry.Logger(log.New(io.Discard, "", 0)))
		testServer = httptest.NewServer(reg.HandleFunc())

		testServerUrl, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		registryURL = testServerUrl.Host

		_, err = test.Run(exec.Command(
			"./bin/ocm",
			"transfer",
			"ctf",
			"./test/fixtures/helmdemo-ctf",
			fmt.Sprintf("http://%s/test", registryURL),
		))

		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		testServer.Close()
	})

	Context("when reconciling Discoveries", func() {
		It("should create a Pod for a discovery resource", func() {
			d := &solarv1alpha1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.DiscoverySpec{
					Registry: solarv1alpha1.Registry{
						RegistryURL: registryURL,
					},
				},
			}
			Expect(k8sClient.Create(ctx, d)).To(Succeed())

			// Check for secret
			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: discoveryPrefixed(d.Name), Namespace: ns.Name}, secret)
			}).Should(Succeed())

			Expect(secret).NotTo(BeNil())
			Expect(secret.Data).To(HaveKey("config.yaml"))
			Expect(string(secret.Data["config.yaml"])).To(ContainSubstring(registryURL))

			// Check for pod
			pod := &corev1.Pod{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: discoveryPrefixed(d.Name), Namespace: ns.Name}, pod)
			}).Should(Succeed())
			Expect(pod).NotTo(BeNil())
			Expect(pod.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", discoveryPrefixed(d.Name)))

			// Check for service
			svc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: discoveryPrefixed(d.Name), Namespace: ns.Name}, svc)
			}).Should(Succeed())
			Expect(svc).NotTo(BeNil())

			// Verify service selector
			Expect(svc.Spec.Selector).To(HaveKeyWithValue("app.kubernetes.io/name", discoveryPrefixed(d.Name)))

			// Verify status contains generation of discovery
			initialGen := d.GetGeneration()
			Eventually(func() int64 {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(d), d); err != nil {
					return -1
				}

				return d.Status.PodGeneration
			}).Should(Equal(d.GetGeneration()))

			d.Spec.DiscoveryInterval = &metav1.Duration{Duration: time.Hour * 24}
			Expect(k8sClient.Update(ctx, d)).To(Succeed())

			// Verify status contains new generation of discovery
			Eventually(func() int64 {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(d), d); err != nil {
					return -1
				}

				return d.Status.PodGeneration
			}).Should(Not(Equal(initialGen)))
		})
	})
})
