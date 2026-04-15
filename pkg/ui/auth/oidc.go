// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
}

// OIDCProvider implements the Provider interface using OpenID Connect.
type OIDCProvider struct {
	provider *oidc.Provider
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	authMode AuthMode
}

// NewOIDCProvider creates a new OIDC provider.
func NewOIDCProvider(cfg OIDCConfig) (*OIDCProvider, error) {
	provider, err := oidc.NewProvider(context.Background(), cfg.Issuer)
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
		provider: provider,
		oauth:    oauthCfg,
		verifier: verifier,
		authMode: authMode,
	}, nil
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

		token, err := p.oauth.Exchange(r.Context(), code)
		if err != nil {
			http.Error(w, fmt.Sprintf("token exchange failed: %v", err), http.StatusInternalServerError)

			return
		}

		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			http.Error(w, "no id_token in response", http.StatusInternalServerError)

			return
		}

		idToken, err := p.verifier.Verify(r.Context(), rawIDToken)
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

		// For demo purposes, we derive some simple groups from the username.
		// In a real implementation, group membership would come from the OIDC provider and be included in the token claims.
		groups := claims.Groups
		switch {
		case strings.Contains(username, "admin"):
			groups = append(groups, "admin")
		case strings.Contains(username, "maintainer"):
			groups = append(groups, "maintainer")
		case strings.Contains(username, "coordinator"):
			groups = append(groups, "coordinator")
		}

		store.Set(w, &session.Data{
			Username:    username,
			Groups:      groups,
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
