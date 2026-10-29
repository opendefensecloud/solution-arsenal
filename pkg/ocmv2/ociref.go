// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ocmv2

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/open-component-model/community/ocm-kit/helmvalues"
	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	descv2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// ErrNoOCIReference reports that a resource has no OCI reference a cluster could
// pull. Callers treat it as "not an OCI resource, skip it".
var ErrNoOCIReference = errors.New("no resolvable OCI reference")

// IsNoOCIReference reports whether err says the resource has no resolvable OCI
// reference, which callers treat as "skip this resource".
func IsNoOCIReference(err error) bool {
	return errors.Is(err, ErrNoOCIReference)
}

// ociContentMediaTypes are the blob media types that mean a LocalBlob holds OCI
// content — an image or a packaged chart — rather than plain component metadata.
var ociContentMediaTypes = map[string]bool{
	"application/vnd.oci.image.manifest.v1+json": true,
	"application/vnd.oci.image.index.v1+json":    true,
}

// ResolveOCIReference returns the absolute OCI reference for a resource backed
// by OCI content.
//
// It returns ErrNoOCIReference (wrapped) when the resource has no resolvable OCI
// reference at all, which callers treat as "not an OCI resource, skip it".
//
// reanchored reports whether the resolved reference belongs to the repository
// named by repoBaseURL, either because its host was corrected to it or because
// it already matched. An external absolute reference leaves reanchored false.
// Callers that need the resource transport use this to apply the component
// registry's scheme only to references it serves; external references are HTTPS.
//
// This is the single resolver for both consumers of component resources — the
// discovery API writer and the renderer's values templating — so the two cannot
// disagree about what a resource points at. It understands:
//
//   - OCIImage (and its ociArtifact/ociRegistry/ociImage aliases), which carries
//     an absolute reference to a published artifact.
//   - LocalBlob with a globalAccess, which likewise names a published location.
//
// A component-local blob holding no OCI content yields ErrNoOCIReference, so
// callers skip it; the Helm values template is the usual case. A local blob that
// does hold OCI content is an error, because the component was transferred
// without publishing its resources — see resolveRef.
//
// repoBaseURL is the "<host>/<namespace>" the repository was opened with, e.g.
// "10.96.200.10:443/test".
func ResolveOCIReference(res descruntime.Resource, repoBaseURL string) (ref helmvalues.ImageReference, reanchored bool, err error) {
	rawRef, err := resolveRef(res, repoBaseURL)
	if err != nil {
		return helmvalues.ImageReference{}, false, err
	}

	parsed, err := helmvalues.ParseOCIRef(rawRef)
	if err != nil {
		return helmvalues.ImageReference{}, false, fmt.Errorf("resource %q resolved to invalid OCI reference %q: %w", res.Name, rawRef, err)
	}

	ref, reanchored = reanchor(parsed, repoBaseURL)

	return ref, reanchored, nil
}

// reanchor corrects the host of a reference to an artifact this repository holds.
//
// A by-value transfer bakes in whichever hostname it was pushed through. That is
// often not the address the cluster reaches the registry by. A port-forward, for
// instance, records "localhost:4443" for a registry the cluster knows as a
// service IP. The repository path is the reliable part: when it sits under the
// namespace this repository was opened with, the artifact was copied here, so the
// host is corrected to the one we are reading from.
//
// reanchored reports whether the reference belongs to this repository. It is true
// even when the host was already the repository's, so callers can use it to apply
// the repository's transport to references served from the same registry.
func reanchor(ref helmvalues.ImageReference, repoBaseURL string) (reanchoredRef helmvalues.ImageReference, reanchored bool) {
	host, namespace, found := strings.Cut(strings.TrimSuffix(repoBaseURL, "/"), "/")
	if !found || host == "" || namespace == "" {
		// A component at the registry root has no namespace to key on, so a
		// copied reference cannot be told apart from an upstream one.
		// If the reference's host already matches the base URL host, treat it as
		// reanchored.
		return ref, host != "" && ref.Host == host
	}

	if ref.Repository != namespace && !strings.HasPrefix(ref.Repository, namespace+"/") {
		return ref, false
	}

	ref.Host = host

	return ref, true
}

// resolveRef produces the raw absolute reference for a resource.
//
// It returns ErrNoOCIReference (wrapped) when the resource has no resolvable OCI
// reference. That is an expected, skippable outcome, distinct from a resolution
// failure such as an unpublished component-local OCI blob.
func resolveRef(res descruntime.Resource, repoBaseURL string) (rawRef string, err error) {
	lb, hasGlobalAccess := localBlob(res.Access)
	if lb == nil || hasGlobalAccess {
		// An absolute access (OCIImage and its aliases) carries its reference
		// directly, and a globalAccess names a published location outside the
		// component. ocm-kit decodes both.
		ref, ok, err := helmvalues.ResourceOCIReference(res, repoBaseURL)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("%w: resource %q", ErrNoOCIReference, res.Name)
		}

		return ref, nil
	}

	// A component-local blob that is not OCI content is not something a cluster
	// pulls. The Helm values template is the usual case, and the renderer reads
	// it straight out of the component. Skipping it is correct, not a failure.
	if !ociContentMediaTypes[lb.MediaType] {
		return "", fmt.Errorf("%w: resource %q is a component-local blob that is not OCI content", ErrNoOCIReference, res.Name)
	}

	// OCI content still sitting inside the component means it was transferred
	// without an oci.uploader configured, so nothing was published for it.
	//
	// Refuse rather than invent a reference. A referenceName looks like a
	// location but nothing is published at it, so resolving it yields a
	// reference that fails at pull time with "not found"; the component's own
	// descriptor path is pullable by digest but carries no tag, which silently
	// breaks any values template that renders one. Both leave a component that
	// half-deploys, which is worse than refusing it here.
	return "", fmt.Errorf(
		"resource %q is OCI content held as a component-local blob, which is not pullable: "+
			"transfer the component with an oci.uploader configured so its resources "+
			"are published as OCI artifacts",
		res.Name)
}

// localBlob normalizes the LocalBlob shapes an access may arrive in into the
// v2 struct, and reports whether it carries a globalAccess.
//
// It returns nil when the access is not a LocalBlob.
func localBlob(access runtime.Typed) (lb *descv2.LocalBlob, hasGlobalAccess bool) {
	switch a := access.(type) {
	case *descv2.LocalBlob:
		return a, a.GlobalAccess != nil
	case *descruntime.LocalBlob:
		return &descv2.LocalBlob{
			Type:           a.Type,
			LocalReference: a.LocalReference,
			MediaType:      a.MediaType,
			ReferenceName:  a.ReferenceName,
		}, a.GlobalAccess != nil
	case *runtime.Raw:
		if a == nil {
			return nil, false
		}
		// The legacy lowercase spelling is deprecated in the access schema but
		// still appears in descriptors already sitting in registries, so it is
		// accepted when reading. Nothing here emits it.
		if a.Name != descv2.LocalBlobAccessType && a.Name != descv2.LegacyLocalBlobAccessType {
			return nil, false
		}
		decoded := &descv2.LocalBlob{}
		if err := json.Unmarshal(a.Data, decoded); err != nil {
			return nil, false
		}

		return decoded, decoded.GlobalAccess != nil
	default:
		return nil, false
	}
}
