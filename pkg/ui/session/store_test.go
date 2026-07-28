// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// login stores data and returns the cookie that resolves to it.
func login(store *Store, data *Data) *http.Cookie {
	rec := httptest.NewRecorder()
	store.Set(rec, data)

	return rec.Result().Cookies()[0]
}

var _ = Describe("Store", func() {
	var store *Store

	BeforeEach(func() {
		var err error
		store, err = NewStore("")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("NewStore", func() {
		It("generates a random key when none is given", func() {
			s, err := NewStore("")
			Expect(err).NotTo(HaveOccurred())
			Expect(s).NotTo(BeNil())
		})

		It("accepts a valid 32-byte hex key", func() {
			s, err := NewStore(strings.Repeat("ab", 32))
			Expect(err).NotTo(HaveOccurred())
			Expect(s).NotTo(BeNil())
		})

		It("rejects a non-hex key", func() {
			_, err := NewStore("zz")
			Expect(err).To(HaveOccurred())
		})

		It("rejects a key of the wrong length", func() {
			_, err := NewStore("abcd")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Get / Set", func() {
		It("stores a session and retrieves it via the cookie", func(ctx SpecContext) {
			cookie := login(store, &Data{Username: "alice", Groups: []string{"devs"}})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)

			got := store.Get(req)
			Expect(got).NotTo(BeNil())
			Expect(got.Username).To(Equal("alice"))
			Expect(got.Groups).To(Equal([]string{"devs"}))
		})

		It("returns nil when there is no cookie", func(ctx SpecContext) {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			Expect(store.Get(req)).To(BeNil())
		})

		It("returns nil for an unknown cookie value", func(ctx SpecContext) {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: cookieName, Value: "nope"})
			Expect(store.Get(req)).To(BeNil())
		})

		It("returns a copy so callers cannot mutate stored state", func(ctx SpecContext) {
			cookie := login(store, &Data{Username: "alice"})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)

			got := store.Get(req)
			got.Username = "mutated"

			Expect(store.Get(req).Username).To(Equal("alice"))
		})
	})

	Describe("SetImpersonation", func() {
		It("sets the preview identity and clears the namespace cache", func(ctx SpecContext) {
			cookie := login(store, &Data{Username: "admin", CanListAllNamespaces: new(true)})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)

			Expect(store.SetImpersonation(req, "bob", []string{"team"})).To(BeTrue())

			got := store.Get(req)
			Expect(got.ImpersonatingAs).To(Equal("bob"))
			Expect(got.ImpersonatingGroups).To(Equal([]string{"team"}))
			Expect(got.CanListAllNamespaces).To(BeNil())
		})

		It("returns false without a cookie", func(ctx SpecContext) {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			Expect(store.SetImpersonation(req, "bob", nil)).To(BeFalse())
		})

		It("returns false for an unknown session", func(ctx SpecContext) {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: cookieName, Value: "nope"})
			Expect(store.SetImpersonation(req, "bob", nil)).To(BeFalse())
		})
	})

	Describe("ClearImpersonation", func() {
		It("removes the preview identity and clears the namespace cache", func(ctx SpecContext) {
			cookie := login(store, &Data{Username: "admin", ImpersonatingAs: "bob", CanListAllNamespaces: new(true)})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)

			Expect(store.ClearImpersonation(req)).To(BeTrue())

			got := store.Get(req)
			Expect(got.ImpersonatingAs).To(BeEmpty())
			Expect(got.ImpersonatingGroups).To(BeNil())
			Expect(got.CanListAllNamespaces).To(BeNil())
		})

		It("returns false without a cookie", func(ctx SpecContext) {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			Expect(store.ClearImpersonation(req)).To(BeFalse())
		})
	})

	Describe("capability caches", func() {
		It("caches CanImpersonate on the session", func(ctx SpecContext) {
			cookie := login(store, &Data{Username: "alice"})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)

			Expect(store.SetCanImpersonate(req, true)).To(BeTrue())
			got := store.Get(req)
			Expect(got.CanImpersonate).NotTo(BeNil())
			Expect(*got.CanImpersonate).To(BeTrue())
		})

		It("caches CanListAllNamespaces on the session", func(ctx SpecContext) {
			cookie := login(store, &Data{Username: "alice"})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)

			Expect(store.SetCanListAllNamespaces(req, true)).To(BeTrue())
			got := store.Get(req)
			Expect(got.CanListAllNamespaces).NotTo(BeNil())
			Expect(*got.CanListAllNamespaces).To(BeTrue())
		})

		It("SetCanImpersonate returns false without a cookie", func(ctx SpecContext) {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			Expect(store.SetCanImpersonate(req, true)).To(BeFalse())
		})

		It("SetCanListAllNamespaces returns false without a cookie", func(ctx SpecContext) {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			Expect(store.SetCanListAllNamespaces(req, true)).To(BeFalse())
		})
	})

	Describe("Clear", func() {
		It("deletes the session and expires the cookie", func(ctx SpecContext) {
			cookie := login(store, &Data{Username: "alice"})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)

			rec := httptest.NewRecorder()
			store.Clear(rec, req)

			Expect(store.Get(req)).To(BeNil())
			cleared := rec.Result().Cookies()[0]
			Expect(cleared.Name).To(Equal(cookieName))
			Expect(cleared.MaxAge).To(BeNumerically("<", 0))
		})

		It("still expires the cookie when there is no session", func(ctx SpecContext) {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			store.Clear(rec, req)

			Expect(rec.Result().Cookies()).NotTo(BeEmpty())
		})
	})

	Describe("OIDC state cookie", func() {
		It("round-trips the state value", func(ctx SpecContext) {
			rec := httptest.NewRecorder()
			store.SetState(rec, "xyz")

			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(rec.Result().Cookies()[0])
			Expect(store.GetState(req)).To(Equal("xyz"))
		})

		It("returns empty when the state cookie is absent", func(ctx SpecContext) {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			Expect(store.GetState(req)).To(BeEmpty())
		})

		It("expires the state cookie on clear", func() {
			rec := httptest.NewRecorder()
			store.ClearState(rec)

			cleared := rec.Result().Cookies()[0]
			Expect(cleared.Name).To(Equal(stateCookieName))
			Expect(cleared.MaxAge).To(BeNumerically("<", 0))
		})
	})
})
