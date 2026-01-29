// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package zot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/discovery/webhook"
)

// Helper function to get a free port
func getFreePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer func() { _ = listener.Close() }()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port
}

func TestZotWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Zot Webhook Handler Suite")
}

var _ = Describe("Zot Webhook Handler", Ordered, func() {
	var (
		webhookPort   int
		eventsChan    chan discovery.RepositoryEvent
		webhookRouter *webhook.WebhookRouter
	)

	BeforeAll(func() {
		// Setup webhook event handling
		eventsChan = make(chan discovery.RepositoryEvent, 100)
		webhookRouter = webhook.NewWebhookRouter(eventsChan)

		// Configure webhook for zot registry
		zotRegistry := webhook.Registry{
			Name:   "test-zot",
			URL:    "http://localhost:5000",
			Flavor: "zot",
			Webhook: &webhook.Webhook{
				Path: "zot",
			},
		}

		err := webhookRouter.RegisterPath(zotRegistry)
		Expect(err).NotTo(HaveOccurred())

		// Find free port for webhook server
		webhookPort = getFreePort()

		// Start webhook server
		go func() {
			server := &http.Server{
				Addr:    fmt.Sprintf("127.0.0.1:%d", webhookPort),
				Handler: webhookRouter,
			}
			_ = server.ListenAndServe()
		}()

		// Give webhook server time to start
		time.Sleep(100 * time.Millisecond)
	})

	AfterAll(func() {
		close(eventsChan)
	})

	Describe("CloudEvent injection", func() {
		It("should receive and process image updated events", func() {
			// Clear the event channel
			for len(eventsChan) > 0 {
				<-eventsChan
			}

			// Create a valid CloudEvent with zot event data
			eventData := ZotEventData{
				Name:      "test/myapp",
				Reference: "v1.0",
				Digest:    "sha256:abc123def456",
				Manifest: Manifest{
					SchemaVersion: 2,
					MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
					Config: Config{
						MediaType: "application/vnd.docker.container.image.v1+json",
						Size:      1024,
						Digest:    "sha256:def456",
					},
				},
			}

			// Create CloudEvent
			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(EventTypeImageUpdated)
			event.SetID("test-event-123")
			event.SetTime(time.Now())
			_ = event.SetData(cloudevents.ApplicationJSON, eventData)

			// Convert event to HTTP request
			body, err := json.Marshal(event)
			Expect(err).NotTo(HaveOccurred())

			// Send the CloudEvent to the webhook endpoint
			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d/webhook/zot", webhookPort),
				"application/cloudevents+json",
				bytes.NewReader(body),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			_ = resp.Body.Close()

			// Verify that the webhook event was received and processed
			Eventually(func() int {
				return len(eventsChan)
			}, 3*time.Second, 100*time.Millisecond).Should(BeNumerically(">", 0))

			// Verify the event is properly formed
			var repositoryEvent discovery.RepositoryEvent
			Expect(eventsChan).Should(Receive(&repositoryEvent))
			Expect(repositoryEvent.Registry.Hostname).To(Equal("http://localhost:5000"))
			Expect(repositoryEvent.Repository).To(Equal("test/myapp"))
			Expect(repositoryEvent.Type).To(Equal(discovery.EventUpdated))
			Expect(repositoryEvent.Timestamp).NotTo(BeZero())
		})

		It("should receive and process repository created events", func() {
			// Clear the event channel
			for len(eventsChan) > 0 {
				<-eventsChan
			}

			eventData := ZotEventData{
				Name:      "test/newrepo",
				Reference: "latest",
				Digest:    "sha256:xyz789",
			}

			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(EventTypeRepositoryCreated)
			event.SetID("test-event-456")
			event.SetTime(time.Now())
			_ = event.SetData(cloudevents.ApplicationJSON, eventData)

			body, err := json.Marshal(event)
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d/webhook/zot", webhookPort),
				"application/cloudevents+json",
				bytes.NewReader(body),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			_ = resp.Body.Close()

			// Verify event received
			Eventually(func() int {
				return len(eventsChan)
			}, 3*time.Second, 100*time.Millisecond).Should(BeNumerically(">", 0))

			var repositoryEvent discovery.RepositoryEvent
			Expect(eventsChan).Should(Receive(&repositoryEvent))
			Expect(repositoryEvent.Repository).To(Equal("test/newrepo"))
			Expect(repositoryEvent.Type).To(Equal(discovery.EventCreated))
		})

		It("should receive and process image deleted events", func() {
			// Clear the event channel
			for len(eventsChan) > 0 {
				<-eventsChan
			}

			eventData := ZotEventData{
				Name:   "test/obsolete",
				Digest: "sha256:old123",
			}

			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(EventTypeImageDeleted)
			event.SetID("test-event-789")
			event.SetTime(time.Now())
			_ = event.SetData(cloudevents.ApplicationJSON, eventData)

			body, err := json.Marshal(event)
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d/webhook/zot", webhookPort),
				"application/cloudevents+json",
				bytes.NewReader(body),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			_ = resp.Body.Close()

			// Verify event received
			Eventually(func() int {
				return len(eventsChan)
			}, 3*time.Second, 100*time.Millisecond).Should(BeNumerically(">", 0))

			var repositoryEvent discovery.RepositoryEvent
			Expect(eventsChan).Should(Receive(&repositoryEvent))
			Expect(repositoryEvent.Repository).To(Equal("test/obsolete"))
			Expect(repositoryEvent.Type).To(Equal(discovery.EventDeleted))
		})
	})

	Describe("Webhook handler registration", func() {
		It("should register zot webhook handler with router", func() {
			// Create a valid CloudEvent with zot event data
			eventData := ZotEventData{
				Name:      "test-repo",
				Reference: "test-tag",
				Digest:    "sha256:abc123",
				Manifest: Manifest{
					SchemaVersion: 2,
					MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
					Config: Config{
						MediaType: "application/vnd.docker.container.image.v1+json",
						Size:      1024,
						Digest:    "sha256:def456",
					},
				},
			}

			eventDataJSON, err := json.Marshal(eventData)
			Expect(err).NotTo(HaveOccurred())

			// Create CloudEvent
			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(EventTypeImageUpdated)
			event.SetID("test-event-123")
			_ = event.SetData(cloudevents.ApplicationJSON, eventDataJSON)

			// Create request with CloudEvents headers in HTTP headers format
			req := httptest.NewRequest("POST", "/webhook/zot", bytes.NewReader(eventDataJSON))
			req.Header.Set("ce-specversion", event.SpecVersion())
			req.Header.Set("ce-type", event.Type())
			req.Header.Set("ce-source", event.Source())
			req.Header.Set("ce-id", event.ID())
			req.Header.Set("content-type", "application/json")

			// Send request to webhook router
			w := httptest.NewRecorder()
			webhookRouter.ServeHTTP(w, req)

			// Handler should process the event successfully (200 OK)
			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should handle unknown webhook paths", func() {
			req := httptest.NewRequest("POST", "/webhook/unknown", nil)
			w := httptest.NewRecorder()
			webhookRouter.ServeHTTP(w, req)

			// Unknown paths should return 404
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
