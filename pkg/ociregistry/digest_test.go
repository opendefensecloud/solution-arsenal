// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ociregistry_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/registry"

	"go.opendefense.cloud/solar/pkg/ociregistry"
)

func TestManifestDigest_ReturnsPushedDigest(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	want := pushRandom(t, host, "charts/release", "v0.0.1-abc")

	got, err := ociregistry.ManifestDigest(context.Background(), fmt.Sprintf("%s/charts/release:v0.0.1-abc", host), authn.Anonymous, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want || !strings.HasPrefix(got, "sha256:") {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestManifestDigest_MissingTagIsAnError(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	pushRandom(t, host, "charts/release", "v1")

	if got, err := ociregistry.ManifestDigest(context.Background(), fmt.Sprintf("%s/charts/release:v2", host), authn.Anonymous, true); err == nil {
		t.Fatalf("expected an error, got %q", got)
	}
}

func TestManifestDigest_InvalidReference(t *testing.T) {
	if _, err := ociregistry.ManifestDigest(context.Background(), "not a ref", authn.Anonymous, false); err == nil {
		t.Fatal("expected an error")
	}
}
