// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"resource1": {
							Repository: "oci://example.com/resource1",
							Tag:        "v1.0.0",
						},
					},
					Entrypoint: solarv1alpha1.Entrypoint{
						ResourceName: "resource1",
						Type:         solarv1alpha1.EntrypointTypeHelm,
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
			customValues := map[string]any{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
		})

		It("should handle target namespace", func() {
			checkHelmRelease := func(content []unstructured.Unstructured, check types.GomegaMatcher) {
				GinkgoHelper()
				helmreleases := 0
				for _, manifest := range content {
					if manifest.GetAPIVersion() != "helm.toolkit.fluxcd.io/v2" ||
						manifest.GetKind() != "HelmRelease" {
						continue
					}
					helmreleases += 1
					Expect(manifest.Object).To(check)
				}
				Expect(helmreleases).To(BeNumerically(">", 0), "helmrelease was not rendered")
			}

			By("checking if spec.targetNamespace was set when a TargetNamespace was given")
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
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"foo": {
							Repository: "example.com/my-chart",
							Tag:        "1.0.0",
						},
					},
					Entrypoint: solarv1alpha1.Entrypoint{
						ResourceName: "foo",
						Type:         solarv1alpha1.EntrypointTypeHelm,
					},
				},
				Values:          runtime.RawExtension{},
				TargetNamespace: "my-namespace",
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			manifests, err := helmTemplate("foo", "default", result.Dir)
			Expect(err).NotTo(HaveOccurred())

			checkHelmRelease(manifests,
				HaveKeyWithValue("spec",
					HaveKeyWithValue("targetNamespace", "my-namespace"),
				))

			By("checking if spec.targetNamespace is not set when a TargetNamespace was not given")
			config = solarv1alpha1.ReleaseConfig{
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
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"foo": {
							Repository: "example.com/my-chart",
							Tag:        "1.0.0",
						},
					},
					Entrypoint: solarv1alpha1.Entrypoint{
						ResourceName: "foo",
						Type:         solarv1alpha1.EntrypointTypeHelm,
					},
				},
				Values: runtime.RawExtension{},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			manifests, err = helmTemplate("foo", "default", result.Dir)
			Expect(err).NotTo(HaveOccurred())

			checkHelmRelease(manifests,
				HaveKeyWithValue("spec",
					Not(HaveKey("targetNamespace")),
				))
		})

		It("should render ConfigMap and valuesFrom when ValuesTemplate is present", func() {
			valuesTemplate := "image:\n  repository: registry.example.com/nginx\n  tag: \"1.25.0\""
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
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"my-chart": {
							Repository: "oci://example.com/my-chart",
							Tag:        "v1.0.0",
							Helm: &solarv1alpha1.HelmResourceMetadata{
								Name:           "my-chart",
								Version:        "1.0.0",
								ValuesTemplate: &valuesTemplate,
							},
						},
					},
					Entrypoint: solarv1alpha1.Entrypoint{
						ResourceName: "my-chart",
						Type:         solarv1alpha1.EntrypointTypeHelm,
					},
				},
				Values: runtime.RawExtension{
					Raw: []byte(`{"replicaCount": 3}`),
				},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			manifests, err := helmTemplate("bar", "test-ns", result.Dir)
			Expect(err).NotTo(HaveOccurred())

			// Find the ConfigMap
			var configMap *unstructured.Unstructured
			var helmRelease *unstructured.Unstructured
			for i := range manifests {
				switch manifests[i].GetKind() {
				case "ConfigMap":
					configMap = &manifests[i]
				case "HelmRelease":
					helmRelease = &manifests[i]
				}
			}

			Expect(configMap).NotTo(BeNil(), "ConfigMap should be rendered")
			Expect(configMap.GetName()).To(Equal("bar-test-component-values"))

			data, found, err := unstructured.NestedStringMap(configMap.Object, "data")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(data).To(HaveKey("values.yaml"))
			Expect(data["values.yaml"]).To(ContainSubstring("repository: registry.example.com/nginx"))
			Expect(data["values.yaml"]).To(ContainSubstring("tag: \"1.25.0\""))

			Expect(helmRelease).NotTo(BeNil(), "HelmRelease should be rendered")
			valuesFrom, found, err := unstructured.NestedSlice(helmRelease.Object, "spec", "valuesFrom")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(valuesFrom).To(HaveLen(1))
			vf := valuesFrom[0].(map[string]any)
			Expect(vf["kind"]).To(Equal("ConfigMap"))
			Expect(vf["name"]).To(Equal("bar-test-component-values"))
			Expect(vf["valuesKey"]).To(Equal("values.yaml"))
		})

		It("should not render ConfigMap or valuesFrom when ValuesTemplate is absent", func() {
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
					Resources: map[string]solarv1alpha1.ResourceAccess{
						"my-chart": {
							Repository: "oci://example.com/my-chart",
							Tag:        "v1.0.0",
						},
					},
					Entrypoint: solarv1alpha1.Entrypoint{
						ResourceName: "my-chart",
						Type:         solarv1alpha1.EntrypointTypeHelm,
					},
				},
				Values: runtime.RawExtension{
					Raw: []byte(`{"replicaCount": 1}`),
				},
			}

			result, err = RenderRelease(config)
			Expect(err).NotTo(HaveOccurred())

			manifests, err := helmTemplate("foo", "default", result.Dir)
			Expect(err).NotTo(HaveOccurred())

			for _, m := range manifests {
				Expect(m.GetKind()).NotTo(Equal("ConfigMap"), "no ConfigMap should be rendered without ValuesTemplate")
			}

			// HelmRelease should not have valuesFrom
			for _, m := range manifests {
				if m.GetKind() == "HelmRelease" {
					_, found, _ := unstructured.NestedSlice(m.Object, "spec", "valuesFrom")
					Expect(found).To(BeFalse(), "HelmRelease should not have valuesFrom without ValuesTemplate")
				}
			}
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
