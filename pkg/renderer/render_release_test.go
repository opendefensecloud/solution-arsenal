// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("RenderRelease", func() {
	var (
		result *solarv1alpha1.RenderResult
		err    error
	)

	AfterEach(func() {
		if result != nil {
			err := result.Close()
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("RenderRelease with valid ReleaseConfig", func() {
		It("should render without errors", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
				Values: runtime.RawExtension{
					Raw: []byte(`{"tag": "{{ .resources.resource1.tag }}"}`),
				},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Dir).NotTo(BeEmpty())
		})

		It("should create a temporary directory", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			// Verify directory exists
			info, err := os.Stat(result.Dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})

		It("should render all expected files", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			expectedFiles := []string{
				"Chart.yaml",
				"values.yaml",
				".helmignore",
				"templates/release.yaml",
			}

			for _, fname := range expectedFiles {
				filePath := filepath.Join(result.Dir, fname)
				_, err := os.Stat(filePath)
				Expect(err).NotTo(HaveOccurred(), "file %s should exist", fname)
			}
		})

		It("should render Chart.yaml with correct template values", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "my-test-chart",
					Description: "My Test Description",
					Version:     "2.5.0",
					AppVersion:  "2.5.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			chartPath := filepath.Join(result.Dir, "Chart.yaml")
			content, err := os.ReadFile(chartPath)
			Expect(err).NotTo(HaveOccurred())

			contentStr := string(content)
			Expect(contentStr).To(ContainSubstring("name: my-test-chart"))
			Expect(contentStr).To(ContainSubstring("description: My Test Description"))
			Expect(contentStr).To(ContainSubstring("version: 2.5.0"))
			Expect(contentStr).To(ContainSubstring("appVersion: 2.5.0"))
			Expect(contentStr).To(ContainSubstring("apiVersion: v2"))
		})

		It("should render values.yaml with input data", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
						Name: "my-component",
					},
					Helm: solarv1alpha1.ResourceAccess{
						Repository: "oci://repo.example.com/helm",
						Tag:        "v2.0.0",
					},
					KRO: solarv1alpha1.ResourceAccess{
						Repository: "oci://repo.example.com/kro",
						Tag:        "v2.0.0",
					},
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"res1": {
							Repository: "oci://repo.example.com/res1",
							Tag:        "v1.5.0",
						},
					},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			valuesPath := filepath.Join(result.Dir, "values.yaml")
			content, err := os.ReadFile(valuesPath)
			Expect(err).NotTo(HaveOccurred())

			contentStr := string(content)
			Expect(contentStr).To(ContainSubstring("component:"))
			Expect(contentStr).To(ContainSubstring("name: my-component"))
			Expect(contentStr).To(ContainSubstring("helm:"))
			Expect(contentStr).To(ContainSubstring("repository: oci://repo.example.com/helm"))
			Expect(contentStr).To(ContainSubstring("tag: v2.0.0"))
		})

		It("should render .helmignore file", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			helmIgnorePath := filepath.Join(result.Dir, ".helmignore")
			content, err := os.ReadFile(helmIgnorePath)
			Expect(err).NotTo(HaveOccurred())

			contentStr := string(content)
			Expect(contentStr).To(ContainSubstring(".DS_Store"))
			Expect(contentStr).To(ContainSubstring(".git/"))
		})

		It("should render templates/release.yaml with values", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			releasePath := filepath.Join(result.Dir, "templates", "release.yaml")
			content, err := os.ReadFile(releasePath)
			Expect(err).NotTo(HaveOccurred())

			contentStr := string(content)
			Expect(contentStr).To(ContainSubstring("kind: OCIRepository"))
			Expect(contentStr).To(ContainSubstring("kind: HelmRelease"))
		})

		It("should render templates/release.yaml with custom values", func() {
			customValues := map[string]interface{}{
				"replicaCount": 3,
				"image": map[string]string{
					"repository": "example.com/image",
					"tag":        "{{ .resources.resource1.tag }}",
				},
			}
			valuesJSON, err := json.Marshal(customValues)
			Expect(err).NotTo(HaveOccurred())

			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{},
				},
				Values: runtime.RawExtension{
					Raw: valuesJSON,
				},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			releasePath := filepath.Join(result.Dir, "templates", "release.yaml")
			content, err := os.ReadFile(releasePath)
			Expect(err).NotTo(HaveOccurred())

			contentStr := string(content)
			Expect(contentStr).To(ContainSubstring("replicaCount: 3"))
			Expect(contentStr).To(ContainSubstring("repository: example.com/image"))
			Expect(contentStr).To(ContainSubstring("tag: '{{ .resources.resource1.tag }}'"))
		})

		It("should create files with proper directory structure", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			// Check templates directory exists
			templatesDir := filepath.Join(result.Dir, "templates")
			info, err := os.Stat(templatesDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())

			// Check all files are regular files
			for _, fname := range []string{"Chart.yaml", "values.yaml", ".helmignore"} {
				filePath := filepath.Join(result.Dir, fname)
				info, err := os.Stat(filePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.IsDir()).To(BeFalse())
			}
		})

		It("should handle empty resources map", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
		})

		It("should handle multiple resources", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
							Repository: "oci://example.com/res1",
							Tag:        "v1.0.0",
						},
						"resource2": {
							Repository: "oci://example.com/res2",
							Tag:        "v2.0.0",
						},
						"resource3": {
							Repository: "oci://example.com/res3",
							Tag:        "v3.0.0",
						},
					},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			valuesPath := filepath.Join(result.Dir, "values.yaml")
			content, err := os.ReadFile(valuesPath)
			Expect(err).NotTo(HaveOccurred())

			contentStr := string(content)
			Expect(contentStr).To(ContainSubstring("resource1:"))
			Expect(contentStr).To(ContainSubstring("resource2:"))
			Expect(contentStr).To(ContainSubstring("resource3:"))
		})
	})

	Describe("RenderRelease cleanup", func() {
		It("should allow cleanup via RenderResult.Close()", func() {
			config := solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-release",
					Description: "Test Release Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			dirPath := result.Dir
			// Verify directory exists before cleanup
			_, err = os.Stat(dirPath)
			Expect(err).NotTo(HaveOccurred())

			// Cleanup
			err = result.Close()
			Expect(err).NotTo(HaveOccurred())

			// Verify directory is removed after cleanup
			_, err = os.Stat(dirPath)
			Expect(err).To(HaveOccurred())
			Expect(os.IsNotExist(err)).To(BeTrue())

			// Set result to nil so AfterEach doesn't try to clean up again
			result = nil
		})
	})
})
