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

	Describe("Start and Stop", func() {
		It("should start and stop the scanner gracefully", func() {
			scanner = NewRegistryScanner(registryTLSURL, RegistryCredentials{}, eventsChan, logger)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Should be able to stop without blocking
			scanner.Stop()
		})
	})

	Describe("Registry scanning", func() {
		It("should discover repositories and tags in the registry", func() {
			scanner = NewRegistryScanner(registryTLSURL, RegistryCredentials{}, eventsChan, logger)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := scanner.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Read the event
			timeout := time.After(1 * time.Second)
			select {
			case receivedEvent := <-eventsChan:
				Expect(receivedEvent.RepositoryURL).To(ContainSubstring("/test"))
				Expect(receivedEvent.Tag).NotTo(Equal(""))
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
