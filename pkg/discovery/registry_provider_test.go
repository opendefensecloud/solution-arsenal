// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRegistryProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RegistryProvider Suite")
}

var _ = Describe("RegistryProvider", func() {

	var provider *RegistryProvider

	BeforeEach(func() {
		provider = NewRegistryProvider()
	})

	Describe("Register", func() {
		It("registers a registry successfully", func() {
			reg := &Registry{
				Name:     "test",
				Flavor:   "zot",
				Hostname: "registry.example.com",
			}

			err := provider.Register(reg)
			Expect(err).NotTo(HaveOccurred())

			stored := provider.Get("test")
			Expect(stored).NotTo(BeNil())
			Expect(stored.Name).To(Equal("test"))
		})

		It("fails when registering a registry with a duplicate name", func() {
			reg := &Registry{Name: "duplicate"}

			Expect(provider.Register(reg)).To(Succeed())
			err := provider.Register(reg)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already registered"))
		})
	})

	Describe("Get", func() {
		It("returns nil if the registry does not exist", func() {
			Expect(provider.Get("unknown")).To(BeNil())
		})

		It("returns the correct registry", func() {
			reg := &Registry{Name: "existing"}
			Expect(provider.Register(reg)).To(Succeed())

			result := provider.Get("existing")
			Expect(result).NotTo(BeNil())
			Expect(result.Name).To(Equal("existing"))
		})
	})

	Describe("GetAll", func() {
		It("returns all registered registries", func() {
			Expect(provider.Register(
				&Registry{Name: "one"},
				&Registry{Name: "two"},
			)).To(Succeed())

			all := provider.GetAll()
			Expect(all).To(HaveLen(2))
			Expect(all).To(ContainElements(
				HaveField("Name", "one"),
				HaveField("Name", "two"),
			))
		})
	})

	Describe("FromYaml", func() {
		var tmpFile *os.File

		BeforeEach(func() {
			var err error
			tmpFile, err = os.CreateTemp("", "registries-*.yaml")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.Remove(tmpFile.Name())).To(Succeed())
		})

		It("loads registries from a YAML file", func() {
			yamlContent := `
registries:
  - name: test-registry
    flavor: zot
    hostname: registry.example.com
    plainHTTP: true
    scanInterval: 24h
    credentials:
      username: admin
      password: secret
`
			_, err := tmpFile.WriteString(yamlContent)
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpFile.Close()).To(Succeed())

			err = provider.Unmarshall(tmpFile.Name())
			Expect(err).NotTo(HaveOccurred())

			reg := provider.Get("test-registry")
			Expect(reg).NotTo(BeNil())
			Expect(reg.Flavor).To(Equal("zot"))
			Expect(reg.PlainHTTP).To(BeTrue())
			Expect(reg.ScanInterval).To(Equal(24 * time.Hour))
			Expect(reg.Credentials.Username).To(Equal("admin"))
		})

		It("fails if the YAML file does not exist", func() {
			err := provider.Unmarshall("/does/not/exist.yaml")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Concurrency", func() {
		It("supports concurrent Register and Get operations", func() {
			const count = 50
			wg := sync.WaitGroup{}

			for i := range count {
				wg.Go(func() {
					reg := &Registry{
						Name:     fmt.Sprintf("reg-%d", i),
						Flavor:   "zot",
						Hostname: "example.com",
					}

					// Register may fail only if duplicate, which should not happen here
					err := provider.Register(reg)
					Expect(err).NotTo(HaveOccurred())

					// Immediate read after write
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
