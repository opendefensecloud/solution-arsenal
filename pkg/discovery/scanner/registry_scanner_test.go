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

	"go.opendefense.cloud/solar/test"
	"go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.opendefense.cloud/solar/pkg/discovery"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestDiscovery(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Discovery Suite")
}

var _ = Describe("RegistryScanner", Ordered, func() {
	var (
		scanner     *RegistryScanner
		eventsChan  chan RepositoryEvent
		errChan     chan ErrorEvent
		registryURL string
		testServer  *httptest.Server
	)
	scannerOptions := []Option{WithPlainHTTP(), WithLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))}

	BeforeAll(func() {
		reg := registry.New()
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
			scanner = NewRegistryScanner(registryURL, eventsChan, errChan, scannerOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Should be able to stop without blocking
			scanner.Stop()
		})
	})

	Describe("Registry scanning", func() {
		It("should discover repositories and tags in the registry", func() {
			scanner = NewRegistryScanner(registryURL, eventsChan, errChan, scannerOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer scanner.Stop()

			// Read the event
			select {
			case receivedEvent := <-eventsChan:
				Expect(receivedEvent.Repository).To(ContainSubstring("test"))
				Expect(receivedEvent.Registry.Hostname).To(Equal(registryURL))
			case <-time.After(5 * time.Second):
				Fail("timeout waiting for event")
			}
		})
	})
})
