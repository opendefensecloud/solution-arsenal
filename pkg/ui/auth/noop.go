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

// HandleLogin returns 501 — no login flow in noop mode.
func (p *NoopProvider) HandleLogin(_ *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "authentication not configured", http.StatusNotImplemented)
	}
}

// HandleCallback returns 501 — no callback in noop mode.
func (p *NoopProvider) HandleCallback(_ *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "authentication not configured", http.StatusNotImplemented)
	}
}

// WrapConfig returns the base config unmodified (uses the server's own identity).
func (p *NoopProvider) WrapConfig(base *rest.Config, _ *session.Data) *rest.Config {
	return base
}
