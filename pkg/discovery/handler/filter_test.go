// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"
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
	)
	opts := NewFilterOptions(discovery.WithLogger[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent](zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))))

	BeforeEach(func() {
		inputChan = make(chan discovery.ComponentVersionEvent, 100)
		outputChan = make(chan discovery.ComponentVersionEvent, 100)
		errChan = make(chan discovery.ErrorEvent, 100)
		solarClient = fake.NewClientset(&v1alpha1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{Name: discovery.SanitizeWithHash("ocm-software-toi-demo-helmdemo-0-12-0"), Namespace: "default"},
		}).SolarV1alpha1()
	})

	AfterEach(func() {
		filter.Stop()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("Filter filtering events", Label("filter"), func() {
		It("should skip events for already existing component versions", func() {
			filter = NewFilter(solarClient, "default", inputChan, outputChan, errChan, opts...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := filter.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer filter.Stop()

			// Send event
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

			select {
			case errEvent := <-errChan:
				Expect(errEvent.Error).To(Not(HaveOccurred()))
			case ev := <-outputChan:
				Fail(fmt.Sprintf("should not have received event, but got: %+v", ev))
			case <-time.After(5 * time.Second):
				// Success, should timeout since no event should be received
			}
		})
	})

})
