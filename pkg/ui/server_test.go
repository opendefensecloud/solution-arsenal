// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing/fstest"
	"time"

	"github.com/go-logr/logr"

	"go.opendefense.cloud/solar/pkg/ui/session"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const kubeconfigYAML = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: http://localhost:8080
contexts:
- name: ctx
  context:
    cluster: c
    user: u
current-context: ctx
users:
- name: u
  user: {}
`

// writeKubeconfig writes a minimal valid kubeconfig and returns its path.
func writeKubeconfig() string {
	p := filepath.Join(GinkgoT().TempDir(), "kubeconfig")
	Expect(os.WriteFile(p, []byte(kubeconfigYAML), 0o600)).To(Succeed())

	return p
}

// baseCfg is a working config: embedded static, noop auth, ephemeral port.
func baseCfg() Config {
	return Config{ListenAddr: "127.0.0.1:0", Kubeconfig: writeKubeconfig()}
}

var _ = Describe("NewServer", func() {
	It("builds a server serving the embedded SPA", func() {
		s, err := NewServer(baseCfg(), logr.Discard())
		Expect(err).NotTo(HaveOccurred())
		Expect(s).NotTo(BeNil())
	})

	It("builds a server proxying to the Vite dev server", func() {
		cfg := baseCfg()
		cfg.DevViteURL = "http://localhost:5173"
		s, err := NewServer(cfg, logr.Discard())
		Expect(err).NotTo(HaveOccurred())
		Expect(s).NotTo(BeNil())
	})

	It("fails on an invalid session key", func() {
		cfg := baseCfg()
		cfg.SessionKey = "zz" // not valid hex
		_, err := NewServer(cfg, logr.Discard())
		Expect(err).To(HaveOccurred())
	})

	It("fails on an unreadable kubeconfig", func() {
		cfg := baseCfg()
		cfg.Kubeconfig = filepath.Join(GinkgoT().TempDir(), "does-not-exist")
		_, err := NewServer(cfg, logr.Discard())
		Expect(err).To(HaveOccurred())
	})

	It("fails on an invalid dev-vite-url", func() {
		cfg := baseCfg()
		cfg.DevViteURL = ":foo" // missing scheme
		_, err := NewServer(cfg, logr.Discard())
		Expect(err).To(HaveOccurred())
	})

	It("fails when OIDC discovery fails", func() {
		oidc := httptest.NewServer(http.NotFoundHandler())
		DeferCleanup(oidc.Close)

		cfg := baseCfg()
		cfg.OIDCIssuer = oidc.URL
		_, err := NewServer(cfg, logr.Discard())
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("routing", func() {
	var handler http.Handler

	BeforeEach(func() {
		s, err := NewServer(baseCfg(), logr.Discard())
		Expect(err).NotTo(HaveOccurred())
		handler = s.server.Handler
	})

	It("serves /api/auth/me as unauthenticated without a session", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/auth/me", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Body.String()).To(ContainSubstring(`"authenticated":false`))
	})

	It("rejects an authenticated route without a session", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/profiles", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusUnauthorized))
	})

	It("clears the session on DELETE /api/auth/session", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/auth/session", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusNoContent))
	})

	It("redirects to / on logout", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/auth/logout", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusFound))
		Expect(rec.Header().Get("Location")).To(Equal("/"))
	})

	It("serves the SPA index for a non-API path", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/some/client/route", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
	})
})

var _ = Describe("authMiddleware", func() {
	var (
		store   *session.Store
		wrapped http.Handler
	)

	BeforeEach(func() {
		var err error
		store, err = session.NewStore("")
		Expect(err).NotTo(HaveOccurred())
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})
		wrapped = authMiddleware(store)(next)
	})

	It("returns 401 without a session", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusUnauthorized))
	})

	It("passes through to next with a valid session", func(ctx SpecContext) {
		setCookie := httptest.NewRecorder()
		store.Set(setCookie, &session.Data{Username: "alice"})

		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		req.AddCookie(setCookie.Result().Cookies()[0])
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusTeapot))
	})
})

var _ = Describe("spaFileServer", func() {
	var handler http.Handler

	BeforeEach(func() {
		fsys := fstest.MapFS{
			"index.html":    {Data: []byte("<!doctype html><title>app</title>")},
			"assets/app.js": {Data: []byte("console.log(1)")},
		}
		handler = spaFileServer(http.FS(fsys))
	})

	It("serves an existing asset directly", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/assets/app.js", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Body.String()).To(ContainSubstring("console.log"))
	})

	It("falls back to index.html for an unknown path", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/deep/client/route", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Body.String()).To(ContainSubstring("<title>app</title>"))
	})
})

var _ = Describe("Run", func() {
	It("starts and shuts down cleanly when the context is cancelled", func() {
		srv := &Server{
			cfg:    Config{ListenAddr: "127.0.0.1:0"},
			log:    logr.Discard(),
			server: &http.Server{Addr: "127.0.0.1:0", Handler: http.NewServeMux(), ReadHeaderTimeout: time.Second},
		}

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			defer GinkgoRecover()
			errCh <- srv.Run(ctx)
		}()

		cancel()
		Eventually(errCh, 5*time.Second).Should(Receive(BeNil()))
	})
})
