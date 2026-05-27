// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ociregistry_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/registry"

	"go.opendefense.cloud/solar/pkg/ociregistry"
)

// TestDeleteTag_InvalidReference ensures DeleteTag returns an error
// immediately when the reference cannot be parsed, without making any network calls.
func TestDeleteTag_InvalidReference(t *testing.T) {
	err := ociregistry.DeleteTag(context.Background(), "not a valid::ref", authn.Anonymous)
	if err == nil {
		t.Fatal("expected error for invalid reference, got nil")
	}
	if !strings.Contains(err.Error(), "invalid OCI reference") {
		t.Errorf("expected 'invalid OCI reference' in error, got: %v", err)
	}
}

// TestDeleteTag_DeleteSucceeds pushes a manifest to an in-process OCI
// registry and then verifies DeleteTag removes it without error.
func TestDeleteTag_DeleteSucceeds(t *testing.T) {
	// Spin up an in-process OCI Distribution Spec compliant registry.
	srv := httptest.NewServer(registry.New())
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	repo := "testns/myrepo"
	tag := "v1.0.0"

	// Push a minimal manifest so there is something to delete.
	if err := pushMinimalManifest(srv, repo, tag); err != nil {
		t.Fatalf("failed to push test manifest: %v", err)
	}

	ref := fmt.Sprintf("%s/%s:%s", host, repo, tag)
	if err := ociregistry.DeleteTag(context.Background(), ref, authn.Anonymous); err != nil {
		t.Fatalf("DeleteTag returned unexpected error: %v", err)
	}
}

// TestDeleteTag_DeleteReturnsErrorOnRegistryFailure verifies that
// DeleteTag surfaces a non-nil error when the registry returns an HTTP error.
func TestDeleteTag_DeleteReturnsErrorOnRegistryFailure(t *testing.T) {
	// Server that always returns 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	ref := fmt.Sprintf("%s/ns/repo:v1", host)
	err := ociregistry.DeleteTag(context.Background(), ref, authn.Anonymous)
	if err == nil {
		t.Fatal("expected error on registry failure, got nil")
	}
}

// pushMinimalManifest pushes a scratch-level OCI manifest directly via the
// HTTP API of the test server so that a tag exists for DeleteTag to delete.
func pushMinimalManifest(srv *httptest.Server, repo, tag string) error {
	// Push an empty config blob first.
	configBlob := []byte(`{}`)
	configDigest := "sha256:44136fa355ba77b9ad7b35f047fbbf10c6ae5d7ba3c2a1b02e7f4c93e1e6d6d1"

	blobURL := fmt.Sprintf("%s/v2/%s/blobs/uploads/", srv.URL, repo)
	//nolint:gosec // test-only code, URL is controlled by httptest.NewServer
	resp, err := http.Post(blobURL, "application/octet-stream", nil) //nolint:noctx
	if err != nil {
		return fmt.Errorf("initiate blob upload: %w", err)
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	if location == "" {
		putURL := fmt.Sprintf("%s/v2/%s/blobs/%s", srv.URL, repo, configDigest)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, putURL, strings.NewReader(string(configBlob)))
		req.Header.Set("Content-Type", "application/octet-stream")
		r, e := http.DefaultClient.Do(req) //nolint:gosec
		if e != nil {
			return e
		}
		defer r.Body.Close()
	} else {
		loc := location
		if !strings.HasPrefix(loc, "http") {
			loc = srv.URL + loc
		}
		loc = addQueryParam(loc, "digest", configDigest)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, loc, strings.NewReader(string(configBlob)))
		req.Header.Set("Content-Type", "application/octet-stream")
		r, e := http.DefaultClient.Do(req) //nolint:gosec
		if e != nil {
			return e
		}
		defer r.Body.Close()
	}

	// Push a minimal image manifest referencing the config blob.
	manifest := fmt.Sprintf(`{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "config": {
    "mediaType": "application/vnd.oci.image.config.v1+json",
    "digest": %q,
    "size": %d
  },
  "layers": []
}`, configDigest, len(configBlob))

	manifestURL := fmt.Sprintf("%s/v2/%s/manifests/%s", srv.URL, repo, tag)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, manifestURL, strings.NewReader(manifest))
	req.Header.Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
	r, err := http.DefaultClient.Do(req) //nolint:gosec
	if err != nil {
		return fmt.Errorf("push manifest: %w", err)
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		return fmt.Errorf("push manifest returned %d", r.StatusCode)
	}

	return nil
}

func addQueryParam(u, key, value string) string {
	if strings.Contains(u, "?") {
		return u + "&" + key + "=" + value
	}

	return u + "?" + key + "=" + value
}
