// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"fmt"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

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
})
