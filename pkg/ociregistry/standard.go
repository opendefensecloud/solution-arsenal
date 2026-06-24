// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ociregistry

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	ociname "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// standardDeleter deletes OCI tags via the OCI Distribution Spec DELETE endpoint:
//
//	DELETE /v2/<name>/manifests/<reference>
//
// This works with any OCI Distribution Spec-compliant registry
type standardDeleter struct{}

func (d *standardDeleter) DeleteTag(ctx context.Context, rawRef string, auth authn.Authenticator, insecure bool) error {
	parseOpts := []ociname.Option{}
	if insecure {
		parseOpts = append(parseOpts, ociname.Insecure)
	}

	ref, err := ociname.ParseReference(rawRef, parseOpts...)
	if err != nil {
		return fmt.Errorf("invalid OCI reference %q: %w", rawRef, err)
	}

	opts := []remote.Option{remote.WithContext(ctx)}
	if auth != nil && auth != authn.Anonymous {
		opts = append(opts, remote.WithAuth(auth))
	}

	if err := remote.Delete(ref, opts...); err != nil {
		return fmt.Errorf("DELETE %s: %w", ref.String(), err)
	}

	return nil
}
