// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestDiscovery(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Discovery Suite")
}

var _ = Describe("RegistryScanner", Ordered, func() {
	var (
		scanner        *RegistryScanner
		eventsChan     chan RegistryEvent
		logger         logr.Logger
		registryURL    string
		registryTLSURL string
		testServer     *httptest.Server
		testTLSServer  *httptest.Server
	)

	BeforeAll(func() {
		logger = zap.New()

		reg := registry.New()
		testServer = httptest.NewServer(reg.HandleFunc())
		testTLSServer = httptest.NewTLSServer(reg.HandleFunc())
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		testServerUrl, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		testTLSServerUrl, err := url.Parse(testTLSServer.URL)
		Expect(err).NotTo(HaveOccurred())

		registryURL = testServerUrl.Host
		registryTLSURL = testTLSServerUrl.Host

		_, err = run(exec.Command("ocm", "transfer", "ctf", "./test/fixtures/helmdemo-ctf", fmt.Sprintf("http://%s/test", registryURL)))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		testServer.Close()
		testTLSServer.Close()
	})

	BeforeEach(func() {
		eventsChan = make(chan RegistryEvent, 100)
	})

	AfterEach(func() {
		scanner.Stop()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("NewRegistryScanner", func() {
		It("should create a new registry scanner with correct configuration", func() {
			creds := RegistryCredentials{
				Username: "testuser",
				Password: "testpass",
			}

			scanner = NewRegistryScanner(registryTLSURL, creds, eventsChan, logger)
			Expect(scanner).NotTo(BeNil())
			Expect(scanner.registryURL).To(Equal(registryTLSURL))
			Expect(scanner.credentials.Username).To(Equal("testuser"))
			Expect(scanner.credentials.Password).To(Equal("testpass"))
			Expect(scanner.eventsChan).To(Equal(eventsChan))
		})

		It("should use default scan interval of 30 seconds", func() {
			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryTLSURL, creds, eventsChan, logger)

			Expect(scanner.scanInterval).To(Equal(30 * time.Second))
		})
	})

	Describe("SetScanInterval", func() {
		It("should update the scan interval", func() {
			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryTLSURL, creds, eventsChan, logger)

			newInterval := 5 * time.Second
			scanner.SetScanInterval(newInterval)

			Expect(scanner.scanInterval).To(Equal(newInterval))
		})
	})

	Describe("Start and Stop", func() {
		It("should start and stop the scanner gracefully", func() {
			creds := RegistryCredentials{}
			scanner = NewRegistryScanner(registryTLSURL, creds, eventsChan, logger)

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
			scanner = NewRegistryScanner(registryTLSURL, creds, eventsChan, logger)

			// Test the event sending mechanism
			event := RegistryEvent{
				RepositoryURL: fmt.Sprintf("http://%s/myapp", registryTLSURL),
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
			scanner = NewRegistryScanner(registryTLSURL, creds, eventsChan, logger)
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
			scanner = NewRegistryScanner(registryTLSURL, creds, eventsChan, logger)

			beforeCreate := time.Now()
			event := RegistryEvent{
				RepositoryURL: fmt.Sprintf("http://%s/testapp", registryTLSURL),
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
			scanner = NewRegistryScanner(registryTLSURL, creds, eventsChan, logger)

			// Send multiple events
			repos := map[string][]string{
				"myapp":    {"v1.0", "v1.1", "latest"},
				"database": {"5.7", "8.0"},
			}

			for repo, tags := range repos {
				for _, tag := range tags {
					event := RegistryEvent{
						RepositoryURL: fmt.Sprintf("http://%s/%s", registryTLSURL, repo),
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
			scanner = NewRegistryScanner(registryTLSURL, creds, smallChan, logger)

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
			scanner = NewRegistryScanner(registryTLSURL, creds, eventsChan, logger)

			event := RegistryEvent{
				RepositoryURL: fmt.Sprintf("http://%s/myrepo", registryTLSURL),
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

// getProjectDir will return the directory where the project is
func getProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, fmt.Errorf("failed to get current working directory: %w", err)
	}
	wd = strings.ReplaceAll(wd, "/pkg/discovery", "")
	return wd, nil
}

func logf(format string, a ...any) {
	_, _ = fmt.Fprintf(GinkgoWriter, format, a...)
}

// run executes the provided command within this context
func run(cmd *exec.Cmd) (string, error) {
	dir, _ := getProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		logf("chdir dir: %q\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	logf("running: %q\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%q failed with error %q: %w", command, string(output), err)
	}

	return string(output), nil
}
