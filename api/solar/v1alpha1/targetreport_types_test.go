// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SplitRevision", func() {
	DescribeTable("splits a reported revision into version and digest",
		func(revision, version, digest string) {
			v, d := SplitRevision(revision)
			Expect(v).To(Equal(version))
			Expect(d).To(Equal(digest))
		},
		Entry("empty", "", "", ""),
		Entry("version only", "1.2.3", "1.2.3", ""),
		Entry("version and digest", "1.2.3+abc", "1.2.3", "abc"),
		Entry("rendered chart tag with digest", "v0.0.3-ab+0cc9a8446c95", "v0.0.3-ab", "0cc9a8446c95"),
	)
})
