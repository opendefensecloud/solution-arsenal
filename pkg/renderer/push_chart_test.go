// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"os"

	"helm.sh/helm/v4/pkg/registry"
	"k8s.io/apimachinery/pkg/runtime"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	testregistry "go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
			opts := PushOptions{
				Reference: "oci://registry.example.com/charts/test:v1.0.0",
				ClientOptions: []registry.ClientOption{
					registry.ClientOptPlainHTTP(),
				},
			}

			result, err := PushChart(nil, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid RenderResult"))
			Expect(result).To(BeNil())
		})

		It("should fail with empty directory", func() {
			emptyResult := &solarv1alpha1.RenderResult{Dir: ""}
			opts := PushOptions{
				Reference: "oci://registry.example.com/charts/test:v1.0.0",
				ClientOptions: []registry.ClientOption{
					registry.ClientOptPlainHTTP(),
				},
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
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"chart": {Repository: "oci://example.com", Tag: "v1.0.0"},
					},
					Entrypoint: solarv1alpha1.Entrypoint{
						ResourceName: "chart",
						Type:         solarv1alpha1.EntrypointTypeHelm,
					},
				},
				Values: runtime.RawExtension{},
			}
			renderResult, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			opts := PushOptions{
				Reference: "",
				ClientOptions: []registry.ClientOption{
					registry.ClientOptPlainHTTP(),
				},
			}

			result, err := PushChart(renderResult, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("registry reference is required"))
			Expect(result).To(BeNil())
		})

		It("should fail with nonexistent chart directory", func() {
			nonExistentResult := &solarv1alpha1.RenderResult{Dir: "/nonexistent/path/to/chart"}
			opts := PushOptions{
				Reference: "oci://registry.example.com/charts/test:v1.0.0",
				ClientOptions: []registry.ClientOption{
					registry.ClientOptPlainHTTP(),
				},
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
			registryHandler *testregistry.Registry
		)

		BeforeEach(func() {
			// Set up a test registry with basic auth
			registryHandler = testregistry.New().WithAuth("testuser", "testpass")
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
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"resource1": {
							Repository: "oci://registry.example.com/res1",
							Tag:        "v2.0.0",
						},
					},
				},
				Values: runtime.RawExtension{
					Raw: []byte(`{"replicas": 3}`),
				},
			}

			renderResult, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(renderResult).NotTo(BeNil())

			// Extract the host and port from the test server URL
			listener := testServer.Listener.Addr().(*net.TCPAddr)
			// OCI reference must match chart name and version in strict mode
			referenceURL := fmt.Sprintf("oci://localhost:%d/my-test-chart:1.5.0", listener.Port)

			// Push the chart with PlainHTTP and basic auth
			opts := PushOptions{
				Reference: referenceURL,
				ClientOptions: []registry.ClientOption{
					registry.ClientOptBasicAuth("testuser", "testpass"),
					registry.ClientOptPlainHTTP(),
				},
			}

			result, err := PushChart(renderResult, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Ref).NotTo(BeEmpty())
			Expect(result.Ref).To(ContainSubstring("localhost"))
		})

		It("should successfully push a rendered chart to a plain HTTP registry with dockerconfig", func() {
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
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"resource1": {
							Repository: "oci://registry.example.com/res1",
							Tag:        "v2.0.0",
						},
					},
				},
				Values: runtime.RawExtension{
					Raw: []byte(`{"replicas": 3}`),
				},
			}

			renderResult, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(renderResult).NotTo(BeNil())

			// Extract the host and port from the test server URL
			listener := testServer.Listener.Addr().(*net.TCPAddr)
			// OCI reference must match chart name and version in strict mode
			referenceURL := fmt.Sprintf("oci://localhost:%d/my-test-chart:1.5.0", listener.Port)

			tmpDockerConfig, err := os.CreateTemp("", "dockerconfig-*.json")
			Expect(err).NotTo(HaveOccurred())

			auth := base64.StdEncoding.EncodeToString([]byte("testuser:testpass"))
			url := fmt.Sprintf("localhost:%d", listener.Port)

			dockerconfig := map[string]any{
				"auths": map[string]any{
					url: map[string]string{
						"auth": auth,
					},
				},
			}

			dockerjson, err := json.Marshal(dockerconfig)
			Expect(err).NotTo(HaveOccurred())
			_, err = tmpDockerConfig.Write(dockerjson)
			Expect(err).NotTo(HaveOccurred())
			_ = tmpDockerConfig.Close()

			// Push the chart with PlainHTTP and dockerconfig
			opts := PushOptions{
				Reference: referenceURL,
				ClientOptions: []registry.ClientOption{
					registry.ClientOptCredentialsFile(tmpDockerConfig.Name()),
					registry.ClientOptPlainHTTP(),
				},
			}

			result, err := PushChart(renderResult, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Ref).NotTo(BeEmpty())
			Expect(result.Ref).To(ContainSubstring("localhost"))
		})

		It("should work without basic auth on PlainHTTP registry", func() {
			// Create a registry without auth
			noAuthRegistry := testregistry.New()
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
				},
				Values: runtime.RawExtension{},
			}

			renderResult, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			listener := noAuthServer.Listener.Addr().(*net.TCPAddr)
			registryURL := fmt.Sprintf("oci://localhost:%d/no-auth-chart:1.0.0", listener.Port)

			opts := PushOptions{
				Reference: registryURL,
				ClientOptions: []registry.ClientOption{
					registry.ClientOptPlainHTTP(),
					// No Username or Password
				},
			}

			result, err := PushChart(renderResult, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Ref).NotTo(BeEmpty())
		})
	})
})
