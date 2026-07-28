// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/rest"

	"go.opendefense.cloud/solar/pkg/ui/session"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeIDP is a minimal OIDC provider that issues a verifiable id_token, so the
// full callback path (token exchange → JWT verification → claims) can be tested.
func fakeIDP(clientID string) *httptest.Server {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	b64 := base64.RawURLEncoding.EncodeToString

	mux := http.NewServeMux()
	var srv *httptest.Server

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/auth",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/keys",
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "test", "alg": "RS256", "use": "sig",
			"n": b64(key.PublicKey.N.Bytes()),
			"e": b64(big.NewInt(int64(key.PublicKey.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		now := time.Now().Unix()
		header := b64([]byte(`{"alg":"RS256","typ":"JWT","kid":"test"}`))
		payload := b64(fmt.Appendf(nil,
			`{"iss":%q,"aud":%q,"sub":"user-1","exp":%d,"iat":%d,"email":"alice@example.com","groups":["devs"]}`,
			srv.URL, clientID, now+3600, now))
		digest := sha256.Sum256([]byte(header + "." + payload))
		sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
		idToken := header + "." + payload + "." + b64(sig)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": idToken,
		})
	})

	srv = httptest.NewServer(mux)

	return srv
}

// fakeIssuer serves a minimal, self-consistent OIDC discovery document so
// oidc.NewProvider succeeds without a real IdP.
func fakeIssuer() *httptest.Server {
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/auth",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/keys",
		})
	})
	srv = httptest.NewServer(mux)

	return srv
}

// writeCertPEM writes a self-signed certificate to a temp file and returns its path.
func writeCertPEM() string {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	p := filepath.Join(GinkgoT().TempDir(), "ca.pem")
	Expect(os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)).To(Succeed())

	return p
}

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

var _ = Describe("newHTTPClient", func() {
	It("returns a plain client when no CA file is set", func() {
		c, err := newHTTPClient("")
		Expect(err).NotTo(HaveOccurred())
		Expect(c).NotTo(BeNil())
	})

	It("errors when the CA file is missing", func() {
		_, err := newHTTPClient(filepath.Join(GinkgoT().TempDir(), "nope.pem"))
		Expect(err).To(HaveOccurred())
	})

	It("errors when the CA file has no certificates", func() {
		p := filepath.Join(GinkgoT().TempDir(), "bad.pem")
		Expect(os.WriteFile(p, []byte("not a certificate"), 0o600)).To(Succeed())
		_, err := newHTTPClient(p)
		Expect(err).To(HaveOccurred())
	})

	It("builds a client with a custom root pool for a valid CA", func() {
		c, err := newHTTPClient(writeCertPEM())
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Transport).NotTo(BeNil())
	})
})

var _ = Describe("NewOIDCProvider", func() {
	It("discovers a valid issuer and defaults to token mode", func() {
		issuer := fakeIssuer()
		DeferCleanup(issuer.Close)

		p, err := NewOIDCProvider(OIDCConfig{Issuer: issuer.URL, ClientID: "solar"})
		Expect(err).NotTo(HaveOccurred())
		Expect(p.authMode).To(Equal(AuthModeToken))
	})

	It("honours an explicit auth mode", func() {
		issuer := fakeIssuer()
		DeferCleanup(issuer.Close)

		p, err := NewOIDCProvider(OIDCConfig{Issuer: issuer.URL, ClientID: "solar", AuthMode: AuthModeImpersonate})
		Expect(err).NotTo(HaveOccurred())
		Expect(p.authMode).To(Equal(AuthModeImpersonate))
	})

	It("fails when discovery is unavailable", func() {
		issuer := httptest.NewServer(http.NotFoundHandler())
		DeferCleanup(issuer.Close)

		_, err := NewOIDCProvider(OIDCConfig{Issuer: issuer.URL})
		Expect(err).To(HaveOccurred())
	})

	It("fails when the CA cert cannot be read", func() {
		_, err := NewOIDCProvider(OIDCConfig{Issuer: "https://example", CACertFile: "/does/not/exist"})
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("OIDCProvider handlers", func() {
	var (
		provider *OIDCProvider
		store    *session.Store
	)

	BeforeEach(func() {
		issuer := fakeIssuer()
		DeferCleanup(issuer.Close)
		var err error
		provider, err = NewOIDCProvider(OIDCConfig{Issuer: issuer.URL, ClientID: "solar", RedirectURL: "http://app/cb"})
		Expect(err).NotTo(HaveOccurred())
		store, err = session.NewStore("")
		Expect(err).NotTo(HaveOccurred())
	})

	It("redirects to the provider and sets a state cookie on login", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/auth/login", nil)
		rec := httptest.NewRecorder()

		provider.HandleLogin(store)(rec, req)

		Expect(rec.Code).To(Equal(http.StatusFound))
		Expect(rec.Header().Get("Location")).To(ContainSubstring("/auth?"))
		Expect(rec.Result().Cookies()).NotTo(BeEmpty())
	})

	It("rejects a callback with no code", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/auth/callback", nil)
		rec := httptest.NewRecorder()

		provider.HandleCallback(store)(rec, req)

		Expect(rec.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects a callback with a missing state cookie", func(ctx SpecContext) {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/auth/callback?code=abc&state=x", nil)
		rec := httptest.NewRecorder()

		provider.HandleCallback(store)(rec, req)

		Expect(rec.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects a callback whose state does not match", func(ctx SpecContext) {
		setState := httptest.NewRecorder()
		store.SetState(setState, "expected")

		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/auth/callback?code=abc&state=wrong", nil)
		req.AddCookie(setState.Result().Cookies()[0])
		rec := httptest.NewRecorder()

		provider.HandleCallback(store)(rec, req)

		Expect(rec.Code).To(Equal(http.StatusBadRequest))
	})

	It("returns 500 when the token exchange fails", func(ctx SpecContext) {
		// provider's issuer has no /token endpoint, so Exchange gets a 404.
		setState := httptest.NewRecorder()
		store.SetState(setState, "s")

		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/auth/callback?code=abc&state=s", nil)
		req.AddCookie(setState.Result().Cookies()[0])
		rec := httptest.NewRecorder()

		provider.HandleCallback(store)(rec, req)

		Expect(rec.Code).To(Equal(http.StatusInternalServerError))
	})
})

var _ = Describe("OIDCProvider callback success", func() {
	It("exchanges the code, verifies the id_token, and establishes a session", func(ctx SpecContext) {
		idp := fakeIDP("solar")
		DeferCleanup(idp.Close)

		provider, err := NewOIDCProvider(OIDCConfig{Issuer: idp.URL, ClientID: "solar", RedirectURL: "http://app/cb"})
		Expect(err).NotTo(HaveOccurred())
		store, err := session.NewStore("")
		Expect(err).NotTo(HaveOccurred())

		setState := httptest.NewRecorder()
		store.SetState(setState, "s")
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/auth/callback?code=abc&state=s", nil)
		req.AddCookie(setState.Result().Cookies()[0])
		rec := httptest.NewRecorder()

		provider.HandleCallback(store)(rec, req)

		Expect(rec.Code).To(Equal(http.StatusFound))
		Expect(rec.Header().Get("Location")).To(Equal("/"))

		// The session cookie resolves to the verified user's claims.
		follow := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		for _, c := range rec.Result().Cookies() {
			follow.AddCookie(c)
		}
		sess := store.Get(follow)
		Expect(sess).NotTo(BeNil())
		Expect(sess.Username).To(Equal("alice@example.com"))
		Expect(sess.Groups).To(Equal([]string{"devs"}))
	})
})

var _ = Describe("OIDCProvider.WrapConfig", func() {
	baseConfig := func() *rest.Config {
		return &rest.Config{
			Host:            "https://k8s",
			BearerTokenFile: "/token",
			TLSClientConfig: rest.TLSClientConfig{
				CertData: []byte("cert"),
				CertFile: "/cert",
				KeyData:  []byte("key"),
				KeyFile:  "/key",
			},
		}
	}

	It("forwards the id_token as a bearer token and clears client certs", func() {
		p := &OIDCProvider{authMode: AuthModeToken}
		cfg := p.WrapConfig(baseConfig(), &session.Data{Username: "alice", IDToken: "tok"})

		Expect(cfg.BearerToken).To(Equal("tok"))
		Expect(cfg.CertData).To(BeNil())
		Expect(cfg.CertFile).To(BeEmpty())
		Expect(cfg.KeyData).To(BeNil())
		Expect(cfg.KeyFile).To(BeEmpty())
		Expect(cfg.BearerTokenFile).To(BeEmpty())
	})

	It("uses impersonation in impersonate mode", func() {
		p := &OIDCProvider{authMode: AuthModeImpersonate}
		cfg := p.WrapConfig(baseConfig(), &session.Data{Username: "alice", Groups: []string{"devs"}})

		Expect(cfg.Impersonate.UserName).To(Equal("alice"))
		Expect(cfg.Impersonate.Groups).To(Equal([]string{"devs"}))
	})

	It("prefers session impersonation over the auth mode", func() {
		p := &OIDCProvider{authMode: AuthModeToken}
		cfg := p.WrapConfig(baseConfig(), &session.Data{
			Username:            "admin",
			IDToken:             "tok",
			ImpersonatingAs:     "bob",
			ImpersonatingGroups: []string{"team"},
		})

		Expect(cfg.Impersonate.UserName).To(Equal("bob"))
		Expect(cfg.Impersonate.Groups).To(Equal([]string{"team"}))
		Expect(cfg.BearerToken).To(BeEmpty()) // token path not taken
	})

	It("does not mutate the base config", func() {
		base := baseConfig()
		p := &OIDCProvider{authMode: AuthModeToken}
		_ = p.WrapConfig(base, &session.Data{IDToken: "tok"})

		Expect(base.CertData).To(Equal([]byte("cert"))) // untouched
	})
})

var _ = Describe("OIDCProvider.MarshalJSON", func() {
	It("reports the provider type", func() {
		b, err := (&OIDCProvider{}).MarshalJSON()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(b)).To(ContainSubstring(`"type":"oidc"`))
	})
})
