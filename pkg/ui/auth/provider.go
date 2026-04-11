// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"net/http"

	"go.opendefense.cloud/solar/pkg/ui/session"
	"k8s.io/client-go/rest"
)

// Provider abstracts authentication mechanisms for the UI backend.
type Provider interface {
	// HandleLogin initiates the authentication flow.
	HandleLogin(store *session.Store) http.HandlerFunc
	// HandleCallback handles the authentication callback (e.g. OIDC redirect).
	HandleCallback(store *session.Store) http.HandlerFunc
	// WrapConfig returns a rest.Config that authenticates as the session's user.
	WrapConfig(base *rest.Config, sess *session.Data) *rest.Config
}
