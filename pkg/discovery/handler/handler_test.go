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
	"ocm.software/ocm/api/ocm/compdesc"
	metav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
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
			"./bin/ocm", "transfer", "ctf", "./test/fixtures/ocm-demo-ctf", fmt.Sprintf("%s/test", testRegistry.GetURL()),
		))
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		inputChan = make(chan discovery.ComponentVersionEvent, 100)
		outputChan = make(chan discovery.WriteAPIResourceEvent, 100)
		errChan = make(chan discovery.ErrorEvent, 100)
	})

	AfterEach(func() {
		close(inputChan)
		close(outputChan)
		close(errChan)
	})

	AfterAll(func() {
		testServer.Close()
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
		var (
			ctx    context.Context
			cancel context.CancelFunc
		)

		BeforeEach(func() {
			ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)

			handler = NewHandler(registryProvider, inputChan, outputChan, errChan, opts...)
			Expect(handler.Start(ctx)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			handler.Stop()
			cancel()
		})

		It("should process events without error", func() {
			inputChan <- discovery.ComponentVersionEvent{
				Source: discovery.RepositoryEvent{
					Registry:   testRegistry.Name,
					Repository: "test/component-descriptors/opendefense.cloud/ocm-demo",
					Version:    "v26.4.1",
					Type:       discovery.EventCreated,
				},
				Namespace: "test",
				Component: "opendefense.cloud/ocm-demo",
			}

			var ev discovery.WriteAPIResourceEvent
			Eventually(outputChan).Should(Receive(&ev))
			Consistently(errChan).ShouldNot(Receive())

			Expect(ev.HelmDiscovery.Name).To(Equal("demo"))
			Expect(ev.HelmDiscovery.Version).To(Equal("0.1.0"))
			Expect(ev.HelmDiscovery.ResourceName).To(Equal("demo-chart"))

			// The ocm-demo CTF contains a helm-values-template resource
			// that renders nginx image references into helm values.
			Expect(ev.HelmDiscovery.ValuesTemplate).NotTo(BeNil())
			GinkgoWriter.Printf("ValuesTemplate content:\n%s\n", *ev.HelmDiscovery.ValuesTemplate)
			Expect(*ev.HelmDiscovery.ValuesTemplate).To(ContainSubstring("image:"))
			// Verify the rendered template contains actual image data, not empty placeholders
			Expect(*ev.HelmDiscovery.ValuesTemplate).To(ContainSubstring("nginx"))
			Expect(*ev.HelmDiscovery.ValuesTemplate).NotTo(ContainSubstring("repository: /\n"))
		})

		It("should support basic auth", func() {
			regWAuth := registry.New().WithAuth("", "")
			testServerWAuth := httptest.NewServer(regWAuth.HandleFunc())

			testServerUrlWAuth, err := url.Parse(testServerWAuth.URL)
			Expect(err).NotTo(HaveOccurred())

			testRegistryWAuth := &discovery.Registry{
				Name:      "test-registry-wAuth",
				Hostname:  testServerUrlWAuth.Host,
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

			inputChan <- discovery.ComponentVersionEvent{
				Source: discovery.RepositoryEvent{
					Registry:   testRegistry.Name,
					Repository: "test/component-descriptors/opendefense.cloud/ocm-demo",
					Version:    "v26.4.1",
					Type:       discovery.EventCreated,
				},
				Namespace: "test",
				Component: "opendefense.cloud/ocm-demo",
			}

			expected := &discovery.WriteAPIResourceEvent{
				ComponentSpec: compdesc.ComponentSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name: "opendefense.cloud/ocm-demo",
					},
				},
				HelmDiscovery: discovery.HelmDiscovery{
					Name:    "echoserver",
					Version: "0.1.0",
				},
			}
			Eventually(outputChan).Should(Receive(expected))
			Consistently(errChan).ShouldNot(Receive())
		})
	})

	Describe("GetHandlerForType", func() {
		It("should return registered handlers", func() {
			handler = NewHandler(registryProvider, inputChan, outputChan, errChan, opts...)
			// expect the handler to be initialized and returned
			h, err := handler.getHandlerForType(HelmHandler)
			Expect(err).ToNot(HaveOccurred())
			Expect(h).ToNot(BeNil())
			// expect the already initialized handler to be returned
			h, err = handler.getHandlerForType(HelmHandler)
			Expect(err).ToNot(HaveOccurred())

			// Kro not yet supported
			Expect(h).ToNot(BeNil())
			_, err = handler.getHandlerForType(KroHandler)
			Expect(err).To(HaveOccurred())

		})
	})
})
