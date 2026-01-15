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

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/test"
	"go.opendefense.cloud/solar/test/registry"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestCatalogr(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Catalogr Suite")
}

var _ = Describe("Catalogr", Ordered, func() {
	var (
		catalogr    *Catalogr
		eventsChan  chan discovery.RepositoryEvent
		registryURL string
		testServer  *httptest.Server
		fakeClient  client.Client
	)
	catalogrOptions := []Option{WithLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))}

	BeforeAll(func() {
		reg := registry.New()
		testServer = httptest.NewServer(reg.HandleFunc())
		scheme := runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).Should(Succeed())
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		testServerUrl, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		registryURL = testServerUrl.Host

		_, err = test.Run(exec.Command("ocm", "transfer", "ctf", "./test/fixtures/helmdemo-ctf", fmt.Sprintf("http://%s/test", registryURL)))
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		eventsChan = make(chan discovery.RepositoryEvent, 100)
	})

	AfterEach(func() {
		catalogr.Stop()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("Start and Stop", func() {
		It("should start and stop the catalogr gracefully", func() {
			catalogr = NewCatalogr(fakeClient, eventsChan, catalogrOptions...)

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
			catalogr = NewCatalogr(fakeClient, eventsChan, catalogrOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := catalogr.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Send event
			// eventsChan <- discovery.RegistryEvent{
			// 	Registry:   registryURL,
			// 	Repository: "test/google-containers/echoserver",
			// 	Schema:     "http",
			// 	Tag:        "1.10",
			// }
			// eventsChan <- discovery.RegistryEvent{
			// 	Registry:   registryURL,
			// 	Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
			// 	Namespace:  "test",
			// 	Schema:     "http",
			// 	Component:  "ocm.software/toi/demo/helmdemo",
			// 	Tag:        "0.12.0",
			// }
			eventsChan <- discovery.RepositoryEvent{
				Registry:   "ghcr.io",
				Repository: "opendefensecloud/component-descriptors/opendefense.cloud/arc",
				Schema:     "https",
			}

			// Wait for processing
			time.Sleep(1 * time.Second)

			// Stop catalogr
			catalogr.Stop()
		})
	})

})
