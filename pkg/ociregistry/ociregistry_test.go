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
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"

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
	rawRef := fmt.Sprintf("%s/%s:%s", host, repo, tag)
	ref, err := name.ParseReference(rawRef, name.Insecure)
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}
	if err := remote.Write(ref, empty.Image, remote.WithContext(context.Background())); err != nil {
		t.Fatalf("failed to push test manifest: %v", err)
	}

	if err := ociregistry.DeleteTag(context.Background(), rawRef, authn.Anonymous); err != nil {
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
