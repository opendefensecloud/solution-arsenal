// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

// RegistryCredentials holds resolved username/password credentials for an OCI registry.
// Credentials are obtained by reading the K8s Secret referenced by
// solar.Registry.Spec.SolarSecretRef.
type RegistryCredentials struct {
	// Username is the username used to authenticate with the registry.
	Username string
	// Password is the password used to authenticate with the registry.
	Password string //nolint:gosec // credential value read from a K8s Secret at runtime
}
