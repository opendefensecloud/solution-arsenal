// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"

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
