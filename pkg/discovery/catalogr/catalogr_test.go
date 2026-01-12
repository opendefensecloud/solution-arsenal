// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package catalogr

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/test"
	"go.opendefense.cloud/solar/test/registry"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestCatalogr(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Catalogr Suite")
}

var _ = Describe("Catalogr", Ordered, func() {
	var (
		catalogr    *Catalogr
		eventsChan  chan discovery.RegistryEvent
		registryURL string
		testServer  *httptest.Server
	)
	catalogrOptions := []Option{WithLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))}

	BeforeAll(func() {
		reg := registry.New()
		testServer = httptest.NewServer(reg.HandleFunc())

		testServerUrl, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		registryURL = testServerUrl.Host

		_, err = test.Run(exec.Command("ocm", "transfer", "ctf", "./test/fixtures/helmdemo-ctf", fmt.Sprintf("http://%s/test", registryURL)))
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		eventsChan = make(chan discovery.RegistryEvent, 100)
	})

	AfterEach(func() {
		catalogr.Stop()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("Start and Stop", func() {
		It("should start and stop the catalogr gracefully", func() {
			catalogr = NewCatalogr(testclient.NewSimpleClientset(), eventsChan, catalogrOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := catalogr.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Should be able to stop without blocking
			catalogr.Stop()
		})
	})

	Describe("Catalogr discovering ocm components", Label("catalogr"), func() {
		It("should process events", func() {
			catalogr = NewCatalogr(testclient.NewSimpleClientset(), eventsChan, catalogrOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := catalogr.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Send event
			eventsChan <- discovery.RegistryEvent{
				Registry:   registryURL,
				Repository: "test/google-containers/echoserver",
				Schema:     "http",
				Tag:        "1.10",
			}
			eventsChan <- discovery.RegistryEvent{
				Registry:   registryURL,
				Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
				Namespace:  "test",
				Schema:     "http",
				Component:  "ocm.software/toi/demo/helmdemo",
				Tag:        "0.12.0",
			}
			// Wait for processing
			time.Sleep(1 * time.Second)

			// Stop catalogr
			catalogr.Stop()
		})
	})

})
