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
					Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
					Version:    "0.12.0",
					Type:       discovery.EventCreated,
				},
				Namespace: "test",
				Component: "ocm.software/toi/demo/helmdemo",
			}

			expected := &discovery.WriteAPIResourceEvent{
				HelmDiscovery: discovery.HelmDiscovery{
					Name:    "echoserver",
					Version: "0.1.0",
				},
			}
			Eventually(outputChan).Should(Receive(expected))
			Consistently(errChan).ShouldNot(Receive())
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
				"./bin/ocm", "--config", "./test/fixtures/units/ocm-config.yaml", "transfer", "ctf", "./test/fixtures/helmdemo-ctf", fmt.Sprintf("%s/test", testRegistry.GetURL()),
			))
			Expect(err).NotTo(HaveOccurred())

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

			expected := &discovery.WriteAPIResourceEvent{
				ComponentSpec: compdesc.ComponentSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ocm.software/toi/demo/helmdemo",
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
})
