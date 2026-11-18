// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ociregistry

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	ociname "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ManifestDigest returns the manifest digest rawRef (tag or digest reference)
// resolves to, in the form sha256:<hex>. A missing manifest is an error.
func ManifestDigest(ctx context.Context, rawRef string, auth authn.Authenticator, insecure bool) (string, error) {
	parseOpts := []ociname.Option{}
	if insecure {
		parseOpts = append(parseOpts, ociname.Insecure)
	}

	ref, err := ociname.ParseReference(rawRef, parseOpts...)
	if err != nil {
		return "", fmt.Errorf("invalid OCI reference %q: %w", rawRef, err)
	}

	opts := []remote.Option{remote.WithContext(ctx)}
	if auth != nil && auth != authn.Anonymous {
		opts = append(opts, remote.WithAuth(auth))
	}

	desc, err := remote.Head(ref, opts...)
	if err != nil {
		return "", fmt.Errorf("HEAD %s: %w", ref.String(), err)
	}

	return desc.Digest.String(), nil
}
