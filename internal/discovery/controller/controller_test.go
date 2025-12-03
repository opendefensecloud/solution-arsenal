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

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefensecloud/solution-arsenal/pkg/ocm"
	"github.com/opendefensecloud/solution-arsenal/pkg/registry/oci"
)

func TestNewController(t *testing.T) {
	t.Parallel()

	store := NewMemoryCatalogStore()
	registries := []RegistryConfig{
		{Name: "test", URL: "https://ghcr.io"},
	}

	c := NewController(registries, store)
	assert.NotNil(t, c)
	assert.Equal(t, 5*time.Minute, c.scanInterval)
	assert.Equal(t, 5, c.concurrency)
	assert.False(t, c.IsRunning())
}

func TestController_WithOptions(t *testing.T) {
	t.Parallel()

	store := NewMemoryCatalogStore()
	metrics := NewMetrics()

	c := NewController(
		[]RegistryConfig{},
		store,
		WithScanInterval(10*time.Minute),
		WithConcurrency(10),
		WithMetrics(metrics),
	)

	assert.Equal(t, 10*time.Minute, c.scanInterval)
	assert.Equal(t, 10, c.concurrency)
	assert.Same(t, metrics, c.metrics)
}

func TestController_StartStop(t *testing.T) {
	t.Parallel()

	store := NewMemoryCatalogStore()
	c := NewController([]RegistryConfig{}, store, WithScanInterval(100*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())

	// Start in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Start(ctx)
	}()

	// Wait for controller to start
	time.Sleep(50 * time.Millisecond)
	assert.True(t, c.IsRunning())

	// Stop via context
	cancel()

	select {
	case err := <-errCh:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("controller did not stop")
	}

	assert.False(t, c.IsRunning())
}

func TestController_StopMethod(t *testing.T) {
	t.Parallel()

	store := NewMemoryCatalogStore()
	c := NewController([]RegistryConfig{}, store, WithScanInterval(100*time.Millisecond))

	ctx := context.Background()

	// Start in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Start(ctx)
	}()

	// Wait for controller to start
	time.Sleep(50 * time.Millisecond)
	assert.True(t, c.IsRunning())

	// Stop via Stop method
	c.Stop()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("controller did not stop")
	}

	assert.False(t, c.IsRunning())
}

func TestController_DoubleStart(t *testing.T) {
	t.Parallel()

	store := NewMemoryCatalogStore()
	c := NewController([]RegistryConfig{}, store, WithScanInterval(100*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start first instance
	go c.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	// Try to start again
	err := c.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

// mockOCMServer creates a mock OCI registry server with OCM components
func mockOCMServer(t *testing.T, components map[string]*ocm.ComponentDescriptor) *httptest.Server {
	mux := http.NewServeMux()

	// Handle /v2/ ping
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Handle tags list
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		// This is a simplified handler - in real tests you'd route properly
	})

	return httptest.NewServer(mux)
}

func TestController_ScanRepository(t *testing.T) {
	t.Parallel()

	// Create a mock OCI server
	componentDescriptor := &ocm.ComponentDescriptor{
		Meta: ocm.Metadata{SchemaVersion: "v2"},
		Component: ocm.ComponentSpec{
			Name:     "github.com/example/test",
			Version:  "1.0.0",
			Provider: ocm.Provider{Name: "example.com"},
			Resources: []ocm.Resource{
				{
					Name:    "image",
					Type:    ocm.ResourceTypeOCIImage,
					Version: "1.0.0",
					Access: ocm.AccessSpec{
						Type:           ocm.AccessTypeOCIArtifact,
						ImageReference: "ghcr.io/example/image:1.0.0",
					},
				},
			},
		},
	}

	cdJSON, err := json.Marshal(componentDescriptor)
	require.NoError(t, err)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)

		case r.URL.Path == "/v2/example/repo/tags/list":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(oci.TagList{
				Name: "example/repo",
				Tags: []string{"v1.0.0", "latest"},
			})

		case r.URL.Path == "/v2/example/repo/manifests/v1.0.0":
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			json.NewEncoder(w).Encode(oci.Manifest{
				SchemaVersion: 2,
				MediaType:     "application/vnd.oci.image.manifest.v1+json",
				Config: oci.Descriptor{
					MediaType: ocm.MediaTypeComponentDescriptorConfig,
					Digest:    "sha256:config123",
					Size:      100,
				},
				Layers: []oci.Descriptor{
					{
						MediaType: ocm.MediaTypeComponentDescriptorV2,
						Digest:    "sha256:layer123",
						Size:      int64(len(cdJSON)),
					},
				},
			})

		case r.URL.Path == "/v2/example/repo/blobs/sha256:layer123":
			w.Write(cdJSON)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	store := NewMemoryCatalogStore()
	c := NewController(
		[]RegistryConfig{
			{
				Name:         "test",
				URL:          server.URL,
				Repositories: []string{"example/repo"},
				Labels:       map[string]string{"source": "test"},
			},
		},
		store,
		WithScanInterval(time.Hour), // Long interval, we'll trigger manually
	)

	// Run a manual scan
	ctx := context.Background()
	err = c.ScanNow(ctx)
	require.NoError(t, err)

	// Verify item was stored
	assert.Equal(t, 1, store.Count())

	item, err := store.Get(ctx, "github.com/example/test", "1.0.0")
	require.NoError(t, err)

	assert.Equal(t, "github.com/example/test", item.ComponentName)
	assert.Equal(t, "1.0.0", item.Version)
	assert.Equal(t, "example.com", item.Provider)
	assert.Equal(t, "test", item.Labels["source"])
	assert.Len(t, item.Resources, 1)
	assert.Equal(t, "image", item.Resources[0].Name)
}

func TestMemoryCatalogStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewMemoryCatalogStore()

	// Test CreateOrUpdate
	item := &DiscoveredItem{
		ComponentName: "github.com/example/component",
		Version:       "1.0.0",
		Provider:      "example.com",
	}

	err := store.CreateOrUpdate(ctx, item)
	require.NoError(t, err)

	// Test Get
	retrieved, err := store.Get(ctx, "github.com/example/component", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, item.ComponentName, retrieved.ComponentName)
	assert.Equal(t, item.Version, retrieved.Version)

	// Test Get not found
	_, err = store.Get(ctx, "notfound", "1.0.0")
	assert.Error(t, err)

	// Test List
	items, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	// Test Count
	assert.Equal(t, 1, store.Count())

	// Test Clear
	store.Clear()
	assert.Equal(t, 0, store.Count())
}

func TestMetrics(t *testing.T) {
	t.Parallel()

	m := NewMetrics()

	// Initial state
	scans, errors, discovered, duration, lastTime := m.GetStats()
	assert.Equal(t, int64(0), scans)
	assert.Equal(t, int64(0), errors)
	assert.Equal(t, int64(0), discovered)
	assert.Equal(t, time.Duration(0), duration)
	assert.True(t, lastTime.IsZero())

	// Record events
	m.RecordScan(100 * time.Millisecond)
	m.RecordError("test")
	m.RecordError("test")
	m.RecordDiscovery()
	m.RecordDiscovery()
	m.RecordDiscovery()

	scans, errors, discovered, duration, lastTime = m.GetStats()
	assert.Equal(t, int64(1), scans)
	assert.Equal(t, int64(2), errors)
	assert.Equal(t, int64(3), discovered)
	assert.Equal(t, 100*time.Millisecond, duration)
	assert.False(t, lastTime.IsZero())
}

func TestDiscoveredItem_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	item := &DiscoveredItem{
		ComponentName: "github.com/example/test",
		Version:       "1.0.0",
		Repository:    "example/repo",
		Registry:      "https://ghcr.io",
		Provider:      "example.com",
		Description:   "A test component",
		Labels: map[string]string{
			"category": "testing",
		},
		Resources: []ResourceInfo{
			{Name: "image", Type: "ociImage", Version: "1.0.0"},
		},
		Dependencies: []DependencyInfo{
			{Name: "github.com/example/dep", Version: "2.0.0"},
		},
		DiscoveredAt: now,
		Digest:       "sha256:abc123",
		Namespace:    "default",
	}

	assert.Equal(t, "github.com/example/test", item.ComponentName)
	assert.Equal(t, "1.0.0", item.Version)
	assert.Equal(t, "example/repo", item.Repository)
	assert.Equal(t, "https://ghcr.io", item.Registry)
	assert.Equal(t, "example.com", item.Provider)
	assert.Equal(t, "A test component", item.Description)
	assert.Equal(t, "testing", item.Labels["category"])
	assert.Len(t, item.Resources, 1)
	assert.Len(t, item.Dependencies, 1)
	assert.Equal(t, now, item.DiscoveredAt)
	assert.Equal(t, "sha256:abc123", item.Digest)
	assert.Equal(t, "default", item.Namespace)
}

// mockOCIClient for testing parser integration
type mockOCIClient struct {
	manifests map[string]*oci.Manifest
	blobs     map[string][]byte
	tags      map[string][]string
}

func (m *mockOCIClient) Ping(ctx context.Context) error {
	return nil
}

func (m *mockOCIClient) ListTags(ctx context.Context, repository string) ([]string, error) {
	return m.tags[repository], nil
}

func (m *mockOCIClient) GetManifest(ctx context.Context, repository, reference string) (*oci.Manifest, error) {
	key := repository + ":" + reference
	manifest, ok := m.manifests[key]
	if !ok {
		return nil, oci.ErrNotFound
	}
	return manifest, nil
}

func (m *mockOCIClient) GetBlob(ctx context.Context, repository, digest string) (io.ReadCloser, error) {
	key := repository + "@" + digest
	data, ok := m.blobs[key]
	if !ok {
		return nil, oci.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
