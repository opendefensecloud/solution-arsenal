// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RenderHydratedTarget", func() {
	var (
		result *RenderResult
		err    error
	)

	AfterEach(func() {
		if result != nil {
			err := result.Close()
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("RenderHydratedTarget with valid HydratedTargetConfig", func() {
		It("should render without errors", func() {
			config := HydratedTargetConfig{
				Chart: ChartConfig{
					Name:        "test-hydrated-target",
					Description: "Test HydratedTarget Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input:  HydratedTargetInput{},
				Values: json.RawMessage(`{}`),
			}

			result, err = RenderHydratedTarget(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Dir).NotTo(BeEmpty())
		})
	})
})
