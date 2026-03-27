// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.opendefense.cloud/solar/pkg/discovery"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	webhookHandlerOut = make(chan discovery.RepositoryEvent, 1)
)

type FakeRegistryScanner struct {
	out chan discovery.RepositoryEvent
}

func NewFakeRegistryScanner() *FakeRegistryScanner {
	return &FakeRegistryScanner{
		out: make(chan discovery.RepositoryEvent, 1),
	}
}

func (s *FakeRegistryScanner) Scan(ctx context.Context, eventsChan chan<- discovery.RepositoryEvent) {
	outEv := discovery.RepositoryEvent{
		Registry:   "default",
		Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
		Version:    "1.1.1",
		Type:       discovery.EventCreated,
		Timestamp:  time.Now().UTC(),
	}
	eventsChan <- outEv
	s.out <- outEv
}

type FakeWebhookHandler struct {
	eventsChan chan<- discovery.RepositoryEvent
}

func NewFakeWebhookHandler(registry *discovery.Registry, eventsChan chan<- discovery.RepositoryEvent) http.Handler {
	return &FakeWebhookHandler{
		eventsChan: eventsChan,
	}
}

func (wh *FakeWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	outEv := discovery.RepositoryEvent{
		Registry:   "default",
		Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
		Version:    "2.2.2",
		Type:       discovery.EventCreated,
		Timestamp:  time.Now().UTC(),
	}
	wh.eventsChan <- outEv
	webhookHandlerOut <- outEv
}

type FakeQualifierProcessor struct {
	in  chan discovery.RepositoryEvent
	out chan discovery.ComponentVersionEvent
}

func NewFakeQualifierProcessor() *FakeQualifierProcessor {
	return &FakeQualifierProcessor{
		in:  make(chan discovery.RepositoryEvent, 1),
		out: make(chan discovery.ComponentVersionEvent, 1),
	}
}

func (q *FakeQualifierProcessor) Process(ctx context.Context, ev discovery.RepositoryEvent) ([]discovery.ComponentVersionEvent, error) {
	q.in <- ev
	outEv := discovery.ComponentVersionEvent{
		Timestamp: time.Now().UTC(),
		Source:    ev,
		Namespace: "default",
		Component: "comp",
	}
	q.out <- outEv

	return []discovery.ComponentVersionEvent{outEv}, nil
}

type FakeFilterProcessor struct {
	inOut chan discovery.ComponentVersionEvent
}

func NewFakeFilterProcessor() *FakeFilterProcessor {
	return &FakeFilterProcessor{
		inOut: make(chan discovery.ComponentVersionEvent, 1),
	}
}

func (f *FakeFilterProcessor) Process(ctx context.Context, ev discovery.ComponentVersionEvent) ([]discovery.ComponentVersionEvent, error) {
	f.inOut <- ev
	return []discovery.ComponentVersionEvent{ev}, nil
}

type FakeHandlerProcessor struct {
	in  chan discovery.ComponentVersionEvent
	out chan discovery.WriteAPIResourceEvent
}

func NewFakeHandlerProcessor() *FakeHandlerProcessor {
	return &FakeHandlerProcessor{
		in:  make(chan discovery.ComponentVersionEvent, 1),
		out: make(chan discovery.WriteAPIResourceEvent, 1),
	}
}

func (h *FakeHandlerProcessor) Process(ctx context.Context, ev discovery.ComponentVersionEvent) ([]discovery.WriteAPIResourceEvent, error) {
	h.in <- ev
	outEv := discovery.WriteAPIResourceEvent{
		Timestamp:     time.Now().UTC(),
		Source:        ev,
		HelmDiscovery: discovery.HelmDiscovery{},
	}
	h.out <- outEv

	return []discovery.WriteAPIResourceEvent{outEv}, nil
}

type FakeAPIWriterProcessor struct {
	in chan discovery.WriteAPIResourceEvent
}

func NewFakeAPIWriterProcessor() *FakeAPIWriterProcessor {
	return &FakeAPIWriterProcessor{
		in: make(chan discovery.WriteAPIResourceEvent, 1),
	}
}

func (h *FakeAPIWriterProcessor) Process(ctx context.Context, ev discovery.WriteAPIResourceEvent) ([]any, error) {
	h.in <- ev

	return nil, nil
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
		It("should start and stop the pipeline", func() {

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			reg := &discovery.Registry{
				Flavor:       "zot",
				Hostname:     "registry.io",
				PlainHTTP:    true,
				ScanInterval: 30 * time.Minute,
				WebhookPath:  "fake",
			}
			regProv := discovery.NewRegistryProvider()
			err := regProv.Register(reg)
			Expect(err).NotTo(HaveOccurred())

			scanner := NewFakeRegistryScanner()
			qualifier := NewFakeQualifierProcessor()
			filter := NewFakeFilterProcessor()
			handler := NewFakeHandlerProcessor()
			writer := NewFakeAPIWriterProcessor()

			errChan := make(chan discovery.ErrorEvent, 1)

			p, err := NewPipeline("default", regProv, "127.0.0.1:0", errChan, log,
				withZotWebhookHandler(NewFakeWebhookHandler, reg),
				withScanner(scanner),
				withQualifierProcessor(qualifier),
				withFilterProcessor(filter),
				withHandlerProcessor(handler),
				withWriterProcessor(writer),
			)

			Expect(err).NotTo(HaveOccurred())
			err = p.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer p.Stop(ctx)

			checkEvents := func(sourceEv discovery.RepositoryEvent) {
				var qualifierIn discovery.RepositoryEvent
				var qualifierOut discovery.ComponentVersionEvent
				var filterInOut discovery.ComponentVersionEvent
				var handlerIn discovery.ComponentVersionEvent
				var handlerOut discovery.WriteAPIResourceEvent
				var writerIn discovery.WriteAPIResourceEvent

				readCount := 0
				for readCount < 6 {
					select {
					case ev := <-qualifier.in:
						qualifierIn = ev
						readCount++
					case ev := <-qualifier.out:
						qualifierOut = ev
						readCount++
					case ev := <-filter.inOut:
						filterInOut = ev
						readCount++
					case ev := <-handler.in:
						handlerIn = ev
						readCount++
					case ev := <-handler.out:
						handlerOut = ev
						readCount++
					case ev := <-writer.in:
						writerIn = ev
						readCount++
					case <-ctx.Done():
						Fail("Waiting for event: " + ctx.Err().Error())
					}
				}
				Expect(sourceEv).To(Equal(qualifierIn))
				Expect(qualifierOut).To(Equal(filterInOut))
				Expect(filterInOut).To(Equal(handlerIn))
				Expect(handlerOut).To(Equal(writerIn))
			}

			// Verify event from fake scanner has maded it through the pipeline
			select {
			case ev := <-scanner.out:
				checkEvents(ev)
			case <-ctx.Done():
				Fail("Waiting for event: " + ctx.Err().Error())
			}

			// Send a fake request to the webhook server and verify that the event from the fake handler has made it through the pipeline
			resp, err := http.Post("http://"+p.webhookServer.Addr+"/webhook/fake", "application/json", bytes.NewBuffer([]byte{}))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(200))

			select {
			case ev := <-webhookHandlerOut:
				checkEvents(ev)
			case <-ctx.Done():
				Fail("Waiting for event: " + ctx.Err().Error())
			}

			Expect(errChan).To(BeEmpty())
		})

	})
})
