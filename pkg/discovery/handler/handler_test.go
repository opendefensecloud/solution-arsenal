// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/test"
	"go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", Ordered, func() {
	var (
		handler          *Handler
		registryProvider *discovery.RegistryProvider
		inputChan        chan discovery.ComponentVersionEvent
		outputChan       chan discovery.WriteAPIResourceEvent
		errChan          chan discovery.ErrorEvent
		testRegistry     *discovery.Registry
		testServer       *httptest.Server
	)
	opts := NewHandlerOptions(discovery.WithLogger[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))))

	BeforeAll(func() {
		reg := registry.New()
		registryProvider = discovery.NewRegistryProvider()
		testServer = httptest.NewServer(reg.HandleFunc())
		scheme := runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).Should(Succeed())

		testServerUrl, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		testRegistry = &discovery.Registry{
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
		inputChan = make(chan discovery.ComponentVersionEvent, 100)
		outputChan = make(chan discovery.WriteAPIResourceEvent, 100)
		errChan = make(chan discovery.ErrorEvent, 100)
	})

	AfterEach(func() {
		handler.Stop()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("Start and Stop", func() {
		It("should start and stop the handler gracefully", func() {
			handler = NewHandler(registryProvider, inputChan, outputChan, errChan, opts...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := handler.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Should be able to stop without blocking
			handler.Stop()
		})
	})

	Describe("ProcessEvent", func() {
		It("should process events without error", func() {
			handler = NewHandler(registryProvider, inputChan, outputChan, errChan, opts...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			Expect(handler.Start(ctx)).To(Succeed())

			inputChan <- discovery.ComponentVersionEvent{
				Source: discovery.RepositoryEvent{
					Registry:   testRegistry.Name,
					Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
					Version:    "0.12.0",
					Type:       discovery.EventCreated,
				},
				Namespace: "test",
				Component: "ocm.software/toi/demo/helmdemo",
			}

			select {
			case err := <-errChan:
				Fail("unexpected error event: " + err.Error.Error())
			case output := <-outputChan:
				Expect(output.HelmDiscovery.Name).To(Equal("echoserver"))
				Expect(output.HelmDiscovery.Version).To(Equal("0.1.0"))
			case <-time.After(2 * time.Second):
				Fail("timeout waiting for output event")
			}
		})
	})
})
