// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/test/registry"
)

var _ = Describe("PushChart", func() {
	var (
		renderResult *solarv1alpha1.RenderResult
		err          error
	)

	AfterEach(func() {
		if renderResult != nil {
			err := renderResult.Close()
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("PushChart with invalid inputs", func() {
		It("should fail with nil RenderResult", func() {
			opts := solarv1alpha1.PushOptions{
				ReferenceURL: "oci://registry.example.com/charts/test:v1.0.0",
				PlainHTTP:    true,
			}

			result, err := PushChart(nil, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid RenderResult"))
			Expect(result).To(BeNil())
		})

		It("should fail with empty directory", func() {
			emptyResult := &solarv1alpha1.RenderResult{Dir: ""}
			opts := solarv1alpha1.PushOptions{
				ReferenceURL: "oci://registry.example.com/charts/test:v1.0.0",
				PlainHTTP:    true,
			}

			result, err := PushChart(emptyResult, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid RenderResult"))
			Expect(result).To(BeNil())
		})

		It("should fail without reference URL", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:       "test-chart",
					Version:    "1.0.0",
					AppVersion: "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{Name: "test"},
					Helm:      solarv1alpha1.ResourceAccess{Repository: "oci://example.com", Tag: "v1"},
					KRO:       solarv1alpha1.ResourceAccess{Repository: "oci://example.com", Tag: "v1"},
				},
				Values: json.RawMessage(`{}`),
			}
			renderResult, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			opts := solarv1alpha1.PushOptions{
				ReferenceURL: "",
				PlainHTTP:    true,
			}

			result, err := PushChart(renderResult, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("registry URL is required"))
			Expect(result).To(BeNil())
		})

		It("should fail with nonexistent chart directory", func() {
			nonExistentResult := &solarv1alpha1.RenderResult{Dir: "/nonexistent/path/to/chart"}
			opts := solarv1alpha1.PushOptions{
				ReferenceURL: "oci://registry.example.com/charts/test:v1.0.0",
				PlainHTTP:    true,
			}

			result, err := PushChart(nonExistentResult, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Chart.yaml not found"))
			Expect(result).To(BeNil())
		})
	})

	Describe("PushChart with plain HTTP registry and basic auth", func() {
		var (
			testServer      *httptest.Server
			registryHandler *registry.Registry
		)

		BeforeEach(func() {
			// Set up a test registry with basic auth
			registryHandler = registry.New().WithAuth("testuser", "testpass")
			testServer = httptest.NewServer(registryHandler.HandleFunc())
		})

		AfterEach(func() {
			testServer.Close()
		})

		It("should successfully push a rendered chart to a plain HTTP registry with basic auth", func() {
			// Create a rendered chart
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "my-test-chart",
					Description: "Test Chart for OCI Push",
					Version:     "1.5.0",
					AppVersion:  "1.5.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
						Name: "my-component",
					},
					Helm: solarv1alpha1.ResourceAccess{
						Repository: "oci://registry.example.com/helm",
						Tag:        "v1.2.0",
					},
					KRO: solarv1alpha1.ResourceAccess{
						Repository: "oci://registry.example.com/kro",
						Tag:        "v1.0.0",
					},
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"resource1": {
							Repository: "oci://registry.example.com/res1",
							Tag:        "v2.0.0",
						},
					},
				},
				Values: json.RawMessage(`{"replicas": 3}`),
			}

			renderResult, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(renderResult).NotTo(BeNil())

			// Extract the host and port from the test server URL
			listener := testServer.Listener.Addr().(*net.TCPAddr)
			// OCI reference must match chart name and version in strict mode
			referenceURL := fmt.Sprintf("oci://localhost:%d/my-test-chart:1.5.0", listener.Port)

			// Push the chart with PlainHTTP and basic auth
			opts := solarv1alpha1.PushOptions{
				ReferenceURL: referenceURL,
				PlainHTTP:    true,
				Username:     "testuser",
				Password:     "testpass",
			}

			result, err := PushChart(renderResult, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Ref).NotTo(BeEmpty())
			Expect(result.Ref).To(ContainSubstring("localhost"))
		})

		It("should work without basic auth on PlainHTTP registry", func() {
			// Create a registry without auth
			noAuthRegistry := registry.New()
			noAuthServer := httptest.NewServer(noAuthRegistry.HandleFunc())
			defer noAuthServer.Close()

			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "no-auth-chart",
					Description: "No Auth Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{Name: "test"},
					Helm:      solarv1alpha1.ResourceAccess{Repository: "oci://example.com", Tag: "v1"},
					KRO:       solarv1alpha1.ResourceAccess{Repository: "oci://example.com", Tag: "v1"},
				},
				Values: json.RawMessage(`{}`),
			}

			renderResult, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			listener := noAuthServer.Listener.Addr().(*net.TCPAddr)
			registryURL := fmt.Sprintf("oci://localhost:%d/no-auth-chart:1.0.0", listener.Port)

			opts := solarv1alpha1.PushOptions{
				ReferenceURL: registryURL,
				PlainHTTP:    true,
				// No Username or Password
			}

			result, err := PushChart(renderResult, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Ref).NotTo(BeEmpty())
		})
	})
})
