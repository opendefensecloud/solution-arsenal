// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Component.OCMRef", func() {
	newComponent := func(scheme, registry, repository, name string) *Component {
		return &Component{
			Spec: ComponentSpec{
				Scheme:     scheme,
				Registry:   registry,
				Repository: repository,
				Name:       name,
			},
		}
	}

	It("splits the namespace off the repository", func() {
		c := newComponent("https", "ghcr.io", "opendefensecloud/opendefense.cloud/arc", "opendefense.cloud/arc")

		Expect(c.OCMRef("v0.2.0")).To(Equal("https://ghcr.io/opendefensecloud//opendefense.cloud/arc:v0.2.0"))
	})

	It("handles a multi-segment namespace", func() {
		c := newComponent("http", "localhost:5000", "a/b/c/opendefense.cloud/arc", "opendefense.cloud/arc")

		Expect(c.OCMRef("v1.0.0")).To(Equal("http://localhost:5000/a/b/c//opendefense.cloud/arc:v1.0.0"))
	})

	It("yields an empty namespace for a component at the registry root", func() {
		c := newComponent("http", "localhost:5000", "opendefense.cloud/arc", "opendefense.cloud/arc")

		Expect(c.OCMRef("v1.0.0")).To(Equal("http://localhost:5000///opendefense.cloud/arc:v1.0.0"))
	})

	It("returns empty when the raw OCM name is unset", func() {
		c := newComponent("https", "ghcr.io", "opendefensecloud/opendefense.cloud/arc", "")

		Expect(c.OCMRef("v0.2.0")).To(BeEmpty())
	})
})
