// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"net/http"
	"net/http/httptest"

	"k8s.io/client-go/rest"

	"go.opendefense.cloud/solar/pkg/ui/session"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NoopProvider", func() {
	var (
		provider *NoopProvider
		store    *session.Store
	)

	BeforeEach(func() {
		provider = NewNoopProvider()
		var err error
		store, err = session.NewStore("")
		Expect(err).NotTo(HaveOccurred())
	})

	It("establishes a synthetic session on login and redirects", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/auth/login", nil)
		rec := httptest.NewRecorder()

		provider.HandleLogin(store)(rec, req)

		Expect(rec.Code).To(Equal(http.StatusFound))
		Expect(rec.Header().Get("Location")).To(Equal("/"))

		// The set cookie resolves to a noop session.
		req2 := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		req2.AddCookie(rec.Result().Cookies()[0])
		Expect(store.Get(req2).Username).To(Equal(noopUsername))
	})

	It("establishes a session on callback and redirects", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/auth/callback", nil)
		rec := httptest.NewRecorder()

		provider.HandleCallback(store)(rec, req)

		Expect(rec.Code).To(Equal(http.StatusFound))
		Expect(rec.Result().Cookies()).NotTo(BeEmpty())
	})

	It("returns the base config unchanged", func() {
		base := &rest.Config{Host: "https://example"}
		Expect(provider.WrapConfig(base, &session.Data{Username: "alice"})).To(BeIdenticalTo(base))
	})
})
