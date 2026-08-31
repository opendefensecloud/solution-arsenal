// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci"
	"github.com/sigstore/cosign/v3/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/payload"
)

// SignChart signs an artifact that was already pushed to an OCI registry using
// cosign's key-based mode and pushes the signature to the same repository,
// tagged "sha256-<digest>.sig".
//
// Verification on the target cluster uses the public half of the keypair via FluxCD's
// OCIRepository spec.verify block
//
// Signatures are additive, signing the same artifact again with a different
// key appends a signature rather than replacing the existing one.
func SignChart(opts SignOptions) error {
	if opts.Reference == "" {
		return fmt.Errorf("registry reference is required")
	}

	if opts.KeyPath == "" {
		return fmt.Errorf("signing key path is required")
	}

	keyBytes, err := os.ReadFile(opts.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to read signing key: %w", err)
	}

	signer, err := cosign.LoadPrivateKey(keyBytes, opts.KeyPassword, nil)
	if err != nil {
		return fmt.Errorf("failed to load signing key: %w", err)
	}

	// cosign signs a digest, not a tag, so resolve the pushed tag first.
	ref, err := name.ParseReference(strings.TrimPrefix(opts.Reference, "oci://"), opts.NameOptions...)
	if err != nil {
		return fmt.Errorf("failed to parse reference %s: %w", opts.Reference, err)
	}

	desc, err := remote.Get(ref, opts.RemoteOptions...)
	if err != nil {
		return fmt.Errorf("failed to resolve %s: %w", opts.Reference, err)
	}

	digest := ref.Context().Digest(desc.Digest.String())

	payloadBytes, err := payload.Cosign{Image: digest, ClaimedIdentity: ref.String()}.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to build signing payload: %w", err)
	}

	rawSig, err := signer.SignMessage(bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to sign payload: %w", err)
	}

	sig, err := static.NewSignature(payloadBytes, base64.StdEncoding.EncodeToString(rawSig))
	if err != nil {
		return fmt.Errorf("failed to build signature: %w", err)
	}

	remoteOpts := ociremote.WithRemoteOptions(opts.RemoteOptions...)

	entity, err := ociremote.SignedEntity(digest, remoteOpts)
	if err != nil {
		return fmt.Errorf("failed to read existing signatures for %s: %w", digest, err)
	}

	signed, err := mutate.AttachSignatureToEntity(entity, sig)
	if err != nil {
		return fmt.Errorf("failed to attach signature: %w", err)
	}

	if err := ociremote.WriteSignatures(digest.Repository, signed, remoteOpts); err != nil {
		return fmt.Errorf("failed to push signature: %w", err)
	}

	return nil
}

// SignatureExists reports whether the artifact at opts.Reference already
// carries a signature made by the key in opts. It lets a render job skip work
// that another job has already done, without ever skipping the signing of an
// artifact that this task's key has not signed yet.
//
// A missing artifact, a missing signature tag, and a signature made by another
// key all report false.
func SignatureExists(opts SignOptions) (bool, error) {
	if opts.Reference == "" {
		return false, fmt.Errorf("registry reference is required")
	}

	if opts.KeyPath == "" {
		return false, fmt.Errorf("signing key path is required")
	}

	keyBytes, err := os.ReadFile(opts.KeyPath)
	if err != nil {
		return false, fmt.Errorf("failed to read signing key: %w", err)
	}

	verifier, err := cosign.LoadPrivateKey(keyBytes, opts.KeyPassword, nil)
	if err != nil {
		return false, fmt.Errorf("failed to load signing key: %w", err)
	}

	ref, err := name.ParseReference(strings.TrimPrefix(opts.Reference, "oci://"), opts.NameOptions...)
	if err != nil {
		return false, fmt.Errorf("failed to parse reference %s: %w", opts.Reference, err)
	}

	desc, err := remote.Get(ref, opts.RemoteOptions...)
	if err != nil {
		// The artifact isn't there, so it cannot be signed yet.
		return false, nil //nolint:nilerr // absent artifact means absent signature
	}

	digest := ref.Context().Digest(desc.Digest.String())
	remoteOpts := ociremote.WithRemoteOptions(opts.RemoteOptions...)

	sigTag, err := ociremote.SignatureTag(digest, remoteOpts)
	if err != nil {
		return false, fmt.Errorf("failed to resolve signature tag for %s: %w", digest, err)
	}

	// Signatures returns an empty set rather than an error when the tag is absent.
	sigs, err := ociremote.Signatures(sigTag, remoteOpts)
	if err != nil {
		return false, fmt.Errorf("failed to read signatures for %s: %w", digest, err)
	}

	list, err := sigs.Get()
	if err != nil {
		return false, fmt.Errorf("failed to list signatures for %s: %w", digest, err)
	}

	for _, sig := range list {
		ok, err := signatureMatches(verifier, sig, digest)
		if err != nil {
			return false, err
		}

		if ok {
			return true, nil
		}
	}

	return false, nil
}

// signatureMatches reports whether sig was made by verifier over a payload
// claiming exactly digest.
func signatureMatches(verifier signature.Verifier, sig oci.Signature, digest name.Digest) (bool, error) {
	payloadBytes, err := sig.Payload()
	if err != nil {
		return false, fmt.Errorf("failed to read signature payload: %w", err)
	}

	claim := payload.Cosign{}
	if err := claim.UnmarshalJSON(payloadBytes); err != nil {
		// Not a payload shape we produce, so not a signature we made.
		return false, nil //nolint:nilerr // unknown payload means no match
	}

	if claim.Image.DigestStr() != digest.DigestStr() {
		return false, nil
	}

	b64sig, err := sig.Base64Signature()
	if err != nil {
		return false, fmt.Errorf("failed to read signature: %w", err)
	}

	rawSig, err := base64.StdEncoding.DecodeString(b64sig)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	if err := verifier.VerifySignature(bytes.NewReader(rawSig), bytes.NewReader(payloadBytes)); err != nil {
		// Signed by some other key.
		return false, nil //nolint:nilerr // failed verification means no match
	}

	return true, nil
}
