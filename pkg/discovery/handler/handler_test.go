// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"testing"
	"time"

	"ocm.software/ocm/api/ocm/compdesc"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "go.opendefense.cloud/solar/pkg/discovery"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRegistryProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handler Suite")
}

var _ = Describe("Handler", func() {
	var (
		handler    *Handler
		inputChan  chan ComponentVersionEvent
		outputChan chan ComponentVersionEvent
		errChan    chan ErrorEvent
	)
	opts := []Option{WithLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))}

	BeforeEach(func() {
		inputChan = make(chan ComponentVersionEvent, 100)
		outputChan = make(chan ComponentVersionEvent, 100)
		errChan = make(chan ErrorEvent, 100)
	})

	AfterEach(func() {
		handler.Stop()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("Start and Stop", func() {
		It("should start and stop the handler gracefully", func() {
			handler = NewHandler(inputChan, outputChan, errChan, opts...)

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
			handler = NewHandler(inputChan, outputChan, errChan, opts...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			Expect(handler.Start(ctx)).To(Succeed())

			desc := compdesc.New("test-component", "0.0.1")
			desc.Resources = []compdesc.Resource{
				{
					ResourceMeta: compdesc.ResourceMeta{
						Type: string(HelmResource),
					},
				},
			}
			testEvent := ComponentVersionEvent{
				Source:     RepositoryEvent{},
				Namespace:  "test",
				Component:  "test-component",
				Type:       EventCreated,
				Descriptor: desc,
			}
			inputChan <- testEvent

			select {
			case err := <-errChan:
				Fail("unexpected error event: " + err.Error.Error())
			case output := <-outputChan:
				Expect(output.ComponentVersion).NotTo(BeNil())
				Expect(output.ComponentVersion.Name).To(Equal("0.0.1"))
			case <-time.After(2 * time.Second):
				Fail("timeout waiting for output event")
			}
		})
	})
})
