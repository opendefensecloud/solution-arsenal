// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package naming

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SanitizeName", func() {
	It("should lowercase and replace non-alphanumeric runs with a single dash", func() {
		Expect(SanitizeName("My Cool_Component!!v1")).To(Equal("my-cool-component-v1"))
	})

	It("should trim leading and trailing dashes", func() {
		Expect(SanitizeName("--hello--")).To(Equal("hello"))
	})

	It("should truncate to 63 characters and trim a trailing dash left by truncation", func() {
		input := strings.Repeat("a", 62) + "-" + strings.Repeat("b", 10)
		result := SanitizeName(input)
		Expect(len(result)).To(BeNumerically("<=", 63))
		Expect(result).NotTo(HaveSuffix("-"))
	})

	It("should return empty string for input with no alphanumeric characters", func() {
		Expect(SanitizeName("!!!")).To(BeEmpty())
	})
})

var _ = Describe("SanitizeWithHash", func() {
	It("should return the sanitized name unchanged when short enough", func() {
		Expect(SanitizeWithHash("my-component")).To(Equal("my-component"))
	})

	It("should append a hash suffix when the sanitized name is too long", func() {
		input := strings.Repeat("a", 100)
		result := SanitizeWithHash(input)
		Expect(result).To(HavePrefix(strings.Repeat("a", 54) + "-"))
	})

	It("should keep the result within the 63 character label limit", func() {
		for _, n := range []int{57, 63, 64, 100, 300} {
			Expect(len(SanitizeWithHash(strings.Repeat("a", n)))).To(BeNumerically("<=", 63), "input length %d", n)
		}
	})

	It("should be deterministic for the same input", func() {
		input := strings.Repeat("x", 100)
		Expect(SanitizeWithHash(input)).To(Equal(SanitizeWithHash(input)))
	})

	It("should differ for different inputs that share the same truncated prefix", func() {
		base := strings.Repeat("a", 57)
		first := SanitizeWithHash(base + "-one")
		second := SanitizeWithHash(base + "-two")
		Expect(first).NotTo(Equal(second))
	})
})

var _ = Describe("ComponentVersionName", func() {
	It("should combine component and version into a sanitized name", func() {
		Expect(ComponentVersionName("My.Component", "1.0.0")).To(Equal("my-component-1-0-0"))
	})
})
