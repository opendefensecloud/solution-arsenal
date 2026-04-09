// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package zot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/discovery/webhook"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Helper function to get a free port
func getFreePort() int {
	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")

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
		zotRegistry := &discovery.Registry{
			Name:        "test-zot",
			Hostname:    "localhost:5000",
			PlainHTTP:   true,
			Flavor:      "zot",
			WebhookPath: "zot",
		}

		err := webhookRouter.RegisterPath(zotRegistry)
		Expect(err).NotTo(HaveOccurred())

		// Find free port for webhook server
		webhookPort = getFreePort()

		// Start webhook server
		go func() {
			// nolint:gosec
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
				Manifest:  "{}",
			}

			// Create CloudEvent
			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(ZotEventTypeImageUpdated)
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
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
			_ = resp.Body.Close()

			// Verify that the webhook event was received and processed
			Eventually(func() int {
				return len(eventsChan)
			}, 3*time.Second, 100*time.Millisecond).Should(BeNumerically(">", 0))

			// Verify the event is properly formed
			var repositoryEvent discovery.RepositoryEvent
			Expect(eventsChan).Should(Receive(&repositoryEvent))
			Expect(repositoryEvent.Registry).To(Equal("test-zot"))
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
			event.SetType(ZotEventTypeRepositoryCreated)
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
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
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
				Name:      "test/obsolete",
				Reference: "0.1",
			}

			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(ZotEventTypeImageDeleted)
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
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
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

		It("should handle invalid CloudEvent payload gracefully", func() {
			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d/webhook/zot", webhookPort),
				"application/cloudevents+json",
				bytes.NewReader([]byte("bad request")),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			_ = resp.Body.Close()
		})

		It("should handle invalid Zot data payload gracefully", func() {
			invalidEventData := struct {
				Name   int
				Digest string
			}{
				Name:   11,
				Digest: "sha256:xyz789",
			}

			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(ZotEventTypeRepositoryCreated)
			event.SetID("test-event-983")
			event.SetTime(time.Now())
			_ = event.SetData(cloudevents.ApplicationJSON, invalidEventData)

			body, err := json.Marshal(event)
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d/webhook/zot", webhookPort),
				"application/cloudevents+json",
				bytes.NewReader(body),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			_ = resp.Body.Close()
		})
	})

	Describe("Webhook handler registration", func() {
		It("should register zot webhook handler with router", func() {
			// Create a valid CloudEvent with zot event data
			eventData := ZotEventData{
				Name:      "test-repo",
				Reference: "test-tag",
				Digest:    "sha256:abc123",
				Manifest:  "{}",
			}

			eventDataJSON, err := json.Marshal(eventData)
			Expect(err).NotTo(HaveOccurred())

			// Create CloudEvent
			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(ZotEventTypeImageUpdated)
			event.SetID("test-event-123")
			_ = event.SetData(cloudevents.ApplicationJSON, eventDataJSON)

			// Create request with CloudEvents headers in HTTP headers format
			req := httptest.NewRequest(http.MethodPost, "/webhook/zot", bytes.NewReader(eventDataJSON))
			req.Header.Set("Ce-Specversion", event.SpecVersion())
			req.Header.Set("Ce-Type", event.Type())
			req.Header.Set("Ce-Source", event.Source())
			req.Header.Set("Ce-Id", event.ID())
			req.Header.Set("Content-Type", "application/json")

			// Send request to webhook router
			w := httptest.NewRecorder()
			webhookRouter.ServeHTTP(w, req)

			// Handler should process the event successfully (200 OK)
			Expect(w.Code).To(Equal(http.StatusAccepted))
		})

		It("should handle unknown webhook paths", func() {
			req := httptest.NewRequest(http.MethodPost, "/webhook/unknown", nil)
			w := httptest.NewRecorder()
			webhookRouter.ServeHTTP(w, req)

			// Unknown paths should return 404
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Digest reference filtering", func() {
		It("should skip created events with digest reference", func() {
			// Clear the event channel
			for len(eventsChan) > 0 {
				<-eventsChan
			}

			eventData := ZotEventData{
				Name:      "test/component-descriptors/opendefense.cloud/ocm-demo",
				Reference: "sha256:40bac3123555936fd4aa8260a853669283fa8d64be8f665ba9d60fd9f7d7df3b",
				Digest:    "sha256:40bac3123555936fd4aa8260a853669283fa8d64be8f665ba9d60fd9f7d7df3b",
			}

			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(ZotEventTypeImageUpdated)
			event.SetID("test-event-digest-create")
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
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
			_ = resp.Body.Close()

			// Verify no event was published
			Consistently(func() int {
				return len(eventsChan)
			}, 500*time.Millisecond, 50*time.Millisecond).Should(Equal(0))
		})

		It("should skip updated events with digest reference", func() {
			// Clear the event channel
			for len(eventsChan) > 0 {
				<-eventsChan
			}

			eventData := ZotEventData{
				Name:      "test/component-descriptors/opendefense.cloud/ocm-demo",
				Reference: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				Digest:    "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			}

			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(ZotEventTypeImageUpdated)
			event.SetID("test-event-digest-update")
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
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
			_ = resp.Body.Close()

			// Verify no event was published
			Consistently(func() int {
				return len(eventsChan)
			}, 500*time.Millisecond, 50*time.Millisecond).Should(Equal(0))
		})

		It("should accept deleted events with digest reference", func() {
			// Clear the event channel
			for len(eventsChan) > 0 {
				<-eventsChan
			}

			eventData := ZotEventData{
				Name:      "test/component-descriptors/opendefense.cloud/ocm-demo",
				Reference: "sha256:40bac3123555936fd4aa8260a853669283fa8d64be8f665ba9d60fd9f7d7df3b",
				Digest:    "sha256:40bac3123555936fd4aa8260a853669283fa8d64be8f665ba9d60fd9f7d7df3b",
			}

			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(ZotEventTypeImageDeleted)
			event.SetID("test-event-digest-delete")
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
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
			_ = resp.Body.Close()

			// Verify the delete event was published with digest info
			Eventually(func() int {
				return len(eventsChan)
			}, 3*time.Second, 100*time.Millisecond).Should(BeNumerically(">", 0))

			var repositoryEvent discovery.RepositoryEvent
			Expect(eventsChan).Should(Receive(&repositoryEvent))
			Expect(repositoryEvent.Repository).To(Equal("test/component-descriptors/opendefense.cloud/ocm-demo"))
			Expect(repositoryEvent.Type).To(Equal(discovery.EventDeleted))
			Expect(repositoryEvent.Digest).To(Equal("sha256:40bac3123555936fd4aa8260a853669283fa8d64be8f665ba9d60fd9f7d7df3b"))
		})

		It("should accept image updated events with version reference", func() {
			// Clear the event channel
			for len(eventsChan) > 0 {
				<-eventsChan
			}

			eventData := ZotEventData{
				Name:      "test/component-descriptors/opendefense.cloud/ocm-demo",
				Reference: "v26.4.0",
				Digest:    "sha256:40bac3123555936fd4aa8260a853669283fa8d64be8f665ba9d60fd9f7d7df3b",
			}

			event := cloudevents.NewEvent()
			event.SetSource("https://zot-registry/")
			event.SetType(ZotEventTypeImageUpdated)
			event.SetID("test-event-version-create")
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
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
			_ = resp.Body.Close()

			// Verify event was published
			Eventually(func() int {
				return len(eventsChan)
			}, 3*time.Second, 100*time.Millisecond).Should(BeNumerically(">", 0))

			var repositoryEvent discovery.RepositoryEvent
			Expect(eventsChan).Should(Receive(&repositoryEvent))
			Expect(repositoryEvent.Version).To(Equal("v26.4.0"))
			Expect(repositoryEvent.Type).To(Equal(discovery.EventUpdated))
		})
	})

	Describe("isDigestReference", func() {
		DescribeTable("should correctly identify digest references",
			func(ref string, expected bool) {
				Expect(isDigestReference(ref)).To(Equal(expected))
			},
			Entry("sha256 digest", "sha256:40bac3123555936fd4aa8260a853669283fa8d64be8f665ba9d60fd9f7d7df3b", true),
			Entry("sha512 digest", "sha512:abcdef1234567890", true),
			Entry("semver version", "v26.4.0", false),
			Entry("semver with v prefix", "v1.0.0", false),
			Entry("simple tag", "latest", false),
			Entry("numeric tag", "123", false),
		)
	})
})
