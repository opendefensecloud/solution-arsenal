// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

// Package ociregistry provides OCI artifact lifecycle operations.
package ociregistry

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
)

// DeleteTag deletes the OCI tag identified by rawRef (e.g. "registry.example.com/ns/repo:v1").
// auth provides credentials for the request.
// A non-nil error means the deletion failed and should be surfaced to the caller.
func DeleteTag(ctx context.Context, rawRef string, auth authn.Authenticator) error {
	return (&standardDeleter{}).DeleteTag(ctx, rawRef, auth)
}
