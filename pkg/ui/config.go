// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ui

// Config holds the configuration for the solar-ui server.
type Config struct {
	ListenAddr       string
	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	SessionKey       string
	Kubeconfig       string
	// AuthMode controls how OIDC identity is conveyed to K8s: "token" (default)
	// forwards the id_token as a bearer token; "impersonate" uses K8s impersonation.
	AuthMode string
	// DevViteURL, when set, proxies non-API requests to the Vite dev server
	// instead of serving the embedded static files. Example: "http://localhost:5173"
	DevViteURL string
}
