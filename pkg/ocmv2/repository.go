// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

// Package ocmv2 wraps the OCM v2 Go bindings with the narrow set of operations
// SolAr needs: open a component repository with credentials, list and fetch
// component versions, and pull resource blobs.
//
// It exists because the v2 bindings have no `ocm.Context` god-object to hang
// credentials off, so every call site would otherwise repeat the resolver,
// auth-client and temp-directory plumbing. Repository also satisfies ocm-kit's
// helmvalues.Repository, so a repository opened here can be handed straight to
// the values-template renderer.
package ocmv2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	ociImageSpecV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"ocm.software/open-component-model/bindings/go/blob"
	"ocm.software/open-component-model/bindings/go/blob/inmemory"
	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/oci"
	urlresolver "ocm.software/open-component-model/bindings/go/oci/resolver/url"
	ociaccessv1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
	"ocm.software/open-component-model/bindings/go/oci/tar"
	"ocm.software/open-component-model/bindings/go/runtime"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// Credentials are the registry credentials used to open a Repository. A nil
// *Credentials means anonymous access.
//
// Exactly one shape is populated, matching the shape of a Registry's
// solarSecretRef: either basic auth, or a path to a docker config file.
type Credentials struct {
	Username string
	Password string `json:"-"`
	// DockerConfigPath points at a docker config file holding the registry's
	// credentials. Used when solarSecretRef is a kubernetes.io/dockerconfigjson
	// Secret.
	DockerConfigPath string
}

// Repository is an OCM component repository.
type Repository struct {
	repo *oci.Repository
	// baseURL is the "<host>/<namespace>" the repository was opened with, with
	// any scheme stripped. Reference resolution needs it to turn
	// repository-relative accesses back into absolute image references.
	baseURL string
	// tempDir is the blob-staging directory, removed by Close.
	tempDir string
}

// OpenRepository opens the OCM repository at url, which may carry a scheme:
// "http://" selects plain HTTP, anything else (or no scheme) selects TLS.
// Call Close when done.
func OpenRepository(url string, creds *Credentials) (*Repository, error) {
	client, err := authClient(creds)
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "solar-ocm-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	baseURL, plainHTTP := SplitScheme(url)
	// A component sitting at the registry root has an empty namespace, so
	// callers building "<registry>/<namespace>" hand us a trailing slash. The
	// resolver would turn that into a double slash in every request path.
	baseURL = strings.TrimSuffix(baseURL, "/")

	resolver, err := urlresolver.New(
		urlresolver.WithBaseURL(baseURL),
		urlresolver.WithPlainHTTP(plainHTTP),
		urlresolver.WithBaseClient(client),
	)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to create resolver for %s: %w", baseURL, err)
	}

	repo, err := oci.NewRepository(oci.WithResolver(resolver), oci.WithTempDir(tempDir))
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to open repository %s: %w", baseURL, err)
	}

	return &Repository{repo: repo, baseURL: baseURL, tempDir: tempDir}, nil
}

// authClient builds the OCI auth client for creds. Without credentials the
// registry is accessed anonymously; there is no implicit docker-config fallback,
// a docker config must be passed explicitly via DockerConfigPath.
func authClient(creds *Credentials) (*auth.Client, error) {
	client := &auth.Client{Client: retry.DefaultClient, Cache: auth.NewCache()}
	if creds == nil {
		return client, nil
	}

	if creds.DockerConfigPath != "" {
		// A docker config that cannot be loaded is an error: these credentials
		// were requested explicitly.
		store, err := credentials.NewStore(creds.DockerConfigPath, credentials.StoreOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to load docker config %s: %w", creds.DockerConfigPath, err)
		}
		client.Credential = credentials.Credential(store)

		return client, nil
	}

	if creds.Username == "" {
		return client, nil
	}

	cred := auth.Credential{Username: creds.Username, Password: creds.Password}
	client.Credential = func(context.Context, string) (auth.Credential, error) { return cred, nil }

	return client, nil
}

// SplitScheme separates an optional "scheme://" prefix from a registry URL,
// returning the host+path the resolver expects and whether plain HTTP applies.
func SplitScheme(url string) (baseURL string, plainHTTP bool) {
	scheme, rest, found := strings.Cut(url, "://")
	if !found {
		return url, false
	}

	return rest, scheme == "http"
}

// BaseURL returns the "<host>/<namespace>" the repository was opened with,
// without a scheme.
func (r *Repository) BaseURL() string {
	return r.baseURL
}

// Close removes the repository's blob-staging directory.
func (r *Repository) Close() error {
	if r.tempDir == "" {
		return nil
	}

	return os.RemoveAll(r.tempDir)
}

// ListComponentVersions returns every version of component present in the
// repository.
func (r *Repository) ListComponentVersions(ctx context.Context, component string) ([]string, error) {
	return r.repo.ListComponentVersions(ctx, component)
}

// GetComponentVersion resolves a component descriptor.
func (r *Repository) GetComponentVersion(ctx context.Context, name, version string) (*descruntime.Descriptor, error) {
	return r.repo.GetComponentVersion(ctx, name, version)
}

// ResourceBytes downloads the local blob of the named resource. It satisfies
// ocm-kit's helmvalues.Repository.
//
// For a resource whose blob is a nested OCI manifest (e.g. a packaged Helm chart)
// the bytes are an OCI layout tar rather than the payload itself. Use
// ChartArchive for those.
func (r *Repository) ResourceBytes(ctx context.Context, name, version, resourceName string) ([]byte, error) {
	b, _, err := r.repo.GetLocalResource(ctx, name, version, runtime.Identity{"name": resourceName})
	if err != nil {
		return nil, fmt.Errorf("failed to get local resource %q: %w", resourceName, err)
	}

	var buf bytes.Buffer
	if err := blob.Copy(&buf, b); err != nil {
		return nil, fmt.Errorf("failed to read resource %q: %w", resourceName, err)
	}

	return buf.Bytes(), nil
}

// ChartArchive downloads res and returns the packaged Helm chart archive (a
// .tgz) it carries.
//
// Two access shapes have to work. A chart added with the `helm` input is a
// LocalBlob living inside the component and is read locally; a chart referenced
// as an OCI artifact is pulled from the registry it points at.
//
// Either way the bindings hand back an OCI layout tar rather than the chart
// itself, so this unwraps that layout: resolve the single main artifact, then
// return its only layer. A blob that is already the plain archive is returned
// unchanged, so both shapes work.
func (r *Repository) ChartArchive(ctx context.Context, desc *descruntime.Descriptor, res descruntime.Resource) ([]byte, error) {
	raw, err := r.resourceContent(ctx, desc, res)
	if err != nil {
		return nil, err
	}

	store, err := tar.ReadOCILayout(ctx, inmemory.New(bytes.NewReader(raw)))
	if err != nil {
		// Not an OCI layout: the blob is the archive itself.
		return raw, nil
	}
	defer func() { _ = store.Close() }()

	mains := store.MainArtifacts(ctx)
	if len(mains) != 1 {
		return nil, fmt.Errorf("expected exactly one main artifact in resource %q, got %d", res.Name, len(mains))
	}

	manifest, err := fetchManifest(ctx, store, mains[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest of resource %q: %w", res.Name, err)
	}

	if len(manifest.Layers) != 1 {
		return nil, fmt.Errorf("expected exactly one layer in resource %q, got %d", res.Name, len(manifest.Layers))
	}

	return fetchBlob(ctx, store, manifest.Layers[0])
}

// resourceContent reads a resource's raw blob, taking the local path for a
// component-local blob and the registry path for anything with an external
// access.
func (r *Repository) resourceContent(ctx context.Context, desc *descruntime.Descriptor, res descruntime.Resource) ([]byte, error) {
	if lb, _ := localBlob(res.Access); lb != nil {
		return r.ResourceBytes(ctx, desc.Component.Name, desc.Component.Version, res.Name)
	}

	// Download from the re-anchored reference rather than the recorded one.
	// An absolute access records whichever host the component was transferred
	// through (a port-forward, typically) and the OCI resolver honours the
	// reference's own registry rather than the one this repository was opened
	// with, so fetching the recorded reference would try a host that is usually
	// unreachable from here.
	ref, ok, err := ResolveOCIReference(res, r.baseURL)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("resource %q has no resolvable OCI reference to download", res.Name)
	}

	fetch := res
	access := &ociaccessv1.OCIImage{ImageReference: ref.String()}
	access.Type = runtime.NewVersionedType(ociaccessv1.OCIImageType, ociaccessv1.Version)
	fetch.Access = access

	b, err := r.repo.DownloadResource(ctx, &fetch)
	if err != nil {
		return nil, fmt.Errorf("failed to download resource %q from %s: %w", res.Name, ref.String(), err)
	}

	var buf bytes.Buffer
	if err := blob.Copy(&buf, b); err != nil {
		return nil, fmt.Errorf("failed to read resource %q: %w", res.Name, err)
	}

	return buf.Bytes(), nil
}

// fetchManifest fetches and decodes desc from store as an OCI image manifest.
func fetchManifest(ctx context.Context, store *tar.CloseableReadOnlyStore, desc ociImageSpecV1.Descriptor) (*ociImageSpecV1.Manifest, error) {
	raw, err := fetchBlob(ctx, store, desc)
	if err != nil {
		return nil, err
	}

	manifest := &ociImageSpecV1.Manifest{}
	if err := json.Unmarshal(raw, manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	return manifest, nil
}

// fetchBlob reads desc out of store in full.
func fetchBlob(ctx context.Context, store *tar.CloseableReadOnlyStore, desc ociImageSpecV1.Descriptor) ([]byte, error) {
	rc, err := store.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", desc.Digest, err)
	}
	defer func() { _ = rc.Close() }()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", desc.Digest, err)
	}

	return buf.Bytes(), nil
}
