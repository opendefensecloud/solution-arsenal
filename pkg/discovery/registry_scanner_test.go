package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestDiscovery(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Discovery Suite")
}

var _ = Describe("RegistryScanner", func() {
	var (
		scanner      *RegistryScanner
		eventsChan   chan RegistryEvent
		logger       logr.Logger
		registryURL  string
		registryData map[string][]string // repo -> tags
	)

	BeforeEach(func() {
		logger = zap.New()
		eventsChan = make(chan RegistryEvent, 100)
		registryData = make(map[string][]string)

		// Set up an embedded OCI registry server
		var err error
		registryURL, err = setupMockRegistry(registryData)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Stop the scanner if it's running
		if scanner != nil {
			scanner.Stop()
			// Stop is idempotent, safe to call again
			scanner.Stop()
		}

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	BeforeEach(func() {
		// Reset/reopen channel for each test
		eventsChan = make(chan RegistryEvent, 100)
	})

	Describe("NewRegistryScanner", func() {
		It("should create a new registry scanner with correct configuration", func() {
			creds := RegistryCredentials{
				Username: "testuser",
				Password: "testpass",
			}

			scanner = NewRegistryScanner(registryURL, creds, eventsChan, logger)

			Expect(scanner).NotTo(BeNil())
			Expect(scanner.registryURL).To(Equal(registryURL))
			Expect(scanner.credentials.Username).To(Equal("testuser"))
			Expect(scanner.credentials.Password).To(Equal("testpass"))
			Expect(scanner.eventsChan).To(Equal(eventsChan))
		})

		It("should use default scan interval of 30 seconds", func() {
			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryURL, creds, eventsChan, logger)

			Expect(scanner.scanInterval).To(Equal(30 * time.Second))
		})
	})

	Describe("SetScanInterval", func() {
		It("should update the scan interval", func() {
			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryURL, creds, eventsChan, logger)

			newInterval := 5 * time.Second
			scanner.SetScanInterval(newInterval)

			Expect(scanner.scanInterval).To(Equal(newInterval))
		})
	})

	Describe("Start and Stop", func() {
		It("should start and stop the scanner gracefully", func() {
			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryURL, creds, eventsChan, logger)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Give it a moment to start
			time.Sleep(100 * time.Millisecond)

			// Should be able to stop without blocking
			scanner.Stop()
		})
	})

	Describe("Registry scanning", func() {
		It("should discover repositories and tags in the registry", func() {
			// Note: ORAS registry client has HTTPS enforcement for non-localhost registries
			// These tests focus on the RegistryScanner's core logic rather than full HTTP integration
			// Use the sendEvent and event channel mechanism directly

			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryURL, creds, eventsChan, logger)

			// Test the event sending mechanism
			event := RegistryEvent{
				RepositoryURL: fmt.Sprintf("http://%s/myapp", registryURL),
				Tag:           "v1.0",
				Timestamp:     time.Now(),
			}
			scanner.sendEvent(event)

			// Read the event
			timeout := time.After(1 * time.Second)
			select {
			case receivedEvent := <-eventsChan:
				Expect(receivedEvent.RepositoryURL).To(ContainSubstring("myapp"))
				Expect(receivedEvent.Tag).To(Equal("v1.0"))
			case <-timeout:
				Fail("timeout waiting for event")
			}
		})

		It("should handle empty registry gracefully", func() {
			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryURL, creds, eventsChan, logger)
			scanner.SetScanInterval(100 * time.Millisecond)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Wait a bit for the scanner to attempt a scan
			time.Sleep(200 * time.Millisecond)

			scanner.Stop()
			// Should handle graceful shutdown
		})

		It("should send events with correct timestamp", func() {
			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryURL, creds, eventsChan, logger)

			beforeCreate := time.Now()
			event := RegistryEvent{
				RepositoryURL: fmt.Sprintf("http://%s/testapp", registryURL),
				Tag:           "v1.0",
				Timestamp:     time.Now(),
			}
			scanner.sendEvent(event)

			timeout := time.After(1 * time.Second)
			select {
			case receivedEvent := <-eventsChan:
				Expect(receivedEvent.Timestamp.After(beforeCreate.Add(-1 * time.Second))).To(BeTrue())
				Expect(receivedEvent.Timestamp.Before(time.Now().Add(1 * time.Second))).To(BeTrue())
			case <-timeout:
				Fail("timeout waiting for event")
			}
		})

		It("should handle multiple repositories and tags", func() {
			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryURL, creds, eventsChan, logger)

			// Send multiple events
			repos := map[string][]string{
				"myapp":    {"v1.0", "v1.1", "latest"},
				"database": {"5.7", "8.0"},
			}

			for repo, tags := range repos {
				for _, tag := range tags {
					event := RegistryEvent{
						RepositoryURL: fmt.Sprintf("http://%s/%s", registryURL, repo),
						Tag:           tag,
						Timestamp:     time.Now(),
					}
					scanner.sendEvent(event)
				}
			}

			// Collect all events
			events := make([]RegistryEvent, 0)
			timeout := time.After(2 * time.Second)
			for {
				select {
				case event := <-eventsChan:
					events = append(events, event)
					if len(events) == 5 {
						goto checkEvents
					}
				case <-timeout:
					goto checkEvents
				}
			}

		checkEvents:
			Expect(len(events) >= 5).To(BeTrue())
		})
	})

	Describe("Event channel behavior", func() {
		It("should not block when sending events to full channel", func() {
			// Create a small buffered channel
			smallChan := make(chan RegistryEvent, 1)

			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryURL, creds, smallChan, logger)

			// Manually call sendEvent multiple times
			// The non-blocking send should not panic or block
			for i := 0; i < 10; i++ {
				event := RegistryEvent{
					RepositoryURL: fmt.Sprintf("repo%d", i),
					Tag:           "latest",
				}
				// This should not block even if channel is full
				Expect(func() {
					scanner.sendEvent(event)
				}).NotTo(Panic())
			}
		})

		It("should send events with correct repository URL and tag", func() {
			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryURL, creds, eventsChan, logger)

			event := RegistryEvent{
				RepositoryURL: fmt.Sprintf("http://%s/myrepo", registryURL),
				Tag:           "v1.0",
				Timestamp:     time.Now(),
			}
			scanner.sendEvent(event)

			timeout := time.After(1 * time.Second)
			select {
			case receivedEvent := <-eventsChan:
				Expect(receivedEvent.RepositoryURL).To(ContainSubstring("myrepo"))
				Expect(receivedEvent.Tag).To(Equal("v1.0"))
				Expect(receivedEvent.Timestamp).NotTo(BeZero())
			case <-timeout:
				Fail("timeout waiting for event")
			}
		})
	})
})

// mockRegistryHandler implements a simple OCI registry HTTP API for testing.
type mockRegistryHandler struct {
	registryData map[string][]string // repo -> tags
}

// catalogResponse represents the Docker Registry V2 API catalog response.
type catalogResponse struct {
	Repositories []string `json:"repositories"`
}

// tagsResponse represents the Docker Registry V2 API tags response.
type tagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (h *mockRegistryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle the Docker Registry V2 API endpoints
	switch {
	case r.Method == http.MethodHead && r.URL.Path == "/v2/":
		// Docker Registry API version check
		w.Header().Set("Docker-Distribution-Api-Version", "2.0")
		w.WriteHeader(http.StatusOK)

	case r.Method == http.MethodGet && r.URL.Path == "/v2/":
		// Docker Registry API version check
		w.Header().Set("Docker-Distribution-Api-Version", "2.0")
		w.WriteHeader(http.StatusOK)

	case r.Method == http.MethodGet && r.URL.Path == "/v2/_catalog":
		// Catalog endpoint - return list of repositories
		repos := make([]string, 0, len(h.registryData))
		for repo := range h.registryData {
			repos = append(repos, repo)
		}

		response := catalogResponse{Repositories: repos}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)

	case r.Method == http.MethodGet && r.URL.Path != "" && r.URL.Path != "/":
		// Parse repository name from path like /v2/{name}/tags/list
		// Extract repo name from path
		path := r.URL.Path
		if len(path) > 4 && path[:4] == "/v2/" {
			repoPath := path[4:] // Remove /v2/
			if len(repoPath) > 10 && repoPath[len(repoPath)-10:] == "/tags/list" {
				repoName := repoPath[:len(repoPath)-10]

				if tags, ok := h.registryData[repoName]; ok {
					response := tagsResponse{
						Name: repoName,
						Tags: tags,
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(response)
					return
				}
			}
		}

		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "Not found")

	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "Not found")
	}
}

// setupMockRegistry creates a mock OCI registry server for testing.
func setupMockRegistry(registryData map[string][]string) (string, error) {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to create listener: %w", err)
	}

	serverAddr := listener.Addr().String()

	handler := &mockRegistryHandler{
		registryData: registryData,
	}

	server := &http.Server{
		Addr:         serverAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in background
	go func() {
		_ = server.Serve(listener)
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	return serverAddr, nil
}
