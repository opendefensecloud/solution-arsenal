// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/runtime"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func validHydratedTargetConfig() solarv1alpha1.HydratedTargetConfig {
	return solarv1alpha1.HydratedTargetConfig{
		Chart: solarv1alpha1.ChartConfig{
			Name:        "test-hydrated-target",
			Description: "Test HydratedTarget Chart",
			Version:     "1.0.0",
			AppVersion:  "1.0.0",
		},
		Input: solarv1alpha1.HydratedTargetInput{
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

var _ = Describe("RenderHydratedTarget", func() {
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

	Describe("Render HydratedTarget with valid HydratedTargetConfig", func() {
		It("should render without errors", func() {
			config := validHydratedTargetConfig()
			result, err = RenderHydratedTarget(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Dir).NotTo(BeEmpty())
		})

		It("should create a temporary directory", func() {
			config := validHydratedTargetConfig()
			result, err = RenderHydratedTarget(config)
			Expect(err).NotTo(HaveOccurred())

			// Verify directory exists
			info, err := os.Stat(result.Dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})

		It("should render Chart.yaml with correct template values", func() {
			config := validHydratedTargetConfig()
			result, err = RenderHydratedTarget(config)
			Expect(err).NotTo(HaveOccurred())

			chartPath := filepath.Join(result.Dir, "Chart.yaml")
			content, err := os.ReadFile(chartPath)
			Expect(err).NotTo(HaveOccurred())

			contentStr := string(content)
			Expect(contentStr).To(ContainSubstring("name: test-hydrated-target"))
			Expect(contentStr).To(ContainSubstring("description: Test HydratedTarget Chart"))
			Expect(contentStr).To(ContainSubstring("version: 1.0.0"))
			Expect(contentStr).To(ContainSubstring("appVersion: 1.0.0"))
			Expect(contentStr).To(ContainSubstring("apiVersion: v2"))
		})

		It("should render values.yaml with correct template values", func() {
			config := validHydratedTargetConfig()
			result, err = RenderHydratedTarget(config)
			Expect(err).NotTo(HaveOccurred())

			valuesPath := filepath.Join(result.Dir, "values.yaml")
			content, err := os.ReadFile(valuesPath)
			Expect(err).NotTo(HaveOccurred())

			contentStr := string(content)
			Expect(contentStr).To(ContainSubstring("repository: example.com/foo"))
			Expect(contentStr).To(ContainSubstring("tag: ^1.0"))
		})
	})
})
