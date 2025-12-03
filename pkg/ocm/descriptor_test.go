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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseComponentDescriptor(t *testing.T) {
	t.Parallel()

	validDescriptor := `{
		"meta": {
			"schemaVersion": "v2"
		},
		"component": {
			"name": "github.com/example/mycomponent",
			"version": "1.0.0",
			"provider": {
				"name": "example.com"
			},
			"resources": [
				{
					"name": "myimage",
					"type": "ociImage",
					"version": "1.0.0",
					"access": {
						"type": "ociArtifact",
						"imageReference": "ghcr.io/example/myimage:1.0.0"
					}
				}
			],
			"componentReferences": [
				{
					"name": "dependency",
					"componentName": "github.com/example/dependency",
					"version": "2.0.0"
				}
			],
			"labels": [
				{
					"name": "description",
					"value": "\"A test component\""
				}
			]
		}
	}`

	tests := []struct {
		name    string
		data    string
		wantErr bool
		check   func(t *testing.T, cd *ComponentDescriptor)
	}{
		{
			name:    "valid descriptor",
			data:    validDescriptor,
			wantErr: false,
			check: func(t *testing.T, cd *ComponentDescriptor) {
				assert.Equal(t, "v2", cd.Meta.SchemaVersion)
				assert.Equal(t, "github.com/example/mycomponent", cd.Component.Name)
				assert.Equal(t, "1.0.0", cd.Component.Version)
				assert.Equal(t, "example.com", cd.Component.Provider.Name)
				assert.Len(t, cd.Component.Resources, 1)
				assert.Equal(t, "myimage", cd.Component.Resources[0].Name)
				assert.Equal(t, ResourceTypeOCIImage, cd.Component.Resources[0].Type)
				assert.Len(t, cd.Component.References, 1)
				assert.Equal(t, "dependency", cd.Component.References[0].Name)
			},
		},
		{
			name:    "invalid JSON",
			data:    "{invalid",
			wantErr: true,
		},
		{
			name:    "empty JSON",
			data:    "{}",
			wantErr: false,
			check: func(t *testing.T, cd *ComponentDescriptor) {
				assert.Empty(t, cd.Meta.SchemaVersion)
				assert.Empty(t, cd.Component.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cd, err := ParseComponentDescriptor([]byte(tt.data))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, cd)
			}
		})
	}
}

func TestParseComponentDescriptorYAML(t *testing.T) {
	t.Parallel()

	validYAML := `
meta:
  schemaVersion: v2
component:
  name: github.com/example/mycomponent
  version: "1.0.0"
  provider:
    name: example.com
  resources:
    - name: myimage
      type: ociImage
      version: "1.0.0"
      access:
        type: ociArtifact
        imageReference: ghcr.io/example/myimage:1.0.0
`

	cd, err := ParseComponentDescriptorYAML([]byte(validYAML))
	require.NoError(t, err)

	assert.Equal(t, "v2", cd.Meta.SchemaVersion)
	assert.Equal(t, "github.com/example/mycomponent", cd.Component.Name)
	assert.Equal(t, "1.0.0", cd.Component.Version)
	assert.Len(t, cd.Component.Resources, 1)
}

func TestComponentDescriptor_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cd      *ComponentDescriptor
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid",
			cd: &ComponentDescriptor{
				Meta: Metadata{SchemaVersion: "v2"},
				Component: ComponentSpec{
					Name:     "github.com/example/component",
					Version:  "1.0.0",
					Provider: Provider{Name: "example.com"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing schema version",
			cd: &ComponentDescriptor{
				Component: ComponentSpec{
					Name:     "github.com/example/component",
					Version:  "1.0.0",
					Provider: Provider{Name: "example.com"},
				},
			},
			wantErr: true,
			errMsg:  "meta.schemaVersion is required",
		},
		{
			name: "missing component name",
			cd: &ComponentDescriptor{
				Meta: Metadata{SchemaVersion: "v2"},
				Component: ComponentSpec{
					Version:  "1.0.0",
					Provider: Provider{Name: "example.com"},
				},
			},
			wantErr: true,
			errMsg:  "component.name is required",
		},
		{
			name: "missing version",
			cd: &ComponentDescriptor{
				Meta: Metadata{SchemaVersion: "v2"},
				Component: ComponentSpec{
					Name:     "github.com/example/component",
					Provider: Provider{Name: "example.com"},
				},
			},
			wantErr: true,
			errMsg:  "component.version is required",
		},
		{
			name: "missing provider",
			cd: &ComponentDescriptor{
				Meta: Metadata{SchemaVersion: "v2"},
				Component: ComponentSpec{
					Name:    "github.com/example/component",
					Version: "1.0.0",
				},
			},
			wantErr: true,
			errMsg:  "component.provider.name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cd.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestComponentDescriptor_GetResource(t *testing.T) {
	t.Parallel()

	cd := &ComponentDescriptor{
		Component: ComponentSpec{
			Resources: []Resource{
				{Name: "image1", Type: ResourceTypeOCIImage},
				{Name: "chart1", Type: ResourceTypeHelmChart},
				{Name: "image2", Type: ResourceTypeOCIImage},
			},
		},
	}

	// Test GetResource
	r, ok := cd.GetResource("chart1")
	require.True(t, ok)
	assert.Equal(t, "chart1", r.Name)
	assert.Equal(t, ResourceTypeHelmChart, r.Type)

	// Test not found
	_, ok = cd.GetResource("notfound")
	assert.False(t, ok)
}

func TestComponentDescriptor_GetResourcesByType(t *testing.T) {
	t.Parallel()

	cd := &ComponentDescriptor{
		Component: ComponentSpec{
			Resources: []Resource{
				{Name: "image1", Type: ResourceTypeOCIImage},
				{Name: "chart1", Type: ResourceTypeHelmChart},
				{Name: "image2", Type: ResourceTypeOCIImage},
			},
		},
	}

	images := cd.GetResourcesByType(ResourceTypeOCIImage)
	assert.Len(t, images, 2)
	assert.Equal(t, "image1", images[0].Name)
	assert.Equal(t, "image2", images[1].Name)

	charts := cd.GetResourcesByType(ResourceTypeHelmChart)
	assert.Len(t, charts, 1)

	blueprints := cd.GetResourcesByType(ResourceTypeBlueprint)
	assert.Len(t, blueprints, 0)
}

func TestComponentDescriptor_GetReference(t *testing.T) {
	t.Parallel()

	cd := &ComponentDescriptor{
		Component: ComponentSpec{
			References: []Reference{
				{Name: "dep1", ComponentName: "github.com/example/dep1", Version: "1.0.0"},
				{Name: "dep2", ComponentName: "github.com/example/dep2", Version: "2.0.0"},
			},
		},
	}

	ref, ok := cd.GetReference("dep2")
	require.True(t, ok)
	assert.Equal(t, "github.com/example/dep2", ref.ComponentName)

	_, ok = cd.GetReference("notfound")
	assert.False(t, ok)
}

func TestLabels_Get(t *testing.T) {
	t.Parallel()

	labels := Labels{
		{Name: "key1", Value: json.RawMessage(`"value1"`)},
		{Name: "key2", Value: json.RawMessage(`{"nested": "object"}`)},
	}

	val, ok := labels.Get("key1")
	require.True(t, ok)
	assert.Equal(t, json.RawMessage(`"value1"`), val)

	_, ok = labels.Get("notfound")
	assert.False(t, ok)
}

func TestLabels_GetString(t *testing.T) {
	t.Parallel()

	labels := Labels{
		{Name: "string", Value: json.RawMessage(`"hello"`)},
		{Name: "number", Value: json.RawMessage(`123`)},
		{Name: "object", Value: json.RawMessage(`{"key": "value"}`)},
	}

	s, ok := labels.GetString("string")
	require.True(t, ok)
	assert.Equal(t, "hello", s)

	// Number should fail
	_, ok = labels.GetString("number")
	assert.False(t, ok)

	// Object should fail
	_, ok = labels.GetString("object")
	assert.False(t, ok)

	// Not found
	_, ok = labels.GetString("notfound")
	assert.False(t, ok)
}

func TestLabels_ToMap(t *testing.T) {
	t.Parallel()

	labels := Labels{
		{Name: "key1", Value: json.RawMessage(`"value1"`)},
		{Name: "key2", Value: json.RawMessage(`"value2"`)},
		{Name: "nonstring", Value: json.RawMessage(`123`)},
	}

	m := labels.ToMap()
	assert.Len(t, m, 2)
	assert.Equal(t, "value1", m["key1"])
	assert.Equal(t, "value2", m["key2"])
	_, ok := m["nonstring"]
	assert.False(t, ok)
}
