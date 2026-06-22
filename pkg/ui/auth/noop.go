// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"net/http"

	"k8s.io/client-go/rest"

	"go.opendefense.cloud/solar/pkg/ui/session"
)

// NoopProvider is an auth provider that does not perform any authentication.
// It is used when no OIDC issuer is configured — requests use the server's
// own service account credentials (useful for development and testing).
type NoopProvider struct{}

// NewNoopProvider creates a no-op auth provider.
func NewNoopProvider() *NoopProvider {
	return &NoopProvider{}
}

// noopUsername labels the synthetic session created in noop mode. Requests
// still authenticate to K8s with the server's own credentials (see
// WrapConfig); this only satisfies the BFF's auth gate.
const noopUsername = "noop"

// HandleLogin establishes a synthetic authenticated session so that auth-gated
// API routes work in noop mode, then redirects to the SPA.
func (p *NoopProvider) HandleLogin(store *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store.Set(w, &session.Data{Username: noopUsername})
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// HandleCallback mirrors HandleLogin: there is no external IdP in noop mode, so
// it simply establishes the synthetic session and redirects to the SPA.
func (p *NoopProvider) HandleCallback(store *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store.Set(w, &session.Data{Username: noopUsername})
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// WrapConfig returns the base config unmodified (uses the server's own identity).
func (p *NoopProvider) WrapConfig(base *rest.Config, _ *session.Data) *rest.Config {
	return base
}
