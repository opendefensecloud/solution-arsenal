// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		It("should create required resources for a discovery resource", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "root-bundle",
				},
				Data: map[string]string{
					"trust-bundle.pem": "certs-data",
				},
			}
			d := &solarv1alpha1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.DiscoverySpec{
					Registry: solarv1alpha1.Registry{
						RegistryURL: registryURL,
						CAConfigMapRef: corev1.LocalObjectReference{
							Name: cm.Name,
						},
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

			Expect(pod.Spec.Volumes).To(HaveLen(2))
			Expect(pod.Spec.Volumes[0].Name).To(Equal("config"))
			Expect(pod.Spec.Volumes[1].Name).To(Equal("ca-bundle"))
			Expect(pod.Spec.Volumes[1].ConfigMap.Name).To(Equal("root-bundle"))

			container := pod.Spec.Containers[0]
			Expect(strings.Join(container.Args, " ")).To(ContainSubstring("--namespace " + ns.Name))
			Expect(container.VolumeMounts).To(HaveLen(2))
			Expect(container.VolumeMounts[0].Name).To(Equal("config"))
			Expect(container.VolumeMounts[1].Name).To(Equal("ca-bundle"))

			Expect(container.Env).To(ContainElement(corev1.EnvVar{
				Name:  "SSL_CERT_FILE",
				Value: "/etc/ssl/certs/ca-bundle.pem",
			}))

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
			saName := discoveryPrefixed(d.Name)
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: saName, Namespace: ns.Name}, sa)
			}).Should(Succeed())
			Expect(sa).NotTo(BeNil())

			// Verify role binding was created
			rb := &rbacv1.RoleBinding{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: "solar-discovery-worker", Namespace: ns.Name}, rb)
			}).Should(Succeed())
			Expect(rb).NotTo(BeNil())
			Expect(rb.RoleRef.Name).To(Equal("solar-discovery-worker"))
			Expect(rb.Subjects).To(HaveLen(1))
			Expect(rb.Subjects[0].Name).To(Equal(saName))
			Expect(rb.Subjects[0].Namespace).To(Equal(ns.Name))

			// Verify role was created
			role := &rbacv1.Role{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: "solar-discovery-worker", Namespace: ns.Name}, role)
			}).Should(Succeed())
			Expect(role).NotTo(BeNil())
			Expect(role.Rules).To(HaveLen(1))
			Expect(role.Rules[0].Verbs).To(ConsistOf("get", "list", "watch", "create", "update", "patch", "delete"))
			Expect(role.Rules[0].APIGroups).To(ConsistOf(solarv1alpha1.GroupName))
			Expect(role.Rules[0].Resources).To(ConsistOf("components", "componentversions"))
		})

		It("should cleanup resources for a deleted discovery resource", func() {
			// Create a Discovery
			d := &solarv1alpha1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.DiscoverySpec{
					Registry: solarv1alpha1.Registry{
						RegistryURL: registryURL,
					},
					DiscoveryInterval: &metav1.Duration{
						Duration: time.Hour * 12,
					},
				},
			}
			Expect(k8sClient.Create(ctx, d)).To(Succeed())

			// Wait for all resources to be created
			pod := &corev1.Pod{}
			svc := &corev1.Service{}
			rb := &rbacv1.RoleBinding{}
			role := &rbacv1.Role{}
			Eventually(func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: discoveryPrefixed(d.Name), Namespace: ns.Name}, pod); err != nil {
					return err
				}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: discoveryPrefixed(d.Name), Namespace: ns.Name}, svc); err != nil {
					return err
				}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "solar-discovery-worker", Namespace: ns.Name}, rb); err != nil {
					return err
				}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "solar-discovery-worker", Namespace: ns.Name}, role); err != nil {
					return err
				}

				return nil
			}).Should(Succeed())

			// Delete Discovery
			Expect(k8sClient.Delete(ctx, d)).To(Succeed())

			checkGone := func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
				err := k8sClient.Get(ctx, key, obj)
				if apierrors.IsNotFound(err) {
					return nil
				}
				if err != nil {
					return err
				}

				return fmt.Errorf("Object `%s` was still there", obj.GetName())
			}

			// Validate resources were removed
			Eventually(func() error {
				if err := checkGone(ctx, types.NamespacedName{Name: discoveryPrefixed(d.Name), Namespace: ns.Name}, pod); err != nil {
					return err
				}
				if err := checkGone(ctx, types.NamespacedName{Name: discoveryPrefixed(d.Name), Namespace: ns.Name}, svc); err != nil {
					return err
				}
				if err := checkGone(ctx, types.NamespacedName{Name: "solar-discovery-worker", Namespace: ns.Name}, rb); err != nil {
					return err
				}
				if err := checkGone(ctx, types.NamespacedName{Name: "solar-discovery-worker", Namespace: ns.Name}, role); err != nil {
					return err
				}

				return nil
			}).Should(Succeed())
		})

		It("should increase pod generation when spec changes", func() {
			d := &solarv1alpha1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.DiscoverySpec{
					Registry: solarv1alpha1.Registry{
						RegistryURL: registryURL,
					},
					DiscoveryInterval: &metav1.Duration{
						Duration: time.Hour * 12,
					},
				},
			}
			Expect(k8sClient.Create(ctx, d)).To(Succeed())
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

			// Verify Role exists with correct verbs
			role := &rbacv1.Role{}
			roleName := "solar-discovery-worker"
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: roleName, Namespace: ns.Name}, role)
			}).Should(Succeed())
			Expect(role.Rules[0].Verbs).To(ConsistOf("get", "list", "watch", "create", "update", "patch", "delete"))

			// Verify CRB exists with correct roleRef
			rb := &rbacv1.RoleBinding{}
			rbName := roleName
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: rbName, Namespace: ns.Name}, rb)
			}).Should(Succeed())
			Expect(rb.RoleRef.Name).To(Equal("solar-discovery-worker"))

			// Modify Role to test update
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: roleName, Namespace: ns.Name}, role)).To(Succeed())
			role.Rules[0].Verbs = []string{"get", "list", "watch"}
			Expect(k8sClient.Update(ctx, rb)).To(Succeed())

			// Modify CRB to test update
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: rbName, Namespace: ns.Name}, rb)).To(Succeed())
			rb.Subjects = append(rb.Subjects, rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "foo",
				Namespace: "bar",
			})
			Expect(k8sClient.Update(ctx, rb)).To(Succeed())

			// Trigger reconciliation again by updating discovery
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(d), d)).To(Succeed())
			d.Spec.DiscoveryInterval = &metav1.Duration{Duration: time.Hour * 48}
			Expect(k8sClient.Update(ctx, d)).To(Succeed())

			// Verify Role was reconciled back to correct roleRef
			Eventually(func() []string {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: roleName, Namespace: ns.Name}, role)).To(Succeed())
				return role.Rules[0].Verbs
			}).Should(ConsistOf("get", "list", "watch", "create", "update", "patch", "delete"))

			// Verify CRB was reconciled back to correct roleRef
			Eventually(func() []rbacv1.Subject {
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: rbName, Namespace: ns.Name}, rb)).To(Succeed())
				return rb.Subjects
			}).Should(ConsistOf(rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      discoveryPrefixed(d.Name),
				Namespace: ns.Name,
			}))

			// Verify pod still exists and was not duplicated
			podList := &corev1.PodList{}
			Eventually(func() int {
				Expect(k8sClient.List(ctx, podList, client.InNamespace(ns.Name), client.MatchingLabels{"app.kubernetes.io/name": discoveryPrefixed(d.Name)})).To(Succeed())
				return len(podList.Items)
			}).Should(Equal(1))
		})
	})
})
