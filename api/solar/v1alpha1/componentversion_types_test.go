// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ComponentVersion.OCMRef", func() {
	newCV := func(scheme, registry, repository, name, tag string) *ComponentVersion {
		return &ComponentVersion{Spec: ComponentVersionSpec{
			Scheme: scheme, Registry: registry, Repository: repository, ComponentName: name, Tag: tag,
		}}
	}

	It("builds the ref from a single-level namespace", func() {
		cv := newCV("https", "ghcr.io", "opendefensecloud/opendefense.cloud/arc", "opendefense.cloud/arc", "v0.2.0")
		Expect(cv.OCMRef()).To(Equal("https://ghcr.io/opendefensecloud//opendefense.cloud/arc:v0.2.0"))
	})

	It("builds the ref from a multi-level namespace", func() {
		cv := newCV("http", "localhost:5000", "a/b/c/opendefense.cloud/arc", "opendefense.cloud/arc", "v1.0.0")
		Expect(cv.OCMRef()).To(Equal("http://localhost:5000/a/b/c//opendefense.cloud/arc:v1.0.0"))
	})

	It("yields an empty namespace for a component at the registry root", func() {
		cv := newCV("http", "localhost:5000", "opendefense.cloud/arc", "opendefense.cloud/arc", "v1.0.0")
		Expect(cv.OCMRef()).To(Equal("http://localhost:5000///opendefense.cloud/arc:v1.0.0"))
	})

	It("returns empty when ComponentName is unset", func() {
		cv := newCV("https", "ghcr.io", "opendefensecloud/opendefense.cloud/arc", "", "v0.2.0")
		Expect(cv.OCMRef()).To(BeEmpty())
	})
	It("returns empty when scheme or registry is unset", func() {
		Expect(newCV("", "ghcr.io", "x/podinfo", "podinfo", "1.0.0").OCMRef()).To(BeEmpty())
		Expect(newCV("https", "", "x/podinfo", "podinfo", "1.0.0").OCMRef()).To(BeEmpty())
	})
	It("returns empty when repository does not end with the component name", func() {
		Expect(newCV("https", "ghcr.io", "weird/path/mismatch", "opendefense.cloud/arc", "v1").OCMRef()).To(BeEmpty())
		Expect(newCV("https", "ghcr.io", "x/myopendefense.cloud/arc", "opendefense.cloud/arc", "v1").OCMRef()).To(BeEmpty())
	})
})
