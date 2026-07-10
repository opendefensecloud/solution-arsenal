// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SplitRepository", func() {
	It("should parse a valid OCM repository", func() {
		base, comp, err := SplitRepository("myregistry/component-descriptors/example.com/mycomp")
		Expect(err).NotTo(HaveOccurred())
		Expect(base).To(Equal("myregistry"))
		Expect(comp).To(Equal("example.com/mycomp"))
	})

	It("should parse a valid OCM repository at the root level", func() {
		base, comp, err := SplitRepository("component-descriptors/example.com/mycomp")
		Expect(err).NotTo(HaveOccurred())
		Expect(base).To(Equal(""))
		Expect(comp).To(Equal("example.com/mycomp"))
	})

	It("should return ErrNotComponentDescriptor for a non-OCM repository", func() {
		_, _, err := SplitRepository("nginx")
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ErrNotComponentDescriptor)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("is not an ocm repository"))
	})

	It("should return ErrNotComponentDescriptor for a repo with multiple component-descriptors segments", func() {
		_, _, err := SplitRepository("a/component-descriptors/b/component-descriptors/c")
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ErrNotComponentDescriptor)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("has multiple 'component-descriptors' separators"))
	})

	It("should return ErrNotComponentDescriptor for a repo at the root level with multiple component-descriptors segments", func() {
		_, _, err := SplitRepository("component-descriptors/b/component-descriptors/c")
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ErrNotComponentDescriptor)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("has multiple 'component-descriptors' separators"))
	})
})

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
		Expect(len(result)).To(BeNumerically(">", 57))
		Expect(result).To(HavePrefix(strings.Repeat("a", 57) + "-"))
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

var _ = Describe("FromContextWithCreds", func() {
	It("should register credentials for a valid host:port and return a usable context", func() {
		octx, err := FromContextWithCreds(context.Background(), "registry.example.com:5000", &RegistryCredentials{
			Username: "user",
			Password: "pass",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(octx).NotTo(BeNil())
	})

	It("should error when hostname has no port", func() {
		_, err := FromContextWithCreds(context.Background(), "registry.example.com", &RegistryCredentials{
			Username: "user",
			Password: "pass",
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to split host and port"))
	})
})
