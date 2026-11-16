// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ocmv2

import (
	"context"
	"errors"
	"net/http"
	"testing"

	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	ociaccessv1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
	"ocm.software/open-component-model/bindings/go/runtime"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

func TestAuthClientScopesBasicCredentials(t *testing.T) {
	const host = "registry.example.com:5000"
	client, err := authClient(host, &Credentials{Username: "user", Password: "password"})
	if err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{host, "other.example.com:5000", "registry.example.com", "registry.example.com:5001"} {
		t.Run(target, func(t *testing.T) {
			got, err := client.Credential(context.Background(), target)
			if err != nil {
				t.Fatal(err)
			}
			want := auth.EmptyCredential
			if target == host {
				want = auth.Credential{Username: "user", Password: "password"}
			}
			if got != want {
				t.Fatal("credentials did not match the expected host scope")
			}
		})
	}
}

func TestResourceContentRequestURL(t *testing.T) {
	for _, tt := range []struct {
		name                string
		componentRepository string
		resourceReference   string
		wantRequestURL      string
	}{
		{
			name:                "external resource uses HTTPS when component registry uses HTTP",
			componentRepository: "http://registry.example.com/copied",
			resourceReference:   "ghcr.io/upstream/chart:1.0",
			wantRequestURL:      "https://ghcr.io/v2/upstream/chart/manifests/1.0",
		},
		{
			name:                "external resource uses HTTPS when component registry uses HTTPS",
			componentRepository: "https://registry.example.com/copied",
			resourceReference:   "ghcr.io/upstream/chart:1.0",
			wantRequestURL:      "https://ghcr.io/v2/upstream/chart/manifests/1.0",
		},
		{
			name:                "copied resource is reanchored to component registry using HTTP",
			componentRepository: "http://registry.example.com/copied",
			resourceReference:   "localhost:5000/copied/chart:1.0",
			wantRequestURL:      "http://registry.example.com/v2/copied/chart/manifests/1.0",
		},
		{
			name:                "copied resource is reanchored to component registry using HTTPS",
			componentRepository: "https://registry.example.com/copied",
			resourceReference:   "localhost:5000/copied/chart:1.0",
			wantRequestURL:      "https://registry.example.com/v2/copied/chart/manifests/1.0",
		},
		{
			name:                "resource already in component registry inherits HTTP",
			componentRepository: "http://registry.example.com/copied",
			resourceReference:   "registry.example.com/copied/chart:1.0",
			wantRequestURL:      "http://registry.example.com/v2/copied/chart/manifests/1.0",
		},
		{
			name:                "resource already in component registry inherits HTTPS",
			componentRepository: "https://registry.example.com/copied",
			resourceReference:   "registry.example.com/copied/chart:1.0",
			wantRequestURL:      "https://registry.example.com/v2/copied/chart/manifests/1.0",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requestURL string
			errStopDownload := errors.New("stop download after capturing request")
			originalClient := retry.DefaultClient
			t.Cleanup(func() { retry.DefaultClient = originalClient })
			retry.DefaultClient = &http.Client{Transport: resourceTransportFunc(func(req *http.Request) (*http.Response, error) {
				requestURL = req.URL.String()

				return nil, errStopDownload
			})}

			repo, err := OpenRepository(tt.componentRepository, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := repo.Close(); err != nil {
					t.Error(err)
				}
			})
			access := &ociaccessv1.OCIImage{ImageReference: tt.resourceReference}
			access.Type = runtime.NewVersionedType(ociaccessv1.OCIImageType, ociaccessv1.Version)

			_, err = repo.resourceContent(t.Context(), nil, descruntime.Resource{Access: access})
			if !errors.Is(err, errStopDownload) {
				t.Fatalf("expected download to reach the capturing transport, got %v", err)
			}
			if requestURL != tt.wantRequestURL {
				t.Errorf("resource download URL = %q, want %q", requestURL, tt.wantRequestURL)
			}
		})
	}
}

type resourceTransportFunc func(*http.Request) (*http.Response, error)

func (f resourceTransportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
