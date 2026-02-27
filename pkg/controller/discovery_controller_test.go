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
	rbacv1 "k8s.io/api/rbac/v1"
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

			// Verify service account was created
			sa := &corev1.ServiceAccount{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: discoveryPrefixed(d.Name), Namespace: ns.Name}, sa)
			}).Should(Succeed())
			Expect(sa).NotTo(BeNil())

			// Verify cluster role binding was created
			crb := &rbacv1.ClusterRoleBinding{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-worker", discoveryPrefixed(d.Name))}, crb)
			}).Should(Succeed())
			Expect(crb).NotTo(BeNil())
			Expect(crb.RoleRef.Name).To(Equal("solar-controller-manager"))
			Expect(crb.Subjects).To(HaveLen(1))
			Expect(crb.Subjects[0].Name).To(Equal(discoveryPrefixed(d.Name)))
			Expect(crb.Subjects[0].Namespace).To(Equal(ns.Name))

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

		It("should handle existing resources (idempotency)", func() {
			d := &solarv1alpha1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery-idempotent",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.DiscoverySpec{
					Registry: solarv1alpha1.Registry{
						RegistryURL: registryURL,
					},
				},
			}
			Expect(k8sClient.Create(ctx, d)).To(Succeed())

			// Wait for initial resources
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: discoveryPrefixed(d.Name), Namespace: ns.Name}, &corev1.Pod{})
			}).Should(Succeed())

			// Verify CRB exists with correct roleRef
			crb := &rbacv1.ClusterRoleBinding{}
			crbName := fmt.Sprintf("%s-worker", discoveryPrefixed(d.Name))
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: crbName}, crb)
			}).Should(Succeed())
			Expect(crb.RoleRef.Name).To(Equal("solar-controller-manager"))

			// Modify CRB to test update
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crbName}, crb)).To(Succeed())
			crb.RoleRef.Name = "custom-role"
			Expect(k8sClient.Update(ctx, crb)).To(Succeed())

			// Trigger reconciliation again by updating discovery
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(d), d)).To(Succeed())
			d.Spec.DiscoveryInterval = &metav1.Duration{Duration: time.Hour * 48}
			Expect(k8sClient.Update(ctx, d)).To(Succeed())

			// Verify CRB was reconciled back to correct roleRef
			Eventually(func() string {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crbName}, crb)).To(Succeed())
				return crb.RoleRef.Name
			}).Should(Equal("solar-controller-manager"))

			// Verify pod still exists and was not duplicated
			podList := &corev1.PodList{}
			Eventually(func() int {
				Expect(k8sClient.List(ctx, podList, client.InNamespace(ns.Name), client.MatchingLabels{"app.kubernetes.io/name": discoveryPrefixed(d.Name)})).To(Succeed())
				return len(podList.Items)
			}).Should(Equal(1))
		})
	})
})
