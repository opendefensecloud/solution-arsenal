// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SanitizeDigestLabel", func() {
	It("should strip the algorithm prefix", func() {
		Expect(SanitizeDigestLabel("sha256:abcdef1234567890")).To(Equal("abcdef1234567890"))
	})

	It("should strip sha512 prefix", func() {
		Expect(SanitizeDigestLabel("sha512:abcdef1234567890")).To(Equal("abcdef1234567890"))
	})

	It("should return empty string for empty input", func() {
		Expect(SanitizeDigestLabel("")).To(BeEmpty())
	})

	It("should truncate to 63 characters", func() {
		long := "sha256:" + strings.Repeat("a", 100)
		result := SanitizeDigestLabel(long)
		Expect(result).To(HaveLen(63))
	})

	It("should return hex as-is when no prefix and under 63 chars", func() {
		Expect(SanitizeDigestLabel("abcdef1234567890")).To(Equal("abcdef1234567890"))
	})

	It("should handle a realistic sha256 digest", func() {
		// sha256 hex is 64 chars, truncated to 63
		digest := "sha256:40bac3123555936fd4aa8260a853669283fa8d64be8f665ba9d60fd9f7d7df3b"
		result := SanitizeDigestLabel(digest)
		Expect(result).To(Equal("40bac3123555936fd4aa8260a853669283fa8d64be8f665ba9d60fd9f7d7df3"))
		Expect(len(result)).To(BeNumerically("<=", 63))
	})
})
