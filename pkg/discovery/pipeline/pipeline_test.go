// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.opendefense.cloud/solar/pkg/discovery"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type FakeRegistryScanner struct {
	eventsSent []*discovery.RepositoryEvent
}

func (s *FakeRegistryScanner) Scan(ctx context.Context, eventsChan chan<- discovery.RepositoryEvent) {
	outEv := discovery.RepositoryEvent{
		Registry:   "default",
		Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
		Version:    "0.12.0",
		Type:       discovery.EventCreated,
		Timestamp:  time.Now().UTC(),
	}
	s.eventsSent = append(s.eventsSent, &outEv)
	eventsChan <- outEv
}

type FakeQualifierProcessor struct {
	eventsSeen []*discovery.RepositoryEvent
	eventsSent []*discovery.ComponentVersionEvent
}

func (q *FakeQualifierProcessor) Process(ctx context.Context, ev discovery.RepositoryEvent) ([]discovery.ComponentVersionEvent, error) {
	q.eventsSeen = append(q.eventsSeen, &ev)
	outEv := discovery.ComponentVersionEvent{
		Timestamp: time.Now().UTC(),
		Source:    ev,
		Namespace: "default",
		Component: "comp",
	}
	q.eventsSent = append(q.eventsSent, &outEv)

	return []discovery.ComponentVersionEvent{outEv}, nil
}

type FakeFilterProcessor struct {
	events []*discovery.ComponentVersionEvent
}

func (f *FakeFilterProcessor) Process(ctx context.Context, ev discovery.ComponentVersionEvent) ([]discovery.ComponentVersionEvent, error) {
	f.events = append(f.events, &ev)
	return []discovery.ComponentVersionEvent{ev}, nil
}

type FakeHandlerProcessor struct {
	eventsSeen []*discovery.ComponentVersionEvent
	doneChan   chan bool
}

func NewFakeHandlerProcessor() *FakeHandlerProcessor {
	return &FakeHandlerProcessor{
		// We use a buffer of 1 to prevent blocking on send
		doneChan: make(chan bool, 1),
	}
}

func (h *FakeHandlerProcessor) Process(ctx context.Context, ev discovery.ComponentVersionEvent) ([]discovery.WriteAPIResourceEvent, error) {
	h.eventsSeen = append(h.eventsSeen, &ev)
	outEv := discovery.WriteAPIResourceEvent{
		Timestamp:     time.Now().UTC(),
		Source:        ev,
		HelmDiscovery: discovery.HelmDiscovery{},
	}
	h.doneChan <- true

	return []discovery.WriteAPIResourceEvent{outEv}, nil
}

func TestPipeline(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pipeline Suite")
}

var _ = Describe("Pipeline", Ordered, func() {
	var (
		log logr.Logger
	)

	BeforeAll(func() {
		log = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
		// Set to satisfy the filter, since we use a fake filter it doesn't need to point to an actual server
		os.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
		os.Setenv("KUBERNETES_SERVICE_PORT", "443")
	})

	Describe("Start and stop", func() {
		It("should  start and stop the pipeline", func() {

			ctx := context.Background()

			regProv := discovery.NewRegistryProvider()
			err := regProv.Register(&discovery.Registry{
				Hostname:     "registry.io",
				PlainHTTP:    true,
				ScanInterval: 30 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			fakeSca := &FakeRegistryScanner{}
			fakeQual := &FakeQualifierProcessor{}
			fakeFil := &FakeFilterProcessor{}
			fakeHan := NewFakeHandlerProcessor()

			p, err := NewPipeline(ctx, log, "default", regProv, "127.0.0.1:8080",
				WithScanner(fakeSca),
				WithQualifierProcessor[discovery.RepositoryEvent, discovery.ComponentVersionEvent](fakeQual),
				WithFilterProcessor[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent](fakeFil),
				WithHandlerProcessor[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](fakeHan),
			)
			Expect(err).NotTo(HaveOccurred())
			defer p.Stop()
			err = p.Start()
			Expect(err).NotTo(HaveOccurred())

			select {
			// FIXME Once the pipeline is completely implemented, check for the final result
			case flag := <-fakeHan.doneChan:
				Expect(flag).To(BeTrue())
			case <-time.After(5 * time.Second):
				Fail("timeout waiting for event")
			}

			Expect(fakeSca.eventsSent).To(HaveLen(1))
			Expect(fakeQual.eventsSeen).To(HaveLen(1))
			Expect(fakeQual.eventsSent).To(HaveLen(1))
			Expect(fakeFil.events).To(HaveLen(1))
			Expect(fakeHan.eventsSeen).To(HaveLen(1))

			Expect(fakeSca.eventsSent[0]).To(Equal(fakeQual.eventsSeen[0]))
			Expect(fakeQual.eventsSent[0]).To(Equal(fakeFil.events[0]))
			Expect(fakeFil.events[0]).To(Equal(fakeHan.eventsSeen[0]))
		})
	})

})
