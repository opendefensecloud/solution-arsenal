// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package qualifier

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

func TestQualifier(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Qualifier Suite")
}

var _ = Describe("Qualifier", Ordered, func() {
	var (
		qualifier        *Qualifier
		registryProvider *discovery.RegistryProvider
		inputEventsChan  chan discovery.RepositoryEvent
		outputEventsChan chan discovery.ComponentVersionEvent
		errChan          chan discovery.ErrorEvent
		testRegistry     *discovery.Registry
		testServer       *httptest.Server
	)
	qualifierOptions := NewQualifierOptions(discovery.WithLogger[discovery.RepositoryEvent, discovery.ComponentVersionEvent](zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))))

	BeforeAll(func() {
		reg := registry.New()
		registryProvider = discovery.NewRegistryProvider()
		testServer = httptest.NewServer(reg.HandleFunc())

		testServerUrl, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		testRegistry = &discovery.Registry{
			Name:      "test-registry",
			Hostname:  testServerUrl.Host,
			PlainHTTP: true,
		}

		Expect(registryProvider.Register(testRegistry)).To(Succeed())

		_, err = test.Run(exec.Command(
			"./bin/ocm", "transfer", "ctf", "./test/fixtures/ocm-demo-ctf", fmt.Sprintf("%s/test", testRegistry.GetURL()),
		))
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		inputEventsChan = make(chan discovery.RepositoryEvent, 100)
		outputEventsChan = make(chan discovery.ComponentVersionEvent, 100)
		errChan = make(chan discovery.ErrorEvent, 100)
	})

	AfterEach(func() {
		close(inputEventsChan)
		close(outputEventsChan)
		close(errChan)
	})

	AfterAll(func() {
		testServer.Close()
	})

	Describe("Start and Stop", Label("qualifier"), func() {
		It("should start and stop the qualifier gracefully", func() {
			qualifier = NewQualifier(registryProvider, "default", inputEventsChan, outputEventsChan, errChan, qualifierOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := qualifier.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Should be able to stop without blocking
			qualifier.Stop()
		})
	})

	Describe("Qualifier discovering ocm components", Label("qualifier"), func() {
		var (
			ctx    context.Context
			cancel context.CancelFunc
		)
		BeforeEach(func() {
			ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)

			qualifier = NewQualifier(registryProvider, "default", inputEventsChan, outputEventsChan, errChan, qualifierOptions...)
			Expect(qualifier.Start(ctx)).To(Succeed())
		})

		AfterEach(func() {
			qualifier.Stop()
			cancel()
		})

		It("should process events", func() {
			// Send event
			inputEventsChan <- discovery.RepositoryEvent{
				Registry:   testRegistry.Name,
				Repository: "test/google-containers/echoserver",
			}
			inputEventsChan <- discovery.RepositoryEvent{
				Registry:   testRegistry.Name,
				Repository: "test/component-descriptors/opendefense.cloud/ocm-demo",
			}
			inputEventsChan <- discovery.RepositoryEvent{
				Registry:   testRegistry.Name,
				Repository: "test/component-descriptors/opendefense.cloud/ocm-demo",
				Version:    "v26.4.0",
			}

			expected := &discovery.ComponentVersionEvent{
				Component: "opendefense.cloud/ocm-demo",
				Source:    discovery.RepositoryEvent{Version: "v26.4.0"},
			}
			Eventually(outputEventsChan).Should(Receive(expected))
			Eventually(outputEventsChan).Should(Receive(expected))
			Consistently(errChan).ShouldNot(Receive())
		})

		It("should support basic auth", func() {
			regWAuth := registry.New().WithAuth("usr", "psswrd")
			testServerWAuth := httptest.NewServer(regWAuth.HandleFunc())
			defer testServerWAuth.Close()

			testServerWAuthUrl, err := url.Parse(testServerWAuth.URL)
			Expect(err).NotTo(HaveOccurred())

			testRegistryWAuth := &discovery.Registry{
				Name:      "test-registry-wAuth",
				Hostname:  testServerWAuthUrl.Host,
				PlainHTTP: true,
				Credentials: &discovery.RegistryCredentials{
					Username: "usr",
					Password: "psswrd",
				},
			}

			Expect(registryProvider.Register(testRegistryWAuth)).To(Succeed())

			_, err = test.Run(exec.Command(
				"./bin/ocm", "--config", "./test/fixtures/units/ocm-config.yaml", "transfer", "ctf", "./test/fixtures/ocm-demo-ctf", fmt.Sprintf("%s/test", testRegistry.GetURL()),
			))
			Expect(err).NotTo(HaveOccurred())

			// Send event that requires requesting the registry to verify basic auth support
			inputEventsChan <- discovery.RepositoryEvent{
				Registry:   testRegistry.Name,
				Repository: "test/component-descriptors/opendefense.cloud/ocm-demo",
			}
			expected := &discovery.ComponentVersionEvent{
				Component: "opendefense.cloud/ocm-demo",
				Source:    discovery.RepositoryEvent{Version: "v26.4.0"},
			}
			Eventually(outputEventsChan).Should(Receive(expected))
			Consistently(errChan).ShouldNot(Receive())
		})

	})

})
