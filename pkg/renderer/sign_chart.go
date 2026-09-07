// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"bytes"
	"encoding/base64"
	"errors"
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

// errUnresolved reports that opts.Reference does not resolve in the registry.
// Signing treats that as a failure, the signature check treats an absent
// artifact as an absent signature.
var errUnresolved = errors.New("failed to resolve")

// signingTarget is the state both signing and the signature check need: the
// keypair from opts and the digest the reference resolves to.
type signingTarget struct {
	key        signature.SignerVerifier
	ref        name.Reference
	digest     name.Digest
	remoteOpts ociremote.Option
}

// resolveSigningTarget validates opts, loads the keypair and resolves the
// reference to the digest cosign works on.
func resolveSigningTarget(opts SignOptions) (signingTarget, error) {
	if opts.Reference == "" {
		return signingTarget{}, fmt.Errorf("registry reference is required")
	}

	if opts.KeyPath == "" {
		return signingTarget{}, fmt.Errorf("signing key path is required")
	}

	keyBytes, err := os.ReadFile(opts.KeyPath)
	if err != nil {
		return signingTarget{}, fmt.Errorf("failed to read key: %w", err)
	}

	// Only the public half is needed to check for an existing signature, but
	// the private key file is all a render task is given, so both paths load it.
	key, err := cosign.LoadPrivateKey(keyBytes, opts.KeyPassword, nil)
	if err != nil {
		return signingTarget{}, fmt.Errorf("failed to load key: %w", err)
	}

	// cosign signs a digest, not a tag, so resolve the pushed tag first.
	ref, err := name.ParseReference(strings.TrimPrefix(opts.Reference, "oci://"), opts.NameOptions...)
	if err != nil {
		return signingTarget{}, fmt.Errorf("failed to parse reference %s: %w", opts.Reference, err)
	}

	desc, err := remote.Get(ref, opts.RemoteOptions...)
	if err != nil {
		return signingTarget{}, fmt.Errorf("%w %s: %w", errUnresolved, opts.Reference, err)
	}

	return signingTarget{
		key:        key,
		ref:        ref,
		digest:     ref.Context().Digest(desc.Digest.String()),
		remoteOpts: ociremote.WithRemoteOptions(opts.RemoteOptions...),
	}, nil
}

// signAttempts bounds the read/merge/write retries in SignChart.
const signAttempts = 3

// SignChart signs an artifact that was already pushed to an OCI registry using
// cosign's key-based mode and pushes the signature to the same repository,
// tagged "sha256-<hex>.sig".
//
// Verification on the target cluster uses the public half of the keypair via FluxCD's
// OCIRepository spec.verify block.
//
// Signatures are additive, signing the same artifact again with a different
// key appends a signature rather than replacing the existing one.
func SignChart(opts SignOptions) error {
	target, err := resolveSigningTarget(opts)
	if err != nil {
		return err
	}

	payloadBytes, err := payload.Cosign{Image: target.digest, ClaimedIdentity: target.ref.String()}.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to build signing payload: %w", err)
	}

	rawSig, err := target.key.SignMessage(bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to sign payload: %w", err)
	}

	sig, err := static.NewSignature(payloadBytes, base64.StdEncoding.EncodeToString(rawSig))
	if err != nil {
		return fmt.Errorf("failed to build signature: %w", err)
	}

	// Targets of the same release share a chart reference and therefore the
	// signature tag, and writing that tag replaces the whole set. A write that
	// lands between another job's read and its write drops that job's
	// signature, so re-merge onto the current set until ours survives.
	for range signAttempts {
		if err := attachSignature(target, sig); err != nil {
			return err
		}

		signed, err := signedByKey(target)
		if err != nil {
			return err
		}

		if signed {
			return nil
		}
	}

	return fmt.Errorf("signature for %s was overwritten by a concurrent signer %d times", target.digest, signAttempts)
}

// attachSignature merges sig into the artifact's current signature set and
// writes the result back to the shared signature tag.
func attachSignature(target signingTarget, sig oci.Signature) error {
	entity, err := ociremote.SignedEntity(target.digest, target.remoteOpts)
	if err != nil {
		return fmt.Errorf("failed to read existing signatures for %s: %w", target.digest, err)
	}

	signed, err := mutate.AttachSignatureToEntity(entity, sig)
	if err != nil {
		return fmt.Errorf("failed to attach signature: %w", err)
	}

	if err := ociremote.WriteSignatures(target.digest.Repository, signed, target.remoteOpts); err != nil {
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
	target, err := resolveSigningTarget(opts)
	if errors.Is(err, errUnresolved) {
		// The artifact isn't there, so it cannot be signed yet.
		return false, nil
	} else if err != nil {
		return false, err
	}

	return signedByKey(target)
}

// signedByKey reports whether the artifact already carries a signature made by
// target's key.
func signedByKey(target signingTarget) (bool, error) {
	sigTag, err := ociremote.SignatureTag(target.digest, target.remoteOpts)
	if err != nil {
		return false, fmt.Errorf("failed to resolve signature tag for %s: %w", target.digest, err)
	}

	// Signatures returns an empty set rather than an error when the tag is absent.
	sigs, err := ociremote.Signatures(sigTag, target.remoteOpts)
	if err != nil {
		return false, fmt.Errorf("failed to read signatures for %s: %w", target.digest, err)
	}

	list, err := sigs.Get()
	if err != nil {
		return false, fmt.Errorf("failed to list signatures for %s: %w", target.digest, err)
	}

	for _, sig := range list {
		ok, err := signatureMatches(target.key, sig, target.digest)
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
