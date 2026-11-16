// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
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

var _ = Describe("OCMCredentials", func() {
	It("should convert registry credentials into OCM credentials", func() {
		creds := OCMCredentials(&RegistryCredentials{Username: "user", Password: "pass"})
		Expect(creds).NotTo(BeNil())
		Expect(creds.Username).To(Equal("user"))
		Expect(creds.Password).To(Equal("pass"))
	})

	It("should map nil credentials to nil, meaning anonymous access", func() {
		Expect(OCMCredentials(nil)).To(BeNil())
	})
})
