/*
Copyright 2024 Open Defense Cloud Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oci

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		registry string
		opts     []ClientOption
		want     string
	}{
		{
			name:     "with https prefix",
			registry: "https://ghcr.io",
			want:     "https://ghcr.io",
		},
		{
			name:     "without prefix",
			registry: "ghcr.io",
			want:     "https://ghcr.io",
		},
		{
			name:     "with http prefix",
			registry: "http://localhost:5000",
			want:     "http://localhost:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := NewClient(tt.registry, tt.opts...)
			assert.Equal(t, tt.want, client.registry)
		})
	}
}

func TestClient_Ping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		errContains string
	}{
		{
			name: "successful ping",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v2/", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name: "unauthorized but accessible",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr: false,
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:     true,
			errContains: "unexpected status code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			err := client.Ping(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClient_ListTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		repository string
		handler    http.HandlerFunc
		want       []string
		wantErr    bool
	}{
		{
			name:       "successful list",
			repository: "myorg/myrepo",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v2/myorg/myrepo/tags/list", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(TagList{
					Name: "myorg/myrepo",
					Tags: []string{"v1.0.0", "v1.1.0", "latest"},
				})
			},
			want:    []string{"v1.0.0", "v1.1.0", "latest"},
			wantErr: false,
		},
		{
			name:       "empty tags",
			repository: "myorg/empty",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(TagList{
					Name: "myorg/empty",
					Tags: []string{},
				})
			},
			want:    nil, // Empty slice from JSON unmarshaling becomes nil
			wantErr: false,
		},
		{
			name:       "not found",
			repository: "myorg/notfound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("repository not found"))
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:       "paginated response",
			repository: "myorg/large",
			handler: func() http.HandlerFunc {
				page := 0
				return func(w http.ResponseWriter, r *http.Request) {
					page++
					w.Header().Set("Content-Type", "application/json")
					if page == 1 {
						w.Header().Set("Link", `</v2/myorg/large/tags/list?last=v1.1.0>; rel="next"`)
						json.NewEncoder(w).Encode(TagList{
							Name: "myorg/large",
							Tags: []string{"v1.0.0", "v1.1.0"},
						})
					} else {
						json.NewEncoder(w).Encode(TagList{
							Name: "myorg/large",
							Tags: []string{"v1.2.0", "v1.3.0"},
						})
					}
				}
			}(),
			want:    []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			got, err := client.ListTags(context.Background(), tt.repository)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestClient_GetManifest(t *testing.T) {
	t.Parallel()

	testManifest := &Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: Descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    "sha256:abc123",
			Size:      1234,
		},
		Layers: []Descriptor{
			{
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
				Digest:    "sha256:layer1",
				Size:      5678,
			},
		},
		Annotations: map[string]string{
			"org.opencontainers.image.version": "1.0.0",
		},
	}

	tests := []struct {
		name       string
		repository string
		reference  string
		handler    http.HandlerFunc
		want       *Manifest
		wantErr    bool
	}{
		{
			name:       "successful get by tag",
			repository: "myorg/myrepo",
			reference:  "v1.0.0",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v2/myorg/myrepo/manifests/v1.0.0", r.URL.Path)
				// Check Accept header
				accept := r.Header.Get("Accept")
				assert.Contains(t, accept, "application/vnd.oci.image.manifest.v1+json")
				w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
				json.NewEncoder(w).Encode(testManifest)
			},
			want:    testManifest,
			wantErr: false,
		},
		{
			name:       "successful get by digest",
			repository: "myorg/myrepo",
			reference:  "sha256:abc123",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v2/myorg/myrepo/manifests/sha256:abc123", r.URL.Path)
				w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
				json.NewEncoder(w).Encode(testManifest)
			},
			want:    testManifest,
			wantErr: false,
		},
		{
			name:       "not found",
			repository: "myorg/myrepo",
			reference:  "notfound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			got, err := client.GetManifest(context.Background(), tt.repository, tt.reference)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want.SchemaVersion, got.SchemaVersion)
				assert.Equal(t, tt.want.Config.Digest, got.Config.Digest)
				assert.Len(t, got.Layers, len(tt.want.Layers))
			}
		})
	}
}

func TestClient_GetBlob(t *testing.T) {
	t.Parallel()

	blobContent := []byte(`{"test": "data"}`)

	tests := []struct {
		name       string
		repository string
		digest     string
		handler    http.HandlerFunc
		want       []byte
		wantErr    bool
	}{
		{
			name:       "successful get",
			repository: "myorg/myrepo",
			digest:     "sha256:abc123",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v2/myorg/myrepo/blobs/sha256:abc123", r.URL.Path)
				w.Write(blobContent)
			},
			want:    blobContent,
			wantErr: false,
		},
		{
			name:       "not found",
			repository: "myorg/myrepo",
			digest:     "sha256:notfound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			reader, err := client.GetBlob(context.Background(), tt.repository, tt.digest)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				defer reader.Close()
				got, err := io.ReadAll(reader)
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestClient_WithAuthenticator(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TagList{
			Name: "myorg/myrepo",
			Tags: []string{"v1.0.0"},
		})
	}))
	defer server.Close()

	// Without auth - should fail
	clientNoAuth := NewClient(server.URL)
	_, err := clientNoAuth.ListTags(context.Background(), "myorg/myrepo")
	require.Error(t, err)

	// With auth - should succeed
	clientWithAuth := NewClient(server.URL,
		WithAuthenticator(NewBasicAuthenticator("user", "pass")),
	)
	tags, err := clientWithAuth.ListTags(context.Background(), "myorg/myrepo")
	require.NoError(t, err)
	assert.Equal(t, []string{"v1.0.0"}, tags)
}

func TestClient_WithHTTPClient(t *testing.T) {
	t.Parallel()

	customClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	client := NewClient("https://ghcr.io", WithHTTPClient(customClient))
	assert.Equal(t, customClient, client.httpClient)
}

func TestClient_WithUserAgent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "custom-agent/1.0", r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, WithUserAgent("custom-agent/1.0"))
	err := client.Ping(context.Background())
	require.NoError(t, err)
}

func TestParseLinkHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		link    string
		baseURL string
		want    string
	}{
		{
			name:    "standard link",
			link:    `</v2/repo/tags/list?last=tag>; rel="next"`,
			baseURL: "https://registry.example.com",
			want:    "https://registry.example.com/v2/repo/tags/list?last=tag",
		},
		{
			name:    "no next rel",
			link:    `</v2/repo/tags/list?last=tag>; rel="prev"`,
			baseURL: "https://registry.example.com",
			want:    "",
		},
		{
			name:    "empty link",
			link:    "",
			baseURL: "https://registry.example.com",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseLinkHeader(tt.link, tt.baseURL)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	assert.True(t, IsNotFound(ErrNotFound))
	assert.False(t, IsNotFound(ErrUnauthorized))
	assert.False(t, IsNotFound(nil))
}
