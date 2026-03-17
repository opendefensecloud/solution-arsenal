// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.opendefense.cloud/solar/pkg/discovery"
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
		eventsChan   chan discovery.RepositoryEvent
		errChan      chan discovery.ErrorEvent
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

		_, err = test.Run(exec.Command("./bin/ocm", "transfer", "ctf", "./test/fixtures/helmdemo-ctf", fmt.Sprintf("http://%s/test", registryHost)))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		testServer.Close()
	})

	BeforeEach(func() {
		eventsChan = make(chan discovery.RepositoryEvent, 100)
		errChan = make(chan discovery.ErrorEvent, 100)
	})

	AfterEach(func() {
		close(eventsChan)
		close(errChan)
	})

	Describe("Start and Stop", func() {
		It("should start and stop the scanner gracefully", func() {
			testReg := &discovery.Registry{
				Hostname:  registryHost,
				PlainHTTP: true,
			}
			scanner := NewRegistryScanner(testReg, eventsChan, errChan, scannerOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Should be able to stop without blocking
			scanner.Stop()
		})
	})

	Describe("Registries scanning", func() {
		var (
			ctx    context.Context
			cancel context.CancelFunc
		)
		BeforeEach(func() {
			ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		})

		AfterEach(func() {
			cancel()
		})

		It("should discover repositories and tags in the registry", func() {
			testReg := &discovery.Registry{
				Name:      "test-registry",
				Hostname:  registryHost,
				PlainHTTP: true,
			}
			scanner := NewRegistryScanner(testReg, eventsChan, errChan, scannerOptions...)

			err := scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer scanner.Stop()

			// Read the event
			expected := &discovery.RepositoryEvent{
				Registry:   testReg.Name,
				Repository: "test",
			}
			Eventually(eventsChan).Should(Receive(expected))
			Consistently(errChan).ShouldNot(Receive())
		})

		It("should access the registry with basic auth", func() {
			regWAuth := registry.New().WithAuth("usr", "psswrd")
			testServerWAuth := httptest.NewServer(regWAuth.HandleFunc())
			defer testServerWAuth.Close()

			testServerWAuthUrl, err := url.Parse(testServerWAuth.URL)
			Expect(err).NotTo(HaveOccurred())

			_, err = test.Run(exec.Command("./bin/ocm", "--config", "./test/fixtures/units/ocm-config.yaml", "transfer", "ctf", "./test/fixtures/helmdemo-ctf", fmt.Sprintf("http://%s/test", testServerWAuthUrl.Host)))
			Expect(err).NotTo(HaveOccurred())

			testRegWAuth := &discovery.Registry{
				Name:      "test-registry-wAuth",
				Hostname:  testServerWAuthUrl.Host,
				PlainHTTP: true,
				Credentials: &discovery.RegistryCredentials{
					Username: "usr",
					Password: "psswrd",
				},
			}

			scanner := NewRegistryScanner(testRegWAuth, eventsChan, errChan, scannerOptions...)

			err = scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer scanner.Stop()

			expected := &discovery.RepositoryEvent{
				Registry: testRegWAuth.Name,
			}
			Eventually(eventsChan).Should(Receive(expected))
			Consistently(errChan).ShouldNot(Receive())
		})
	})
})
