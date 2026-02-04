// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"go.opendefense.cloud/solar/pkg/renderer"
	"go.opendefense.cloud/solar/test/registry"
	"sigs.k8s.io/yaml"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func TestSolarRenderer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Solar Renderer Suite")
}

var _ = Describe("solar-renderer command", func() {
	var (
		tmpConfigFile *os.File
		testRegistry  *registry.Registry
		testServer    *http.Server
		registryURL   string
	)

	validReleaseConfig := func() renderer.Config {
		return renderer.Config{
			Type: renderer.TypeRelease,
			ReleaseConfig: renderer.ReleaseConfig{
				Chart: renderer.ChartConfig{
					Name:        "test-chart",
					Description: "Test Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: renderer.ReleaseInput{
					Component: renderer.ReleaseComponent{
						Name: "test-component",
					},
					Helm: solarv1alpha1.ResourceAccess{
						Repository: "oci://example.com/helm",
						Tag:        "v1.0.0",
					},
					KRO: solarv1alpha1.ResourceAccess{
						Repository: "oci://example.com/kro",
						Tag:        "v1.0.0",
					},
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"resource1": {
							Repository: "oci://example.com/resource1",
							Tag:        "v1.0.0",
						},
					},
				},
				Values: json.RawMessage(`{}`),
			},
			PushOptions: renderer.PushOptions{
				ReferenceURL: registryURL + "/test-chart:1.0.0",
				PlainHTTP:    true,
				Username:     "testuser",
				Password:     "testpass",
			},
		}
	}

	writeToTmpConfig := func(config renderer.Config) {
		configData, err := yaml.Marshal(config)
		Expect(err).NotTo(HaveOccurred())

		_, err = tmpConfigFile.Write(configData)
		Expect(err).NotTo(HaveOccurred())
		_ = tmpConfigFile.Close()
	}

	BeforeEach(func() {
		var err error
		tmpConfigFile, err = os.CreateTemp("", "renderer-config-*.yaml")
		Expect(err).NotTo(HaveOccurred())

		// Start test registry
		testRegistry = registry.New().WithAuth("testuser", "testpass")

		// Find an available port
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		registryAddr := listener.Addr().String()
		_ = listener.Close()

		// Create HTTP server for the registry
		testServer = &http.Server{
			Addr:    registryAddr,
			Handler: testRegistry.HandleFunc(),
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
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--push=false"})

			var output bytes.Buffer
			cmd.SetOut(&output)

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Verify output mentions rendering
			Expect(output.String()).To(ContainSubstring("Renderered release"))
		})

		It("should fail with invalid config file", func() {
			cmd := newRootCmd()
			cmd.SetArgs([]string{"/nonexistent/config.yaml", "--push=false"})

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
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--push=false"})

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
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--push=false"})

			err = cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no such file or directory"))
		})

		It("should fail with unknown type", func() {
			writeToTmpConfig(renderer.Config{
				Type: "unknown",
			})

			cmd := newRootCmd()
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--push=false"})

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
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--push=true"})

			var output bytes.Buffer
			cmd.SetOut(&output)

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Verify output mentions both rendering and pushing
			Expect(output.String()).To(ContainSubstring("Renderered release"))
			Expect(output.String()).To(ContainSubstring("Pushed result to"))
		})

		It("should fail push with invalid registry credentials", func() {
			config := validReleaseConfig()
			config.PushOptions.Password = "wrongpass"
			writeToTmpConfig(config)

			cmd := newRootCmd()
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--push=true"})

			var output bytes.Buffer
			cmd.SetOut(&output)

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to push result"))
		})

	})
})
