// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/pkg/chart/common"
	chartutil "helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/engine"
	"k8s.io/apimachinery/pkg/runtime"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func validBootstrapConfig() solarv1alpha1.BootstrapConfig {
	return solarv1alpha1.BootstrapConfig{
		Chart: solarv1alpha1.ChartConfig{
			Name:        "test-bootstrap",
			Description: "Test Bootstrap Chart",
			Version:     "1.0.0",
			AppVersion:  "1.0.0",
		},
		Input: solarv1alpha1.BootstrapInput{
			Releases: map[string]solarv1alpha1.ResourceAccess{
				"foo": {
					Repository: "example.com/foo",
					Tag:        "^1.0",
				},
			},
			Userdata: runtime.RawExtension{
				Raw: []byte(`{"foo": "bar"}`),
			},
		},
	}
}

var _ = Describe("RenderBootstrap", func() {
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

	Describe("Render Bootstrap with valid BootstrapConfig", func() {
		It("should render without errors", func() {
			config := validBootstrapConfig()
			result, err = RenderBootstrap(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Dir).NotTo(BeEmpty())
		})

		It("should create a temporary directory", func() {
			config := validBootstrapConfig()
			result, err = RenderBootstrap(config)
			Expect(err).NotTo(HaveOccurred())

			// Verify directory exists
			info, err := os.Stat(result.Dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})

		It("should render Chart.yaml with correct template values", func() {
			config := validBootstrapConfig()
			result, err = RenderBootstrap(config)
			Expect(err).NotTo(HaveOccurred())

			chartPath := filepath.Join(result.Dir, "Chart.yaml")
			content, err := os.ReadFile(chartPath)
			Expect(err).NotTo(HaveOccurred())

			contentStr := string(content)
			Expect(contentStr).To(ContainSubstring("name: test-bootstrap"))
			Expect(contentStr).To(ContainSubstring("description: Test Bootstrap Chart"))
			Expect(contentStr).To(ContainSubstring("version: 1.0.0"))
			Expect(contentStr).To(ContainSubstring("appVersion: 1.0.0"))
			Expect(contentStr).To(ContainSubstring("apiVersion: v2"))
		})

		It("should render values.yaml with correct template values", func() {
			config := validBootstrapConfig()
			result, err = RenderBootstrap(config)
			Expect(err).NotTo(HaveOccurred())

			valuesPath := filepath.Join(result.Dir, "values.yaml")
			content, err := os.ReadFile(valuesPath)
			Expect(err).NotTo(HaveOccurred())

			contentStr := string(content)
			Expect(contentStr).To(ContainSubstring("repository: example.com/foo"))
			Expect(contentStr).To(ContainSubstring("tag: ^1.0"))
		})
	})

	Describe("Helm template validation for bootstrap release.yaml", func() {
		renderAndTemplate := func(input solarv1alpha1.BootstrapInput) (map[string]string, error) {
			config := solarv1alpha1.BootstrapConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-bootstrap",
					Description: "Test Bootstrap Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: input,
			}

			renderResult, err := RenderBootstrap(config)
			if err != nil {
				return nil, err
			}
			defer func() {
				Expect(renderResult.Close()).To(Succeed())
			}()

			chrt, err := loader.Load(renderResult.Dir)
			if err != nil {
				return nil, err
			}

			options := common.ReleaseOptions{
				Name:      "my-release",
				Namespace: "my-namespace",
				Revision:  1,
				IsInstall: true,
			}
			vals, err := chartutil.ToRenderValues(chrt, nil, options, nil)
			if err != nil {
				return nil, err
			}

			return engine.Render(chrt, vals)
		}

		releaseYAML := func(rendered map[string]string) string {
			const suffix = "templates/release.yaml"
			var matches []string
			for k, v := range rendered {
				if strings.HasSuffix(k, suffix) {
					matches = append(matches, v)
				}
			}
			Expect(matches).To(HaveLen(1), "expected exactly one template ending in %s", suffix)

			return matches[0]
		}

		It("insecure release renders OCIRepository with insecure: true", func() {
			rendered, err := renderAndTemplate(solarv1alpha1.BootstrapInput{
				Releases: map[string]solarv1alpha1.ResourceAccess{
					"my-app": {
						Repository: "registry.example.com/charts/my-app",
						Tag:        "v1.0.0",
						Insecure:   true,
					},
				},
				Userdata: runtime.RawExtension{Raw: []byte(`{}`)},
			})
			Expect(err).NotTo(HaveOccurred())
			yaml := releaseYAML(rendered)
			Expect(yaml).To(ContainSubstring("insecure: true"))
			Expect(yaml).To(ContainSubstring("url: oci://registry.example.com/charts/my-app"))
		})

		It("secure release omits insecure field", func() {
			rendered, err := renderAndTemplate(solarv1alpha1.BootstrapInput{
				Releases: map[string]solarv1alpha1.ResourceAccess{
					"my-app": {
						Repository: "registry.example.com/charts/my-app",
						Tag:        "v1.0.0",
						Insecure:   false,
					},
				},
				Userdata: runtime.RawExtension{Raw: []byte(`{}`)},
			})
			Expect(err).NotTo(HaveOccurred())
			yaml := releaseYAML(rendered)
			Expect(yaml).NotTo(ContainSubstring("insecure"))
		})
	})
})
