// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"helm.sh/helm/v4/pkg/registry"
)

type PushOptions struct {
	Reference     string
	ClientOptions []registry.ClientOption
}

// SignOptions configures SignChart.
// The registry access cosign needs is expressed with go-containerregistry options.
type SignOptions struct {
	// Reference is the OCI reference of the pushed artifact, with or without
	// the "oci://" prefix.
	Reference string
	// KeyPath is the path to the cosign private key.
	KeyPath string
	// KeyPassword decrypts the private key.
	KeyPassword []byte
	// NameOptions are applied when parsing Reference (e.g. name.Insecure).
	NameOptions []name.Option
	// RemoteOptions carry registry credentials and the request context.
	RemoteOptions []remote.Option
}
