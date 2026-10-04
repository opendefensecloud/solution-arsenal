// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ocmv2

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/open-component-model/community/ocm-kit/helmvalues"
	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	descv2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// ociContentMediaTypes are the blob media types that mean a LocalBlob holds OCI
// content — an image or a packaged chart — rather than plain component metadata.
var ociContentMediaTypes = map[string]bool{
	"application/vnd.oci.image.manifest.v1+json": true,
	"application/vnd.oci.image.index.v1+json":    true,
}

// ResolveOCIReference returns the absolute OCI reference for a resource backed
// by OCI content. ok is false when the resource has no resolvable OCI reference
// at all, which callers treat as "not an OCI resource, skip it".
//
// This is the single resolver for both consumers of component resources — the
// discovery API writer and the renderer's values templating — so the two cannot
// disagree about what a resource points at. It understands:
//
//   - OCIImage (and its ociArtifact/ociRegistry/ociImage aliases), which carries
//     an absolute reference to a published artifact.
//   - LocalBlob with a globalAccess, which likewise names a published location.
//
// A component-local blob holding no OCI content yields ok=false and no error, so
// callers skip it; the Helm values template is the usual case. A local blob that
// does hold OCI content is an error, because the component was transferred
// without publishing its resources — see resolveRef.
//
// repoBaseURL is the "<host>/<namespace>" the repository was opened with, e.g.
// "10.96.200.10:443/test".
func ResolveOCIReference(res descruntime.Resource, repoBaseURL string) (helmvalues.ImageReference, bool, error) {
	ref, ok, err := resolveRef(res, repoBaseURL)
	if err != nil || !ok {
		return helmvalues.ImageReference{}, false, err
	}

	parsed, err := helmvalues.ParseOCIRef(ref)
	if err != nil {
		return helmvalues.ImageReference{}, false, fmt.Errorf("resource %q resolved to invalid OCI reference %q: %w", res.Name, ref, err)
	}

	return reanchor(parsed, repoBaseURL), true, nil
}

// reanchor corrects the host of a reference to an artifact this repository holds.
//
// A by-value transfer bakes in whichever hostname it was pushed through. That is
// often not the address the cluster reaches the registry by. A port-forward, for
// instance, records "localhost:4443" for a registry the cluster knows as a
// service IP. The repository path is the reliable part: when it sits under the
// namespace this repository was opened with, the artifact was copied here, so the
// host is corrected to the one we are reading from.
func reanchor(ref helmvalues.ImageReference, repoBaseURL string) helmvalues.ImageReference {
	host, namespace, found := strings.Cut(strings.TrimSuffix(repoBaseURL, "/"), "/")
	if !found || host == "" || namespace == "" {
		// A component at the registry root has no namespace to key on, so a
		// copied reference cannot be told apart from an upstream one.
		return ref
	}

	if ref.Repository != namespace && !strings.HasPrefix(ref.Repository, namespace+"/") {
		return ref
	}

	ref.Host = host

	return ref
}

// resolveRef produces the raw absolute reference string for a resource.
func resolveRef(res descruntime.Resource, repoBaseURL string) (string, bool, error) {
	lb, hasGlobalAccess := localBlob(res.Access)
	if lb == nil {
		// An absolute access (OCIImage and its aliases) carries its reference
		// directly.
		return helmvalues.ResourceOCIReference(res, repoBaseURL)
	}

	// A globalAccess names a published location outside the component, so it is
	// a real reference; ocm-kit decodes it.
	if hasGlobalAccess {
		return helmvalues.ResourceOCIReference(res, repoBaseURL)
	}

	// A component-local blob that is not OCI content is not something a cluster
	// pulls. The Helm values template is the usual case, and the renderer reads
	// it straight out of the component. Skipping it is correct, not a failure.
	if !ociContentMediaTypes[lb.MediaType] {
		return "", false, nil
	}

	// OCI content still sitting inside the component means it was transferred by
	// value without `--upload-as ociArtifact`, so nothing was published for it.
	//
	// Refuse rather than invent a reference. A referenceName looks like a
	// location but nothing is published at it, so resolving it yields a
	// reference that fails at pull time with "not found"; the component's own
	// descriptor path is pullable by digest but carries no tag, which silently
	// breaks any values template that renders one. Both leave a component that
	// half-deploys, which is worse than refusing it here.
	return "", false, fmt.Errorf(
		"resource %q is OCI content held as a component-local blob, which is not pullable: "+
			"transfer the component with `--upload-as ociArtifact` so its resources are published as OCI artifacts",
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
