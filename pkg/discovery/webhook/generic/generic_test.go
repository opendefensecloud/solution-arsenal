// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package generic

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

	"github.com/aws/smithy-go/ptr"
	"github.com/google/uuid"

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

func TestGenericWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Generic Webhook Handler Suite")
}

var _ = Describe("Generic Webhook Handler", Ordered, func() {
	var (
		webhookPort   int
		eventsChan    chan discovery.RepositoryEvent
		webhookRouter *webhook.WebhookRouter
	)

	BeforeAll(func() {
		// Setup webhook event handling
		eventsChan = make(chan discovery.RepositoryEvent, 100)
		webhookRouter = webhook.NewWebhookRouter(eventsChan)

		// Configure webhook for generic registry
		registry := &discovery.Registry{
			Name:        "test-generic",
			Hostname:    "localhost:5000",
			PlainHTTP:   true,
			Flavor:      "generic",
			WebhookPath: "generic",
		}

		err := webhookRouter.RegisterPath(registry)
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

	Describe("event injection", func() {
		It("should receive and process image updated events", func() {
			// Clear the event channel
			for len(eventsChan) > 0 {
				<-eventsChan
			}

			eventID := uuid.NewString()

			data, err := json.Marshal(Data{
				Repository: "test/myapp",
				Version:    ptr.String("v0.1.0"),
			})
			Expect(err).NotTo(HaveOccurred())

			// Create a valid event with generic event data
			event := Envelope{
				ID:        eventID,
				Timestamp: time.Now(),
				Type:      EventTypeImageUpdated,
				Data:      data,
			}

			// Convert event to HTTP request
			body, err := json.Marshal(event)
			Expect(err).NotTo(HaveOccurred())

			// Send the event to the webhook endpoint
			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d/webhook/generic", webhookPort),
				"application/json",
				bytes.NewReader(body),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
			_ = resp.Body.Close()

			// Verify that the webhook event was received and processed
			Eventually(func() int {
				return len(eventsChan)
			}, 3*time.Second, 100*time.Millisecond).Should(BeNumerically(">", 0))

			// Verify the event is properly formed
			var repositoryEvent discovery.RepositoryEvent
			Expect(eventsChan).Should(Receive(&repositoryEvent))
			Expect(repositoryEvent.Registry).To(Equal("test-generic"))
			Expect(repositoryEvent.Repository).To(Equal("test/myapp"))
			Expect(repositoryEvent.Type).To(Equal(discovery.EventUpdated))
			Expect(repositoryEvent.Version).To(Equal("v0.1.0"))
			Expect(repositoryEvent.Timestamp).NotTo(BeZero())
		})

		It("should receive and process image created events", func() {
			// Clear the event channel
			for len(eventsChan) > 0 {
				<-eventsChan
			}

			eventID := uuid.NewString()

			data, err := json.Marshal(Data{
				Repository: "test/newrepo",
				Version:    nil,
			})
			Expect(err).NotTo(HaveOccurred())

			// Create a valid event with generic event data
			event := Envelope{
				ID:        eventID,
				Timestamp: time.Now(),
				Type:      EventTypeImageCreated,
				Data:      data,
			}

			body, err := json.Marshal(event)
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d/webhook/generic", webhookPort),
				"application/cloudevents+json",
				bytes.NewReader(body),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
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

			eventID := uuid.NewString()

			data, err := json.Marshal(Data{
				Repository: "test/obsolete",
				Version:    nil,
			})
			Expect(err).NotTo(HaveOccurred())

			// Create a valid event with generic event data
			event := Envelope{
				ID:        eventID,
				Timestamp: time.Now(),
				Type:      EventTypeImageDeleted,
				Data:      data,
			}

			body, err := json.Marshal(event)
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d/webhook/generic", webhookPort),
				"application/cloudevents+json",
				bytes.NewReader(body),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
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
		It("should register generic webhook handler with router", func() {
			eventID := uuid.NewString()

			data, err := json.Marshal(Data{
				Repository: "",
				Version:    nil,
			})
			Expect(err).NotTo(HaveOccurred())

			// Create a valid event with generic event data
			event := Envelope{
				ID:        eventID,
				Timestamp: time.Now(),
				Type:      EventTypeImageUpdated,
				Data:      data,
			}

			eventDataJSON, err := json.Marshal(event)
			Expect(err).NotTo(HaveOccurred())

			// Create request with CloudEvents headers in HTTP headers format
			req := httptest.NewRequest(http.MethodPost, "/webhook/generic", bytes.NewReader(eventDataJSON))
			req.Header.Set("Content-Type", "application/json")

			// Send request to webhook router
			w := httptest.NewRecorder()
			webhookRouter.ServeHTTP(w, req)

			// Handler should process the event successfully (204 NO CONTENT)
			Expect(w.Code).To(Equal(http.StatusNoContent))
		})

		It("should handle unknown webhook paths", func() {
			req := httptest.NewRequest(http.MethodPost, "/webhook/unknown", nil)
			w := httptest.NewRecorder()
			webhookRouter.ServeHTTP(w, req)

			// Unknown paths should return 404
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
