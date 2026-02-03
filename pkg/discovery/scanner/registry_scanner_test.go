// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/test"
	"go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDiscovery(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Discovery Suite")
}

var _ = Describe("RegistryScanner", Ordered, func() {
	var (
		scanner      *RegistryScanner
		eventsChan   chan RepositoryEvent
		errChan      chan ErrorEvent
		registryHost string
		testServer   *httptest.Server
	)
	scannerOptions := []Option{WithLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))}

	BeforeAll(func() {
		reg := registry.New()
		testServer = httptest.NewServer(reg.HandleFunc())

		testServerUrl, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		registryHost = testServerUrl.Host

		_, err = test.Run(exec.Command(
			"./bin/ocm",
			"transfer",
			"ctf",
			"./test/fixtures/helmdemo-ctf",
			fmt.Sprintf("http://%s/test", registryHost),
		))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		testServer.Close()
	})

	BeforeEach(func() {
		eventsChan = make(chan RepositoryEvent, 100)
		errChan = make(chan ErrorEvent, 100)
	})

	AfterEach(func() {
		scanner.Stop()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("Start and Stop", func() {
		It("should start and stop the scanner gracefully", func() {
			reg := Registry{
				Hostname:  registryHost,
				PlainHTTP: true,
			}
			scanner = NewRegistryScanner(reg, eventsChan, errChan, scannerOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Should be able to stop without blocking
			scanner.Stop()
		})
	})

	Describe("Registries scanning", func() {
		It("should discover repositories and tags in the registry", func() {
			reg := Registry{
				Name:      "test-registry",
				Hostname:  registryHost,
				PlainHTTP: true,
			}

			scanner = NewRegistryScanner(reg, eventsChan, errChan, scannerOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer scanner.Stop()

			// Read the event
			select {
			case receivedEvent := <-eventsChan:
				Expect(receivedEvent.Repository).To(ContainSubstring("test"))
				Expect(receivedEvent.Registry).To(Equal(reg.Name))
			case <-time.After(5 * time.Second):
				Fail("timeout waiting for event")
			}
		})
	})
})
