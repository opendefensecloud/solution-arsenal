// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

// OCM v2 returns descriptors without the OCI storage context used by v1
// pinning. This adapter verifies the tag through the registry resolver and
// retains its immutable reference for descriptor and local-resource reads.
package ocmv2

import (
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/open-component-model/community/ocm-kit/helmvalues"
	"github.com/opencontainers/go-digest"
	ociImageSpecV1 "github.com/opencontainers/image-spec/specs-go/v1"
	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/oci"
	"ocm.software/open-component-model/bindings/go/oci/looseref"
	"ocm.software/open-component-model/bindings/go/oci/spec"
)

// pinOCIReference only uses digests that identify an OCI manifest. Other OCM
// digest normalizations do not identify content the registry can serve.
func pinOCIReference(res descruntime.Resource, ref helmvalues.ImageReference) helmvalues.ImageReference {
	if d := res.Digest; ref.Digest == "" && d != nil &&
		d.NormalisationAlgorithm == "ociArtifactDigest/v1" && d.HashAlgorithm == crypto.SHA256.String() {
		if dg := digest.NewDigestFromEncoded(digest.SHA256, d.Value); dg.Validate() == nil {
			ref.Digest = dg.String()
		}
	}

	return ref
}

type componentVersion struct{ name, version string }

// digestPinnedResolver keeps descriptor and local-resource reads on the
// same verified manifest, even if the registry tag moves between reads.
type digestPinnedResolver struct {
	oci.Resolver
	mu   sync.RWMutex
	pins map[componentVersion]string
}

func (r *digestPinnedResolver) pinnedReference(name, version string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.pins[componentVersion{name, version}]
}

func (r *digestPinnedResolver) pin(name, version, reference string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pins[componentVersion{name, version}] = reference
}

func (r *digestPinnedResolver) ComponentVersionReference(ctx context.Context, name, version string) string {
	if ref := r.pinnedReference(name, version); ref != "" {
		return ref
	}

	return r.Resolver.ComponentVersionReference(ctx, name, version)
}

// PinComponentVersion verifies that the current tag matches the digest recorded
// by discovery, then binds subsequent descriptor and local-resource reads to
// that immutable manifest or index. Call it before GetComponentVersion.
func (r *Repository) PinComponentVersion(ctx context.Context, name, version, want string) error {
	if _, err := digest.Parse(want); err != nil {
		return fmt.Errorf("invalid component descriptor digest %q: %w", want, err)
	}
	reference := r.pinnedResolver.Resolver.ComponentVersionReference(ctx, name, version)
	store, err := r.pinnedResolver.StoreForReference(ctx, reference)
	if err != nil {
		return fmt.Errorf("failed to access component descriptor: %w", err)
	}
	got, err := store.Resolve(ctx, reference)
	if err != nil {
		return fmt.Errorf("failed to resolve component descriptor: %w", err)
	}
	if got.Digest.String() != want && firstManifestDigest(ctx, store, want) != got.Digest {
		return fmt.Errorf("component version %s:%s resolves to %s, expected %s: the tag was overwritten after discovery",
			name, version, got.Digest, want)
	}
	ref, err := looseref.ParseReference(reference)
	if err != nil {
		return fmt.Errorf("failed to parse component descriptor reference: %w", err)
	}
	ref.Reference.Reference = got.Digest.String()
	r.pinnedResolver.pin(name, version, ref.Reference.String())

	return nil
}

// Discovery can record an index digest while a tag points directly at its first
// manifest. Only accept that manifest when it belongs to the recorded index.
func firstManifestDigest(ctx context.Context, store spec.Store, indexDigest string) digest.Digest {
	desc, err := store.Resolve(ctx, indexDigest)
	if err != nil || desc.MediaType != ociImageSpecV1.MediaTypeImageIndex {
		return ""
	}
	reader, err := store.Fetch(ctx, desc)
	if err != nil {
		return ""
	}
	defer func() { _ = reader.Close() }()
	var index ociImageSpecV1.Index
	if err := json.NewDecoder(reader).Decode(&index); err != nil || len(index.Manifests) == 0 {
		return ""
	}

	return index.Manifests[0].Digest
}
