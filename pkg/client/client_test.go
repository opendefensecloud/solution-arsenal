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

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"

	solarv1alpha1 "github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1"
)

func TestSchemeContainsSolarTypes(t *testing.T) {
	t.Parallel()

	// Verify that all Solar types are registered in the scheme.
	tests := []struct {
		name string
		gvk  schema.GroupVersionKind
	}{
		{
			name: "CatalogItem",
			gvk:  solarv1alpha1.GroupVersion.WithKind("CatalogItem"),
		},
		{
			name: "CatalogItemList",
			gvk:  solarv1alpha1.GroupVersion.WithKind("CatalogItemList"),
		},
		{
			name: "ClusterCatalogItem",
			gvk:  solarv1alpha1.GroupVersion.WithKind("ClusterCatalogItem"),
		},
		{
			name: "ClusterCatalogItemList",
			gvk:  solarv1alpha1.GroupVersion.WithKind("ClusterCatalogItemList"),
		},
		{
			name: "ClusterRegistration",
			gvk:  solarv1alpha1.GroupVersion.WithKind("ClusterRegistration"),
		},
		{
			name: "ClusterRegistrationList",
			gvk:  solarv1alpha1.GroupVersion.WithKind("ClusterRegistrationList"),
		},
		{
			name: "Release",
			gvk:  solarv1alpha1.GroupVersion.WithKind("Release"),
		},
		{
			name: "ReleaseList",
			gvk:  solarv1alpha1.GroupVersion.WithKind("ReleaseList"),
		},
		{
			name: "Sync",
			gvk:  solarv1alpha1.GroupVersion.WithKind("Sync"),
		},
		{
			name: "SyncList",
			gvk:  solarv1alpha1.GroupVersion.WithKind("SyncList"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Check that the type is known to the scheme.
			known := Scheme.Recognizes(tt.gvk)
			assert.True(t, known, "Scheme should recognize %s", tt.gvk)

			// Check that we can create a new instance.
			obj, err := Scheme.New(tt.gvk)
			assert.NoError(t, err)
			assert.NotNil(t, obj)
		})
	}
}

func TestSchemeContainsKubernetesTypes(t *testing.T) {
	t.Parallel()

	// Verify that core Kubernetes types are also available.
	coreGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}

	known := Scheme.Recognizes(coreGVK)
	assert.True(t, known, "Scheme should recognize core Kubernetes types")
}
