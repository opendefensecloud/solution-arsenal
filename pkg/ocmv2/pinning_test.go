// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ocmv2

import (
	"strings"
	"testing"

	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	ociaccessv1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
)

func TestResolveOCIReferenceDigestPinning(t *testing.T) {
	const reference = "ghcr.io/upstream/chart:1.0"
	manifestDigest := strings.Repeat("ab", 32)
	existingDigest := "sha256:" + strings.Repeat("cd", 32)
	for _, tt := range []struct {
		name      string
		reference string
		digest    *descruntime.Digest
		want      string
	}{
		{
			name:      "pins a valid SHA256 OCI artifact digest",
			reference: reference,
			digest:    &descruntime.Digest{HashAlgorithm: "SHA-256", NormalisationAlgorithm: "ociArtifactDigest/v1", Value: manifestDigest},
			want:      reference + "@sha256:" + manifestDigest,
		},
		{
			name:      "keeps the tag for a generic blob digest",
			reference: reference,
			digest:    &descruntime.Digest{HashAlgorithm: "SHA-256", NormalisationAlgorithm: "genericBlobDigest/v1", Value: manifestDigest},
			want:      reference,
		},
		{
			name:      "keeps the tag for a malformed OCI artifact digest",
			reference: reference,
			digest:    &descruntime.Digest{HashAlgorithm: "SHA-256", NormalisationAlgorithm: "ociArtifactDigest/v1", Value: "ABC"},
			want:      reference,
		},
		{
			name:      "keeps the tag for an unsupported hash algorithm",
			reference: reference,
			digest:    &descruntime.Digest{HashAlgorithm: "SHA-512", NormalisationAlgorithm: "ociArtifactDigest/v1", Value: strings.Repeat("ab", 64)},
			want:      reference,
		},
		{
			name:      "keeps the tag when no resource digest is recorded",
			reference: reference,
			want:      reference,
		},
		{
			name:      "preserves an explicit access digest over the resource digest",
			reference: reference + "@" + existingDigest,
			digest:    &descruntime.Digest{HashAlgorithm: "SHA-256", NormalisationAlgorithm: "ociArtifactDigest/v1", Value: manifestDigest},
			want:      reference + "@" + existingDigest,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := descruntime.Resource{Access: &ociaccessv1.OCIImage{ImageReference: tt.reference}, Digest: tt.digest}
			ref, _, err := ResolveOCIReference(res, "registry.example.com/copied")
			if err != nil {
				t.Fatal(err)
			}
			if got := ref.String(); got != tt.want {
				t.Errorf("resolved reference = %q, want %q", got, tt.want)
			}
		})
	}
}
