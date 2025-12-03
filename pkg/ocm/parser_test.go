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

package ocm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefensecloud/solution-arsenal/pkg/registry/oci"
)

// mockOCIClient implements oci.Client for testing
type mockOCIClient struct {
	manifests map[string]*oci.Manifest
	blobs     map[string][]byte
	tags      map[string][]string
}

func newMockOCIClient() *mockOCIClient {
	return &mockOCIClient{
		manifests: make(map[string]*oci.Manifest),
		blobs:     make(map[string][]byte),
		tags:      make(map[string][]string),
	}
}

func (m *mockOCIClient) Ping(ctx context.Context) error {
	return nil
}

func (m *mockOCIClient) ListTags(ctx context.Context, repository string) ([]string, error) {
	tags, ok := m.tags[repository]
	if !ok {
		return nil, oci.ErrNotFound
	}
	return tags, nil
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

func createTestComponentDescriptor() *ComponentDescriptor {
	return &ComponentDescriptor{
		Meta: Metadata{SchemaVersion: "v2"},
		Component: ComponentSpec{
			Name:     "github.com/example/mycomponent",
			Version:  "1.0.0",
			Provider: Provider{Name: "example.com"},
			Resources: []Resource{
				{
					Name:    "myimage",
					Type:    ResourceTypeOCIImage,
					Version: "1.0.0",
					Access: AccessSpec{
						Type:           AccessTypeOCIArtifact,
						ImageReference: "ghcr.io/example/myimage:1.0.0",
					},
				},
			},
		},
	}
}

func TestParser_ParseComponent_JSONLayer(t *testing.T) {
	t.Parallel()

	client := newMockOCIClient()

	// Create test component descriptor
	cd := createTestComponentDescriptor()
	cdJSON, err := json.Marshal(cd)
	require.NoError(t, err)

	// Set up manifest with JSON layer
	client.manifests["myorg/myrepo:v1.0.0"] = &oci.Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: oci.Descriptor{
			MediaType: MediaTypeComponentDescriptorConfig,
			Digest:    "sha256:config123",
			Size:      100,
		},
		Layers: []oci.Descriptor{
			{
				MediaType: MediaTypeComponentDescriptorV2,
				Digest:    "sha256:layer123",
				Size:      int64(len(cdJSON)),
			},
		},
	}

	client.blobs["myorg/myrepo@sha256:layer123"] = cdJSON

	parser := NewParser(client)
	result, err := parser.ParseComponent(context.Background(), "myorg/myrepo", "v1.0.0")
	require.NoError(t, err)

	assert.Equal(t, "github.com/example/mycomponent", result.Descriptor.Component.Name)
	assert.Equal(t, "1.0.0", result.Descriptor.Component.Version)
	assert.Equal(t, "myorg/myrepo", result.Repository)
	assert.Equal(t, "v1.0.0", result.Tag)
}

func TestParser_ParseComponent_TarGzLayer(t *testing.T) {
	t.Parallel()

	client := newMockOCIClient()

	// Create test component descriptor as YAML
	cdYAML := `meta:
  schemaVersion: v2
component:
  name: github.com/example/tarcomponent
  version: "2.0.0"
  provider:
    name: example.com
  resources: []
`

	// Create tar.gz blob
	var tarGzBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&tarGzBuf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add component-descriptor.yaml to tar
	header := &tar.Header{
		Name: ComponentDescriptorFileName,
		Size: int64(len(cdYAML)),
		Mode: 0644,
	}
	require.NoError(t, tarWriter.WriteHeader(header))
	_, err := tarWriter.Write([]byte(cdYAML))
	require.NoError(t, err)
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzWriter.Close())

	// Set up manifest with tar.gz layer
	client.manifests["myorg/tarrepo:v2.0.0"] = &oci.Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: oci.Descriptor{
			MediaType: MediaTypeComponentDescriptorConfig,
			Digest:    "sha256:config456",
			Size:      100,
		},
		Layers: []oci.Descriptor{
			{
				MediaType: MediaTypeOCMComponentLayer,
				Digest:    "sha256:tarlayer456",
				Size:      int64(tarGzBuf.Len()),
			},
		},
	}

	client.blobs["myorg/tarrepo@sha256:tarlayer456"] = tarGzBuf.Bytes()

	parser := NewParser(client)
	result, err := parser.ParseComponent(context.Background(), "myorg/tarrepo", "v2.0.0")
	require.NoError(t, err)

	assert.Equal(t, "github.com/example/tarcomponent", result.Descriptor.Component.Name)
	assert.Equal(t, "2.0.0", result.Descriptor.Component.Version)
}

func TestParser_ParseComponent_FromConfig(t *testing.T) {
	t.Parallel()

	client := newMockOCIClient()

	// Create component descriptor embedded in config
	cd := createTestComponentDescriptor()
	cd.Component.Name = "github.com/example/configcomponent"
	configData := struct {
		ComponentDescriptor *ComponentDescriptor `json:"componentDescriptor"`
	}{
		ComponentDescriptor: cd,
	}
	configJSON, err := json.Marshal(configData)
	require.NoError(t, err)

	// Set up manifest without descriptor layer - descriptor is in config
	client.manifests["myorg/configrepo:v3.0.0"] = &oci.Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: oci.Descriptor{
			MediaType: MediaTypeComponentDescriptorConfig,
			Digest:    "sha256:config789",
			Size:      int64(len(configJSON)),
		},
		Layers: []oci.Descriptor{
			{
				MediaType: "application/octet-stream",
				Digest:    "sha256:otherlayer",
				Size:      100,
			},
		},
	}

	client.blobs["myorg/configrepo@sha256:config789"] = configJSON

	parser := NewParser(client)
	result, err := parser.ParseComponent(context.Background(), "myorg/configrepo", "v3.0.0")
	require.NoError(t, err)

	assert.Equal(t, "github.com/example/configcomponent", result.Descriptor.Component.Name)
}

func TestParser_ParseComponent_NotOCM(t *testing.T) {
	t.Parallel()

	client := newMockOCIClient()

	// Set up a non-OCM manifest
	client.manifests["myorg/notocm:v1.0.0"] = &oci.Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: oci.Descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    "sha256:notocmconfig",
			Size:      100,
		},
		Layers: []oci.Descriptor{
			{
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
				Digest:    "sha256:notocmlayer",
				Size:      100,
			},
		},
	}

	parser := NewParser(client)
	_, err := parser.ParseComponent(context.Background(), "myorg/notocm", "v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an OCM component")
}

func TestParser_ParseComponent_NotFound(t *testing.T) {
	t.Parallel()

	client := newMockOCIClient()
	parser := NewParser(client)

	_, err := parser.ParseComponent(context.Background(), "myorg/notfound", "v1.0.0")
	require.Error(t, err)
}

func TestParser_ListComponents(t *testing.T) {
	t.Parallel()

	client := newMockOCIClient()
	client.tags["myorg/myrepo"] = []string{
		"v1.0.0",
		"v1.1.0",
		"v2.0.0-beta.1",
		"latest",
		"main",
		"1.2.3",
		"dev",
	}

	parser := NewParser(client)
	versions, err := parser.ListComponents(context.Background(), "myorg/myrepo")
	require.NoError(t, err)

	// Should include version-like tags, exclude latest/main/dev
	assert.Contains(t, versions, "v1.0.0")
	assert.Contains(t, versions, "v1.1.0")
	assert.Contains(t, versions, "v2.0.0-beta.1")
	assert.Contains(t, versions, "1.2.3")
	assert.NotContains(t, versions, "latest")
	assert.NotContains(t, versions, "main")
	assert.NotContains(t, versions, "dev")
}

func TestIsVersionTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag  string
		want bool
	}{
		{"v1.0.0", true},
		{"v0.1.0", true},
		{"1.0.0", true},
		{"0.1.0-alpha", true},
		{"v2.0.0-beta.1", true},
		{"latest", false},
		{"main", false},
		{"master", false},
		{"dev", false},
		{"feature-branch", false},
		{"vx.y.z", false}, // starts with v but not followed by digit
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()
			got := isVersionTag(tt.tag)
			assert.Equal(t, tt.want, got, "isVersionTag(%q)", tt.tag)
		})
	}
}

func TestIsOCMManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest *oci.Manifest
		want     bool
	}{
		{
			name: "OCM config media type",
			manifest: &oci.Manifest{
				Config: oci.Descriptor{
					MediaType: MediaTypeComponentDescriptorConfig,
				},
			},
			want: true,
		},
		{
			name: "OCM layer media type",
			manifest: &oci.Manifest{
				Config: oci.Descriptor{
					MediaType: "application/vnd.oci.image.config.v1+json",
				},
				Layers: []oci.Descriptor{
					{MediaType: MediaTypeComponentDescriptorV2},
				},
			},
			want: true,
		},
		{
			name: "OCM annotations",
			manifest: &oci.Manifest{
				Config: oci.Descriptor{
					MediaType: "application/vnd.oci.image.config.v1+json",
				},
				Annotations: map[string]string{
					"software.ocm.componentName": "github.com/example/component",
				},
			},
			want: true,
		},
		{
			name: "non-OCM manifest",
			manifest: &oci.Manifest{
				Config: oci.Descriptor{
					MediaType: "application/vnd.oci.image.config.v1+json",
				},
				Layers: []oci.Descriptor{
					{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isOCMManifest(tt.manifest)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsComponentDescriptorLayer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		layer *oci.Descriptor
		want  bool
	}{
		{
			name:  "JSON media type",
			layer: &oci.Descriptor{MediaType: MediaTypeComponentDescriptorV2},
			want:  true,
		},
		{
			name:  "YAML media type",
			layer: &oci.Descriptor{MediaType: MediaTypeComponentDescriptorV2Yaml},
			want:  true,
		},
		{
			name:  "tar+gzip media type",
			layer: &oci.Descriptor{MediaType: MediaTypeOCMComponentLayer},
			want:  true,
		},
		{
			name: "annotation title",
			layer: &oci.Descriptor{
				MediaType:   "application/octet-stream",
				Annotations: map[string]string{"org.opencontainers.image.title": ComponentDescriptorFileName},
			},
			want: true,
		},
		{
			name:  "generic layer",
			layer: &oci.Descriptor{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isComponentDescriptorLayer(tt.layer)
			assert.Equal(t, tt.want, got)
		})
	}
}
