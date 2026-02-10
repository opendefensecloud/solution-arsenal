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

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.opendefense.cloud/solar/api/solar/v1alpha1"
	. "go.opendefense.cloud/solar/pkg/discovery"
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
		registryProvider *RegistryProvider
		inputEventsChan  chan RepositoryEvent
		outputEventsChan chan ComponentVersionEvent
		errChan          chan ErrorEvent
		testRegistry     *Registry
		testServer       *httptest.Server
	)
	qualifierOptions := []Option{WithLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))}

	BeforeAll(func() {
		reg := registry.New()
		registryProvider = NewRegistryProvider()
		testServer = httptest.NewServer(reg.HandleFunc())
		scheme := runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).Should(Succeed())

		testServerUrl, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		testRegistry = &Registry{
			Name:      "test-registry",
			Hostname:  testServerUrl.Host,
			PlainHTTP: true,
		}

		Expect(registryProvider.Register(testRegistry)).To(Succeed())

		_, err = test.Run(exec.Command(
			"./bin/ocm", "transfer", "ctf", "./test/fixtures/helmdemo-ctf", fmt.Sprintf("%s/test", testRegistry.GetURL()),
		))
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		registryProvider = NewRegistryProvider()
		if err := registryProvider.Register(testRegistry); err != nil {
			panic(err)
		}

		inputEventsChan = make(chan RepositoryEvent, 100)
		outputEventsChan = make(chan ComponentVersionEvent, 100)
		errChan = make(chan ErrorEvent, 100)
	})

	AfterEach(func() {
		qualifier.Stop()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("Start and Stop", Label("qualifier"), func() {
		It("should start and stop the qualifier gracefully", func() {
			qualifier = NewQualifier(registryProvider, inputEventsChan, outputEventsChan, errChan, qualifierOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := qualifier.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Should be able to stop without blocking
			qualifier.Stop()
		})
	})

	Describe("Qualifier discovering ocm components", Label("qualifier"), func() {
		It("should process events", func() {
			qualifier = NewQualifier(registryProvider, inputEventsChan, outputEventsChan, errChan, qualifierOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := qualifier.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer qualifier.Stop()

			// Send event
			inputEventsChan <- RepositoryEvent{
				Registry:   testRegistry.Name,
				Repository: "test/google-containers/echoserver",
			}
			inputEventsChan <- RepositoryEvent{
				Registry:   testRegistry.Name,
				Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
			}
			inputEventsChan <- RepositoryEvent{
				Registry:   testRegistry.Name,
				Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
				Version:    "0.12.0",
			}

			select {
			case errEvent := <-errChan:
				Expect(errEvent.Error).ToNot(HaveOccurred())
			case ev := <-outputEventsChan:
				Expect(ev.Component).To(Equal("ocm.software/toi/demo/helmdemo"))
				Expect(ev.Source.Version).To(Equal("0.12.0"))
			case <-time.After(5 * time.Second):
				Fail("timeout waiting for event")
			}
		})
	})

})
