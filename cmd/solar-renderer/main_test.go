// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSolarRenderer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Solar Renderer Suite")
}

var _ = Describe("solar-renderer command", func() {
	var (
		tmpConfigFile   *os.File
		tmpDockerConfig *os.File
		testRegistry    *registry.Registry
		testServer      *http.Server
		registryURL     string

		username = "myusername"
		password = "mypassword"
	)

	validReleaseConfig := func() solarv1alpha1.RendererConfig {
		return solarv1alpha1.RendererConfig{
			Type: solarv1alpha1.RendererConfigTypeRelease,
			ReleaseConfig: solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-chart",
					Description: "Test Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
						Name: "test-component",
					},
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"resource1": {
							Repository: "oci://example.com/resource1",
							Tag:        "v1.0.0",
						},
						"resource2": {
							Repository: "oci://example.com/resource2",
							Tag:        "v1.0.0",
						},
					},
					Entrypoint: solarv1alpha1.Entrypoint{
						ResourceName: "resource1",
						Type:         solarv1alpha1.EntrypointTypeHelm,
					},
				},
				Values: runtime.RawExtension{},
			},
		}
	}

	writeToTmpConfig := func(config solarv1alpha1.RendererConfig) {
		configData, err := yaml.Marshal(config)
		Expect(err).NotTo(HaveOccurred())

		_, err = tmpConfigFile.Write(configData)
		Expect(err).NotTo(HaveOccurred())
		_ = tmpConfigFile.Close()
	}

	writeTmpDockerConfig := func() {
		var err error
		tmpDockerConfig, err = os.CreateTemp("", "dockerconfig-*.json")
		Expect(err).NotTo(HaveOccurred())

		auth := base64.StdEncoding.EncodeToString(fmt.Appendf([]byte{}, "%s:%s", username, password))
		url := strings.TrimPrefix(registryURL, "oci://")

		config := map[string]any{
			"auths": map[string]any{
				url: map[string]string{
					"auth": auth,
				},
			},
		}
		dockerconfig, err := json.Marshal(config)
		Expect(err).NotTo(HaveOccurred())
		_, err = tmpDockerConfig.Write(dockerconfig)
		Expect(err).NotTo(HaveOccurred())
		_ = tmpDockerConfig.Close()
	}

	BeforeEach(func() {
		var err error
		tmpConfigFile, err = os.CreateTemp("", "renderer-config-*.yaml")
		Expect(err).NotTo(HaveOccurred())

		// Start test registry
		testRegistry = registry.New().WithAuth(username, password)

		// Find an available port
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		registryAddr := listener.Addr().String()
		_ = listener.Close()

		// Create HTTP server for the registry
		testServer = &http.Server{
			Addr:              registryAddr,
			Handler:           testRegistry.HandleFunc(),
			ReadHeaderTimeout: 5 * time.Second,
		}

		// Start server in background
		go func() {
			_ = testServer.ListenAndServe()
		}()

		// Give server time to start
		Eventually(func() error {
			conn, err := net.Dial("tcp", registryAddr)
			if err != nil {
				return err
			}
			_ = conn.Close()

			return nil
		}, "5s").Should(Succeed())

		registryURL = fmt.Sprintf("oci://%s", registryAddr)
	})

	AfterEach(func() {
		if tmpConfigFile != nil {
			_ = os.Remove(tmpConfigFile.Name())
		}
		_ = testServer.Shutdown(context.TODO())
	})

	Describe("render-only mode", func() {
		It("should render a release from config file", func() {
			writeToTmpConfig(validReleaseConfig())

			// Execute command
			cmd := newRootCmd()
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--skip-push"})

			var output bytes.Buffer
			cmd.SetOut(&output)

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Verify output mentions rendering
			Expect(output.String()).To(ContainSubstring("Rendered release"))
		})

		It("should fail with invalid config file", func() {
			cmd := newRootCmd()
			cmd.SetArgs([]string{"/nonexistent/config.yaml", "--skip-push"})

			var output bytes.Buffer
			cmd.SetOut(&output)

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read config-file"))
		})

		It("should fail with malformed YAML", func() {
			config := "invalid: yaml: content: ["
			_, err := tmpConfigFile.WriteString(config)
			Expect(err).NotTo(HaveOccurred())
			_ = tmpConfigFile.Close()

			cmd := newRootCmd()
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--skip-push"})

			var output bytes.Buffer
			cmd.SetOut(&output)

			err = cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse config-file"))
		})

		It("should fail with invalid TMPDIR", func() {
			oldTmp := os.Getenv("TMPDIR")
			defer func() { _ = os.Setenv("TMPDIR", oldTmp) }()
			err := os.Setenv("TMPDIR", "/nonexistent")
			Expect(err).NotTo(HaveOccurred())

			writeToTmpConfig(validReleaseConfig())

			cmd := newRootCmd()
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--skip-push"})

			err = cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no such file or directory"))
		})

		It("should fail with unknown type", func() {
			writeToTmpConfig(solarv1alpha1.RendererConfig{
				Type: "unknown",
			})

			cmd := newRootCmd()
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--skip-push"})

			var output bytes.Buffer
			cmd.SetOut(&output)

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown type specified"))
		})
	})

	Describe("render and push mode", func() {
		It("should render and push a release to OCI registry", func() {
			writeToTmpConfig(validReleaseConfig())

			// Execute command
			cmd := newRootCmd()
			cmd.SetArgs([]string{
				"--plain-http",
				"--url=" + registryURL + "/test-chart:1.0.0",
				"--username=" + username,
				"--password=" + password,
				tmpConfigFile.Name(),
			})

			var output bytes.Buffer
			cmd.SetOut(&output)

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Verify output mentions both rendering and pushing
			Expect(output.String()).To(ContainSubstring("Rendered release"))
			Expect(output.String()).To(ContainSubstring("Pushed result to"))
		})

		It("should render and push a release to OCI registry with dockerconfig", func() {
			writeTmpDockerConfig()
			oldDockerConfig := os.Getenv("DOCKER_CONFIG")
			defer func() { _ = os.Setenv("DOCKER_CONFIG", oldDockerConfig) }()
			err := os.Setenv("DOCKER_CONFIG", tmpDockerConfig.Name())
			Expect(err).NotTo(HaveOccurred())

			writeToTmpConfig(validReleaseConfig())

			// Execute command
			cmd := newRootCmd()
			cmd.SetArgs([]string{
				"--plain-http",
				"--url=" + registryURL + "/test-chart:1.0.0",
				tmpConfigFile.Name(),
			})

			var output bytes.Buffer
			cmd.SetOut(&output)

			err = cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Verify output mentions both rendering and pushing
			Expect(output.String()).To(ContainSubstring("Rendered release"))
			Expect(output.String()).To(ContainSubstring("Pushed result to"))
		})

		It("should fail push with invalid registry credentials", func() {
			writeToTmpConfig(validReleaseConfig())

			cmd := newRootCmd()
			cmd.SetArgs([]string{
				tmpConfigFile.Name(),
				"--url=" + registryURL + "/test-chart:1.0.0",
				"--plain-http",
				"--username=" + username,
				"--password=wrong-password",
			})

			var output bytes.Buffer
			cmd.SetOut(&output)

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to push result"))
		})
	})
})
