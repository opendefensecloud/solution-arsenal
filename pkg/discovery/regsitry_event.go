// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import "time"

// RegistryEvent represents an event sent by the RegistryScanner containing
// information about discovered artifacts in the OCI registry.
type RegistryEvent struct {
	Registry   string
	Repository string
	Namespace  string
	Component  string
	Schema     string
	Timestamp  time.Time
	Error      error
}
