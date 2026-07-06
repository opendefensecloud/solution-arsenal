// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import "helm.sh/helm/v4/pkg/registry"

// SigningConfig configures OCI artifact signing after the chart is pushed.
// Signing uses cosign with a static key pair (no OIDC/Fulcio).
type SigningConfig struct {
	// KeyPath is the path to the cosign private key file (PEM-encoded).
	KeyPath string
	// Password is the password for an encrypted private key.
	// Leave empty for unencrypted keys.
	Password string
}

type PushOptions struct {
	Reference     string
	ClientOptions []registry.ClientOption
	// Signing optionally configures signing of the pushed artifact.
	// When set, the chart is signed after a successful push.
	Signing *SigningConfig

	// Username for basic auth registry access (shared between push and signing).
	Username string
	// Password for basic auth registry access (shared between push and signing).
	Password string
	// PlainHTTP indicates whether to use plain HTTP instead of HTTPS.
	PlainHTTP bool
}
