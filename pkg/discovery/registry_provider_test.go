// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"fmt"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solarfake "go.opendefense.cloud/solar/client-go/clientset/versioned/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRegistryProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RegistryProvider Suite")
}

func newTestRegistry(name, hostname string) *solarv1alpha1.Registry {
	return &solarv1alpha1.Registry{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: solarv1alpha1.RegistrySpec{
			Hostname: hostname,
			Flavor:   "zot",
		},
	}
}

var _ = Describe("RegistryProvider", func() {

	var provider *RegistryProvider

	BeforeEach(func() {
		provider = NewRegistryProvider()
	})

	Describe("Register", func() {
		It("registers a registry successfully", func() {
			reg := newTestRegistry("test", "registry.example.com")

			err := provider.Register(reg, nil)
			Expect(err).NotTo(HaveOccurred())

			stored := provider.Get("test")
			Expect(stored).NotTo(BeNil())
			Expect(stored.Name).To(Equal("test"))
		})

		It("registers a registry with credentials", func() {
			reg := newTestRegistry("test-creds", "registry.example.com")
			creds := &RegistryCredentials{Username: "user", Password: "pass"}

			Expect(provider.Register(reg, creds)).To(Succeed())

			stored := provider.GetCredentials("test-creds")
			Expect(stored).NotTo(BeNil())
			Expect(stored.Username).To(Equal("user"))
		})

		It("fails when registering a registry with a duplicate name", func() {
			reg := newTestRegistry("duplicate", "example.com")

			Expect(provider.Register(reg, nil)).To(Succeed())
			err := provider.Register(reg, nil)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already registered"))
		})
	})

	Describe("Get", func() {
		It("returns nil if the registry does not exist", func() {
			Expect(provider.Get("unknown")).To(BeNil())
		})

		It("returns the correct registry", func() {
			reg := newTestRegistry("existing", "example.com")
			Expect(provider.Register(reg, nil)).To(Succeed())

			result := provider.Get("existing")
			Expect(result).NotTo(BeNil())
			Expect(result.Name).To(Equal("existing"))
		})
	})

	Describe("GetCredentials", func() {
		It("returns nil when no credentials were registered", func() {
			reg := newTestRegistry("no-creds", "example.com")
			Expect(provider.Register(reg, nil)).To(Succeed())

			Expect(provider.GetCredentials("no-creds")).To(BeNil())
		})

		It("returns nil for an unknown registry", func() {
			Expect(provider.GetCredentials("unknown")).To(BeNil())
		})
	})

	Describe("GetAll", func() {
		It("returns all registered registries", func() {
			Expect(provider.Register(newTestRegistry("one", "one.example.com"), nil)).To(Succeed())
			Expect(provider.Register(newTestRegistry("two", "two.example.com"), nil)).To(Succeed())

			all := provider.GetAll()
			Expect(all).To(HaveLen(2))
			Expect(all).To(ContainElements(
				HaveField("Name", "one"),
				HaveField("Name", "two"),
			))
		})
	})

	Describe("Concurrency", func() {
		It("supports concurrent Register and Get operations", func() {
			const count = 50
			wg := sync.WaitGroup{}

			for i := range count {
				wg.Go(func() {
					reg := newTestRegistry(fmt.Sprintf("reg-%d", i), "example.com")

					err := provider.Register(reg, nil)
					Expect(err).NotTo(HaveOccurred())

					got := provider.Get(reg.Name)
					Expect(got).NotTo(BeNil())
					Expect(got.Name).To(Equal(reg.Name))
				})
			}

			wg.Wait()

			all := provider.GetAll()
			Expect(all).To(HaveLen(count))
		})
	})

	Describe("LoadFromAPI", func() {
		const ns = "test-ns"

		newRegistryWithSecret := func(name, secretName string) *solarv1alpha1.Registry {
			reg := newTestRegistry(name, "registry.example.com")
			reg.Namespace = ns
			reg.Spec.SolarSecretRef = &corev1.LocalObjectReference{Name: secretName}

			return reg
		}

		newSecret := func(name string, data map[string][]byte) *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Data:       data,
			}
		}

		It("loads a registry with valid credentials", func() {
			reg := newRegistryWithSecret("my-reg", "my-secret")
			secret := newSecret("my-secret", map[string][]byte{
				SecretKeyUsername: []byte("admin"),
				SecretKeyPassword: []byte("hunter2"),
			})

			solarClient := solarfake.NewSimpleClientset(reg)
			k8sClient := k8sfake.NewSimpleClientset(secret)

			err := provider.LoadFromAPI(context.Background(), solarClient.SolarV1alpha1(), k8sClient.CoreV1(), ns)
			Expect(err).NotTo(HaveOccurred())

			creds := provider.GetCredentials("my-reg")
			Expect(creds).NotTo(BeNil())
			Expect(creds.Username).To(Equal("admin"))
			Expect(creds.Password).To(Equal("hunter2"))
		})

		It("loads a registry without a secret ref and leaves credentials unset", func() {
			reg := newTestRegistry("no-secret-reg", "registry.example.com")
			reg.Namespace = ns

			solarClient := solarfake.NewSimpleClientset(reg)
			k8sClient := k8sfake.NewSimpleClientset()

			err := provider.LoadFromAPI(context.Background(), solarClient.SolarV1alpha1(), k8sClient.CoreV1(), ns)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Get("no-secret-reg")).NotTo(BeNil())
			Expect(provider.GetCredentials("no-secret-reg")).To(BeNil())
		})

		It("returns an error when the username key is missing from the secret", func() {
			reg := newRegistryWithSecret("bad-user-reg", "bad-secret")
			secret := newSecret("bad-secret", map[string][]byte{
				SecretKeyPassword: []byte("hunter2"),
				// SecretKeyUsername intentionally absent
			})

			solarClient := solarfake.NewSimpleClientset(reg)
			k8sClient := k8sfake.NewSimpleClientset(secret)

			err := provider.LoadFromAPI(context.Background(), solarClient.SolarV1alpha1(), k8sClient.CoreV1(), ns)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(SecretKeyUsername))
			Expect(err.Error()).To(ContainSubstring("bad-secret"))
		})

		It("returns an error when the password key is missing from the secret", func() {
			reg := newRegistryWithSecret("bad-pass-reg", "bad-secret")
			secret := newSecret("bad-secret", map[string][]byte{
				SecretKeyUsername: []byte("admin"),
				// SecretKeyPassword intentionally absent
			})

			solarClient := solarfake.NewSimpleClientset(reg)
			k8sClient := k8sfake.NewSimpleClientset(secret)

			err := provider.LoadFromAPI(context.Background(), solarClient.SolarV1alpha1(), k8sClient.CoreV1(), ns)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(SecretKeyPassword))
			Expect(err.Error()).To(ContainSubstring("bad-secret"))
		})

		It("returns an error when the referenced secret does not exist", func() {
			reg := newRegistryWithSecret("missing-secret-reg", "ghost-secret")

			solarClient := solarfake.NewSimpleClientset(reg)
			k8sClient := k8sfake.NewSimpleClientset() // secret not added

			err := provider.LoadFromAPI(context.Background(), solarClient.SolarV1alpha1(), k8sClient.CoreV1(), ns)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ghost-secret"))
		})

		It("replaces previously loaded registries on subsequent calls", func() {
			reg := newRegistryWithSecret("reload-reg", "reload-secret")
			secret := newSecret("reload-secret", map[string][]byte{
				SecretKeyUsername: []byte("user1"),
				SecretKeyPassword: []byte("pass1"),
			})

			solarClient := solarfake.NewSimpleClientset(reg)
			k8sClient := k8sfake.NewSimpleClientset(secret)

			Expect(provider.LoadFromAPI(context.Background(), solarClient.SolarV1alpha1(), k8sClient.CoreV1(), ns)).To(Succeed())
			Expect(provider.GetCredentials("reload-reg").Username).To(Equal("user1"))

			// Second load with different credentials
			secret.Data[SecretKeyUsername] = []byte("user2")
			secret.Data[SecretKeyPassword] = []byte("pass2")
			_, err := k8sClient.CoreV1().Secrets(ns).Update(context.Background(), secret, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.LoadFromAPI(context.Background(), solarClient.SolarV1alpha1(), k8sClient.CoreV1(), ns)).To(Succeed())
			Expect(provider.GetCredentials("reload-reg").Username).To(Equal("user2"))
		})
	})
})
