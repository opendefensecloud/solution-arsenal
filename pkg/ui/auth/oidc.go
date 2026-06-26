// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"k8s.io/client-go/rest"

	"go.opendefense.cloud/solar/pkg/ui/session"
)

// AuthMode determines how the OIDC user identity is conveyed to the K8s API.
type AuthMode string

const (
	// AuthModeToken forwards the OIDC id_token as a bearer token.
	// Requires the K8s API server to be configured with OIDC flags.
	AuthModeToken AuthMode = "token"

	// AuthModeImpersonate uses K8s user impersonation. The backend's own
	// credentials (SA or kubeconfig) must have impersonation privileges.
	AuthModeImpersonate AuthMode = "impersonate"
)

// OIDCConfig holds the configuration for the OIDC provider.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string //nolint:gosec // config field, not a hardcoded credential
	RedirectURL  string
	AuthMode     AuthMode
	// CACertFile, when set, is a PEM file whose certificates are used as the TLS
	// roots for talking to the issuer. Needed when the issuer (e.g. a dev Dex)
	// presents a private CA. (SSL_CERT_FILE is ignored on macOS).
	CACertFile string
}

// OIDCProvider implements the Provider interface using OpenID Connect.
type OIDCProvider struct {
	provider   *oidc.Provider
	oauth      oauth2.Config
	verifier   *oidc.IDTokenVerifier
	authMode   AuthMode
	httpClient *http.Client
}

// NewOIDCProvider creates a new OIDC provider.
func NewOIDCProvider(cfg OIDCConfig) (*OIDCProvider, error) {
	httpClient, err := newHTTPClient(cfg.CACertFile)
	if err != nil {
		return nil, err
	}

	// Carry the HTTP client through the context so go-oidc uses it for discovery
	// and (via the provider) for fetching the remote JWKS.
	ctx := oidc.ClientContext(context.Background(), httpClient)

	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider for issuer %q: %w", cfg.Issuer, err)
	}

	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "groups"},
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	authMode := cfg.AuthMode
	if authMode == "" {
		authMode = AuthModeToken
	}

	return &OIDCProvider{
		provider:   provider,
		oauth:      oauthCfg,
		verifier:   verifier,
		authMode:   authMode,
		httpClient: httpClient,
	}, nil
}

// newHTTPClient builds an HTTP client. When caCertFile is set, its PEM certificates
// are added to the system root pool and used as the TLS roots.
func newHTTPClient(caCertFile string) (*http.Client, error) {
	if caCertFile == "" {
		return &http.Client{}, nil
	}

	pem, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read OIDC CA cert %q: %w", caCertFile, err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}

	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no certificates found in OIDC CA cert %q", caCertFile)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}

	return &http.Client{Transport: transport}, nil
}

// HandleLogin redirects the user to the OIDC provider.
func (p *OIDCProvider) HandleLogin(store *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := generateState()
		store.SetState(w, state)
		http.Redirect(w, r, p.oauth.AuthCodeURL(state), http.StatusFound)
	}
}

// HandleCallback processes the OIDC callback.
func (p *OIDCProvider) HandleCallback(store *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code parameter", http.StatusBadRequest)

			return
		}

		expectedState := store.GetState(r)
		actualState := r.URL.Query().Get("state")
		if expectedState == "" || actualState != expectedState {
			http.Error(w, "invalid state parameter", http.StatusBadRequest)

			return
		}
		store.ClearState(w)

		ctx := oidc.ClientContext(r.Context(), p.httpClient)

		token, err := p.oauth.Exchange(ctx, code)
		if err != nil {
			http.Error(w, fmt.Sprintf("token exchange failed: %v", err), http.StatusInternalServerError)

			return
		}

		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			http.Error(w, "no id_token in response", http.StatusInternalServerError)

			return
		}

		idToken, err := p.verifier.Verify(ctx, rawIDToken)
		if err != nil {
			http.Error(w, fmt.Sprintf("id_token verification failed: %v", err), http.StatusInternalServerError)

			return
		}

		var claims struct {
			Email  string   `json:"email"`
			Name   string   `json:"name"`
			Groups []string `json:"groups"`
		}
		if err := idToken.Claims(&claims); err != nil {
			http.Error(w, fmt.Sprintf("failed to parse claims: %v", err), http.StatusInternalServerError)

			return
		}

		username := claims.Email
		if username == "" {
			username = claims.Name
		}

		store.Set(w, &session.Data{
			Username:    username,
			Groups:      claims.Groups,
			IDToken:     rawIDToken,
			AccessToken: token.AccessToken,
		})

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// WrapConfig returns a rest.Config that authenticates as the session's user.
// In token mode, the OIDC id_token is forwarded as a bearer token.
// In impersonate mode, K8s user impersonation is used.
// When the session has an active impersonation override (admin previewing as
// another user), impersonation headers are always used regardless of authMode.
func (p *OIDCProvider) WrapConfig(base *rest.Config, sess *session.Data) *rest.Config {
	cfg := rest.CopyConfig(base)

	// Session-level impersonation (admin "preview as" feature) takes precedence over the global authMode
	if sess.ImpersonatingAs != "" {
		cfg.Impersonate = rest.ImpersonationConfig{
			UserName: sess.ImpersonatingAs,
			Groups:   sess.ImpersonatingGroups,
		}

		return cfg
	}

	switch p.authMode {
	case AuthModeImpersonate:
		cfg.Impersonate = rest.ImpersonationConfig{
			UserName: sess.Username,
			Groups:   sess.Groups,
		}
	default: // token
		cfg.BearerToken = sess.IDToken
		cfg.BearerTokenFile = ""
		// Clear client certificate credentials so the K8s API server
		// authenticates via the OIDC bearer token only. Without this,
		// kubeconfigs that use client cert auth (e.g. Kind) would have
		// the cert take precedence and bypass per-user RBAC enforcement.
		cfg.CertData = nil
		cfg.CertFile = ""
		cfg.KeyData = nil
		cfg.KeyFile = ""
	}

	return cfg
}

// MarshalJSON is used only for /auth/me responses.
func (p *OIDCProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"type": "oidc"})
}

func generateState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return hex.EncodeToString(b)
}
