// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/client-go/clientset/versioned/fake"
	solarv1alpha1 "go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filter", Ordered, func() {
	var (
		filter      *Filter
		inputChan   chan discovery.ComponentVersionEvent
		outputChan  chan discovery.ComponentVersionEvent
		errChan     chan discovery.ErrorEvent
		solarClient solarv1alpha1.SolarV1alpha1Interface
		ctx         context.Context
		cancel      context.CancelFunc
	)
	opts := NewFilterOptions(discovery.WithLogger[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent](zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))))

	BeforeEach(func() {
		inputChan = make(chan discovery.ComponentVersionEvent, 100)
		outputChan = make(chan discovery.ComponentVersionEvent, 100)
		errChan = make(chan discovery.ErrorEvent, 100)
		solarClient = fake.NewClientset(&v1alpha1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{Name: discovery.SanitizeWithHash("ocm-software-toi-demo-helmdemo-0-12-0"), Namespace: "default"},
		}).SolarV1alpha1()

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)

		filter = NewFilter(solarClient, "default", inputChan, outputChan, errChan, opts...)
		err := filter.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		filter.Stop()
		cancel()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("Filter filtering events", Label("filter"), func() {
		It("should skip create events for already existing component versions", func() {
			// Send create event for a component version that already exists in the cluster
			inputChan <- discovery.ComponentVersionEvent{
				Source: discovery.RepositoryEvent{
					Registry:   "default",
					Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
					Version:    "0.12.0",
					Type:       discovery.EventCreated,
				},
				Namespace: "test",
				Component: "ocm.software/toi/demo/helmdemo",
			}

			Consistently(outputChan, 2*time.Second).ShouldNot(Receive())
			Consistently(errChan).ShouldNot(Receive())
		})

		It("should forward create events for non-existing component versions", func() {
			// Send create event for a component version that does NOT exist in the cluster
			inputChan <- discovery.ComponentVersionEvent{
				Source: discovery.RepositoryEvent{
					Registry:   "default",
					Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
					Version:    "0.99.0",
					Type:       discovery.EventCreated,
				},
				Namespace: "test",
				Component: "ocm.software/toi/demo/helmdemo",
			}

			var ev discovery.ComponentVersionEvent
			Eventually(outputChan).Should(Receive(&ev))
			Expect(ev.Component).To(Equal("ocm.software/toi/demo/helmdemo"))
			Expect(ev.Source.Version).To(Equal("0.99.0"))
			Consistently(errChan).ShouldNot(Receive())
		})

		It("should pass through update events without filtering", func() {
			// Send update event for a component version that already exists — should still pass through
			inputChan <- discovery.ComponentVersionEvent{
				Source: discovery.RepositoryEvent{
					Registry:   "default",
					Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
					Version:    "0.12.0",
					Type:       discovery.EventUpdated,
				},
				Namespace: "test",
				Component: "ocm.software/toi/demo/helmdemo",
			}

			var ev discovery.ComponentVersionEvent
			Eventually(outputChan).Should(Receive(&ev))
			Expect(ev.Component).To(Equal("ocm.software/toi/demo/helmdemo"))
			Expect(ev.Source.Version).To(Equal("0.12.0"))
			Expect(ev.Source.Type).To(Equal(discovery.EventUpdated))
			Consistently(errChan).ShouldNot(Receive())
		})

		It("should pass through delete events without filtering", func() {
			// Send delete event — should always pass through regardless of existence
			inputChan <- discovery.ComponentVersionEvent{
				Source: discovery.RepositoryEvent{
					Registry:   "default",
					Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
					Version:    "0.12.0",
					Type:       discovery.EventDeleted,
				},
				Namespace: "test",
				Component: "ocm.software/toi/demo/helmdemo",
			}

			var ev discovery.ComponentVersionEvent
			Eventually(outputChan).Should(Receive(&ev))
			Expect(ev.Component).To(Equal("ocm.software/toi/demo/helmdemo"))
			Expect(ev.Source.Type).To(Equal(discovery.EventDeleted))
			Consistently(errChan).ShouldNot(Receive())
		})
	})

})
