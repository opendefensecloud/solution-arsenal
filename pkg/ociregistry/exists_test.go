// Copyright BWI GmbH and Solution Arsenal contributors
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
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"go.opendefense.cloud/solar/pkg/ociregistry"
)

// pushRandom writes a random image to host/repo:tag and returns its manifest digest.
func pushRandom(t *testing.T, host, repo, tag string) string {
	t.Helper()
	img, err := random.Image(64, 1)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := name.ParseReference(fmt.Sprintf("%s/%s:%s", host, repo, tag), name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatal(err)
	}
	d, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}

	return d.String()
}

func TestManifestExists_Found(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	digest := pushRandom(t, host, "test/component-descriptors/example.com/a", "v1")

	found, err := ociregistry.ManifestExists(context.Background(), fmt.Sprintf("%s/test/component-descriptors/example.com/a@%s", host, digest), authn.Anonymous, true)
	if err != nil || !found {
		t.Fatalf("expected found, got found=%v err=%v", found, err)
	}
}

func TestManifestExists_MissingIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	pushRandom(t, host, "test/x", "v1") // repo exists, digest below does not
	missing := "sha256:" + strings.Repeat("0", 64)

	found, err := ociregistry.ManifestExists(context.Background(), fmt.Sprintf("%s/test/x@%s", host, missing), authn.Anonymous, true)
	if err != nil || found {
		t.Fatalf("expected (false, nil), got found=%v err=%v", found, err)
	}
}

func TestManifestExists_ServerErrorIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	found, err := ociregistry.ManifestExists(context.Background(), host+"/test/x@sha256:"+strings.Repeat("0", 64), authn.Anonymous, true)
	if err == nil || found {
		t.Fatalf("expected an error, got found=%v err=%v", found, err)
	}
}

// A timeout or cancellation must never read as "missing".
func TestManifestExists_CancelledContextIsAnError(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	found, err := ociregistry.ManifestExists(ctx, host+"/test/x@sha256:"+strings.Repeat("0", 64), authn.Anonymous, true)
	if err == nil || found {
		t.Fatalf("expected an error, got found=%v err=%v", found, err)
	}
}

func TestManifestExists_InvalidReference(t *testing.T) {
	if _, err := ociregistry.ManifestExists(context.Background(), "not a valid::ref", authn.Anonymous, false); err == nil {
		t.Fatal("expected error for invalid reference")
	}
}

// A 404 from the /v2/ ping or a token endpoint (e.g. a misrouted ingress) says
// nothing about the manifest and must not read as "missing".
func TestManifestExists_404OutsideManifestIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	found, err := ociregistry.ManifestExists(context.Background(), host+"/test/x@sha256:"+strings.Repeat("0", 64), authn.Anonymous, true)
	if err == nil || found {
		t.Fatalf("expected an error, got found=%v err=%v", found, err)
	}
}
