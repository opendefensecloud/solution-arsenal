// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"context"
	"crypto"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v3/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/signature"
)

func ociremoteOpts(ctx context.Context, opts PushOptions) []ociremote.Option {
	var remoteOpts []remote.Option
	remoteOpts = append(remoteOpts, remote.WithContext(ctx))
	if opts.Username != "" {
		remoteOpts = append(remoteOpts, remote.WithAuth(authn.FromConfig(authn.AuthConfig{
			Username: opts.Username,
			Password: opts.Password,
		})))
	}
	var nameOpts []name.Option
	if opts.PlainHTTP {
		nameOpts = append(nameOpts, name.Insecure)
	}

	return []ociremote.Option{
		ociremote.WithMoreRemoteOptions(remoteOpts...),
		ociremote.WithNameOptions(nameOpts...),
	}
}

func signImage(sv signature.SignerVerifier, ref name.Reference, opts ...ociremote.Option) error {
	d, err := ociremote.ResolveDigest(ref, opts...)
	if err != nil {
		return fmt.Errorf("resolve digest: %w", err)
	}

	payload, sig, err := signature.SignImage(sv, d, nil)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	sigObj, err := static.NewSignature(payload, base64.StdEncoding.EncodeToString(sig))
	if err != nil {
		return fmt.Errorf("create signature: %w", err)
	}

	entity, err := mutate.AttachSignatureToEntity(ociremote.SignedUnknown(d, opts...), sigObj)
	if err != nil {
		return fmt.Errorf("attach signature: %w", err)
	}

	return ociremote.WriteSignatures(d.Context(), entity, opts...)
}

func SignArtifact(ctx context.Context, ref string, opts PushOptions) error {
	if opts.Signing == nil || opts.Signing.KeyPath == "" {
		return fmt.Errorf("signing config is nil or key path is empty")
	}

	sv, err := signature.LoadSignerVerifierFromPEMFile(
		opts.Signing.KeyPath,
		crypto.SHA256,
		func(bool) ([]byte, error) { return []byte(opts.Signing.Password), nil },
	)
	if err != nil {
		return fmt.Errorf("load signing key: %w", err)
	}

	imageRef, err := name.ParseReference(strings.TrimPrefix(ref, "oci://"))
	if err != nil {
		return fmt.Errorf("parse reference %q: %w", ref, err)
	}

	return signImage(sv, imageRef, ociremoteOpts(ctx, opts)...)
}
