// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"net/http"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RegisterHandler / UnregisterHandler", func() {
	BeforeEach(func() {
		UnregisterAllHandlers()
	})

	AfterEach(func() {
		UnregisterAllHandlers()
	})

	It("should panic when registering a nil handler", func() {
		Expect(func() {
			RegisterHandler("nil-handler", nil)
		}).To(PanicWith("cannot register nil handler"))
	})

	It("should panic when registering the same name twice", func() {
		fn := func(_ *solarv1alpha1.Registry, _ chan<- discovery.RepositoryEvent) http.Handler { return nil }
		RegisterHandler("duplicate", fn)

		Expect(func() {
			RegisterHandler("duplicate", fn)
		}).To(PanicWith(`handler "duplicate" already registered`))
	})

	It("should remove a registered handler", func() {
		fn := func(_ *solarv1alpha1.Registry, _ chan<- discovery.RepositoryEvent) http.Handler { return nil }
		RegisterHandler("removable", fn)

		Expect(UnregisterHandler("removable")).To(Succeed())
		Expect(registeredHandlers).NotTo(HaveKey("removable"))
	})

	It("should return an error when unregistering an unknown handler", func() {
		err := UnregisterHandler("does-not-exist")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`handler "does-not-exist" not registered`))
	})
})
