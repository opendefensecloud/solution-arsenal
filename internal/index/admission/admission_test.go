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

package admission

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	solarv1alpha1 "github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1"
)

// CatalogItem Validator Tests

func TestCatalogItemValidator_ValidateCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		item      *solarv1alpha1.CatalogItem
		expectErr bool
		errCount  int
	}{
		{
			name: "valid catalog item",
			item: &solarv1alpha1.CatalogItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-item",
					Namespace: "default",
				},
				Spec: solarv1alpha1.CatalogItemSpec{
					ComponentName: "github.com/org/component",
					Version:       "1.0.0",
					Repository:    "ghcr.io/org/repo",
				},
			},
			expectErr: false,
		},
		{
			name: "valid with v prefix version",
			item: &solarv1alpha1.CatalogItem{
				Spec: solarv1alpha1.CatalogItemSpec{
					ComponentName: "github.com/org/component",
					Version:       "v1.2.3",
					Repository:    "ghcr.io/org/repo",
				},
			},
			expectErr: false,
		},
		{
			name: "valid with prerelease version",
			item: &solarv1alpha1.CatalogItem{
				Spec: solarv1alpha1.CatalogItemSpec{
					ComponentName: "github.com/org/component",
					Version:       "1.0.0-beta.1",
					Repository:    "ghcr.io/org/repo",
				},
			},
			expectErr: false,
		},
		{
			name: "missing component name",
			item: &solarv1alpha1.CatalogItem{
				Spec: solarv1alpha1.CatalogItemSpec{
					Version:    "1.0.0",
					Repository: "ghcr.io/org/repo",
				},
			},
			expectErr: true,
			errCount:  1,
		},
		{
			name: "missing version",
			item: &solarv1alpha1.CatalogItem{
				Spec: solarv1alpha1.CatalogItemSpec{
					ComponentName: "github.com/org/component",
					Repository:    "ghcr.io/org/repo",
				},
			},
			expectErr: true,
			errCount:  1,
		},
		{
			name: "missing repository",
			item: &solarv1alpha1.CatalogItem{
				Spec: solarv1alpha1.CatalogItemSpec{
					ComponentName: "github.com/org/component",
					Version:       "1.0.0",
				},
			},
			expectErr: true,
			errCount:  1,
		},
		{
			name: "invalid version format",
			item: &solarv1alpha1.CatalogItem{
				Spec: solarv1alpha1.CatalogItemSpec{
					ComponentName: "github.com/org/component",
					Version:       "not-a-version",
					Repository:    "ghcr.io/org/repo",
				},
			},
			expectErr: true,
			errCount:  1,
		},
		{
			name: "invalid component name",
			item: &solarv1alpha1.CatalogItem{
				Spec: solarv1alpha1.CatalogItemSpec{
					ComponentName: "invalid component name!",
					Version:       "1.0.0",
					Repository:    "ghcr.io/org/repo",
				},
			},
			expectErr: true,
			errCount:  1,
		},
		{
			name: "valid with dependencies",
			item: &solarv1alpha1.CatalogItem{
				Spec: solarv1alpha1.CatalogItemSpec{
					ComponentName: "github.com/org/component",
					Version:       "1.0.0",
					Repository:    "ghcr.io/org/repo",
					Dependencies: []solarv1alpha1.ComponentReference{
						{Name: "github.com/org/dep1", Version: "1.0.0"},
						{Name: "github.com/org/dep2"},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "duplicate dependencies",
			item: &solarv1alpha1.CatalogItem{
				Spec: solarv1alpha1.CatalogItemSpec{
					ComponentName: "github.com/org/component",
					Version:       "1.0.0",
					Repository:    "ghcr.io/org/repo",
					Dependencies: []solarv1alpha1.ComponentReference{
						{Name: "github.com/org/dep1"},
						{Name: "github.com/org/dep1"},
					},
				},
			},
			expectErr: true,
			errCount:  1,
		},
	}

	validator := NewCatalogItemValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := validator.ValidateCreate(context.Background(), tt.item)
			if tt.expectErr {
				assert.NotEmpty(t, errs)
				if tt.errCount > 0 {
					assert.Len(t, errs, tt.errCount)
				}
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestCatalogItemValidator_ValidateUpdate_ImmutableFields(t *testing.T) {
	t.Parallel()

	validator := NewCatalogItemValidator()
	ctx := context.Background()

	oldItem := &solarv1alpha1.CatalogItem{
		Spec: solarv1alpha1.CatalogItemSpec{
			ComponentName: "github.com/org/component",
			Version:       "1.0.0",
			Repository:    "ghcr.io/org/repo",
		},
	}

	// Test changing component name
	newItem := oldItem.DeepCopy()
	newItem.Spec.ComponentName = "github.com/org/other"
	errs := validator.ValidateUpdate(ctx, oldItem, newItem)
	assert.NotEmpty(t, errs)

	// Test changing version
	newItem = oldItem.DeepCopy()
	newItem.Spec.Version = "2.0.0"
	errs = validator.ValidateUpdate(ctx, oldItem, newItem)
	assert.NotEmpty(t, errs)

	// Test changing repository (allowed)
	newItem = oldItem.DeepCopy()
	newItem.Spec.Repository = "ghcr.io/org/other-repo"
	errs = validator.ValidateUpdate(ctx, oldItem, newItem)
	assert.Empty(t, errs)
}

// Release Validator Tests

func TestReleaseValidator_ValidateCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		release   *solarv1alpha1.Release
		expectErr bool
	}{
		{
			name: "valid release",
			release: &solarv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-release",
					Namespace: "default",
				},
				Spec: solarv1alpha1.ReleaseSpec{
					CatalogItemRef:   solarv1alpha1.ObjectReference{Name: "my-item"},
					TargetClusterRef: solarv1alpha1.ObjectReference{Name: "my-cluster"},
				},
			},
			expectErr: false,
		},
		{
			name: "missing catalog item ref",
			release: &solarv1alpha1.Release{
				Spec: solarv1alpha1.ReleaseSpec{
					TargetClusterRef: solarv1alpha1.ObjectReference{Name: "my-cluster"},
				},
			},
			expectErr: true,
		},
		{
			name: "missing target cluster ref",
			release: &solarv1alpha1.Release{
				Spec: solarv1alpha1.ReleaseSpec{
					CatalogItemRef: solarv1alpha1.ObjectReference{Name: "my-item"},
				},
			},
			expectErr: true,
		},
	}

	validator := NewReleaseValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := validator.ValidateCreate(context.Background(), tt.release)
			if tt.expectErr {
				assert.NotEmpty(t, errs)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestReleaseValidator_ValidateUpdate_ImmutableFields(t *testing.T) {
	t.Parallel()

	validator := NewReleaseValidator()
	ctx := context.Background()

	oldRelease := &solarv1alpha1.Release{
		Spec: solarv1alpha1.ReleaseSpec{
			CatalogItemRef:   solarv1alpha1.ObjectReference{Name: "my-item"},
			TargetClusterRef: solarv1alpha1.ObjectReference{Name: "my-cluster", Namespace: "default"},
		},
	}

	// Test changing target cluster name (immutable)
	newRelease := oldRelease.DeepCopy()
	newRelease.Spec.TargetClusterRef.Name = "other-cluster"
	errs := validator.ValidateUpdate(ctx, oldRelease, newRelease)
	assert.NotEmpty(t, errs)

	// Test changing catalog item ref (allowed)
	newRelease = oldRelease.DeepCopy()
	newRelease.Spec.CatalogItemRef.Name = "other-item"
	errs = validator.ValidateUpdate(ctx, oldRelease, newRelease)
	assert.Empty(t, errs)
}

// ClusterRegistration Validator Tests

func TestClusterRegistrationValidator_ValidateCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cr        *solarv1alpha1.ClusterRegistration
		expectErr bool
	}{
		{
			name: "valid cluster registration",
			cr: &solarv1alpha1.ClusterRegistration{
				Spec: solarv1alpha1.ClusterRegistrationSpec{
					DisplayName: "Production Cluster",
					Description: "Main production cluster",
				},
			},
			expectErr: false,
		},
		{
			name: "missing display name",
			cr: &solarv1alpha1.ClusterRegistration{
				Spec: solarv1alpha1.ClusterRegistrationSpec{
					Description: "Some description",
				},
			},
			expectErr: true,
		},
		{
			name: "with labels",
			cr: &solarv1alpha1.ClusterRegistration{
				Spec: solarv1alpha1.ClusterRegistrationSpec{
					DisplayName: "Test Cluster",
					Labels: map[string]string{
						"env":    "prod",
						"region": "us-west-2",
					},
				},
			},
			expectErr: false,
		},
	}

	validator := NewClusterRegistrationValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := validator.ValidateCreate(context.Background(), tt.cr)
			if tt.expectErr {
				assert.NotEmpty(t, errs)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

// ClusterRegistration Mutator Tests

func TestClusterRegistrationMutator_MutateCreate(t *testing.T) {
	t.Parallel()

	mutator := NewClusterRegistrationMutator()
	ctx := context.Background()

	cr := &solarv1alpha1.ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: solarv1alpha1.ClusterRegistrationSpec{
			DisplayName: "Test Cluster",
		},
	}

	err := mutator.MutateCreate(ctx, cr)
	require.NoError(t, err)

	// Should have finalizer added
	assert.Contains(t, cr.Finalizers, "solar.odc.io/cluster-registration-cleanup")
}

func TestClusterRegistrationMutator_MutateCreate_AlreadyHasFinalizer(t *testing.T) {
	t.Parallel()

	mutator := NewClusterRegistrationMutator()
	ctx := context.Background()

	cr := &solarv1alpha1.ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-cluster",
			Namespace:  "default",
			Finalizers: []string{"solar.odc.io/cluster-registration-cleanup"},
		},
		Spec: solarv1alpha1.ClusterRegistrationSpec{
			DisplayName: "Test Cluster",
		},
	}

	err := mutator.MutateCreate(ctx, cr)
	require.NoError(t, err)

	// Should not duplicate finalizer
	count := 0
	for _, f := range cr.Finalizers {
		if f == "solar.odc.io/cluster-registration-cleanup" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestDefaultTokenGenerator(t *testing.T) {
	t.Parallel()

	gen := NewDefaultTokenGenerator(32)

	token1, err := gen.GenerateToken()
	require.NoError(t, err)
	assert.Len(t, token1, 64) // 32 bytes = 64 hex chars

	token2, err := gen.GenerateToken()
	require.NoError(t, err)

	// Tokens should be unique
	assert.NotEqual(t, token1, token2)
}

// Sync Validator Tests

func TestSyncValidator_ValidateCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sync      *solarv1alpha1.Sync
		expectErr bool
	}{
		{
			name: "valid sync",
			sync: &solarv1alpha1.Sync{
				Spec: solarv1alpha1.SyncSpec{
					SourceRef:           solarv1alpha1.ObjectReference{Name: "source-catalog"},
					DestinationRegistry: "ghcr.io/org/dest",
				},
			},
			expectErr: false,
		},
		{
			name: "missing source ref",
			sync: &solarv1alpha1.Sync{
				Spec: solarv1alpha1.SyncSpec{
					DestinationRegistry: "ghcr.io/org/dest",
				},
			},
			expectErr: true,
		},
		{
			name: "missing destination",
			sync: &solarv1alpha1.Sync{
				Spec: solarv1alpha1.SyncSpec{
					SourceRef: solarv1alpha1.ObjectReference{Name: "source-catalog"},
				},
			},
			expectErr: true,
		},
		{
			name: "with filters",
			sync: &solarv1alpha1.Sync{
				Spec: solarv1alpha1.SyncSpec{
					SourceRef:           solarv1alpha1.ObjectReference{Name: "source-catalog"},
					DestinationRegistry: "ghcr.io/org/dest",
					Filter: solarv1alpha1.SyncFilter{
						IncludeLabels: map[string]string{"env": "prod"},
						ExcludeLabels: map[string]string{"deprecated": "true"},
					},
				},
			},
			expectErr: false,
		},
	}

	validator := NewSyncValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := validator.ValidateCreate(context.Background(), tt.sync)
			if tt.expectErr {
				assert.NotEmpty(t, errs)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

// Registry Tests

func TestValidatorRegistry(t *testing.T) {
	t.Parallel()

	registry := NewValidatorRegistry()

	validator := NewCatalogItemValidator()
	registry.Register("CatalogItem", validator)

	got, ok := registry.Get("CatalogItem")
	assert.True(t, ok)
	assert.Equal(t, validator, got)

	_, ok = registry.Get("NonExistent")
	assert.False(t, ok)
}

func TestMutatorRegistry(t *testing.T) {
	t.Parallel()

	registry := NewMutatorRegistry()

	mutator := NewClusterRegistrationMutator()
	registry.Register("ClusterRegistration", mutator)

	got, ok := registry.Get("ClusterRegistration")
	assert.True(t, ok)
	assert.Equal(t, mutator, got)

	_, ok = registry.Get("NonExistent")
	assert.False(t, ok)
}

// Helper function tests

func TestIsValidOCIReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ref   string
		valid bool
	}{
		{"ghcr.io/org/repo", true},
		{"docker.io/library/nginx", true},
		{"localhost:5000/repo", true},
		{"registry.example.com/path/to/repo", true},
		{"ghcr.io/org/repo:tag", true},
		{"ghcr.io/org/repo@sha256:abc123", true},
		{"", false},
		{"invalid", false},
		{"with spaces/repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.valid, isValidOCIReference(tt.ref))
		})
	}
}

// ChainMutator Tests

func TestChainMutator(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create test mutators
	mutator1 := &testMutator{createCalls: 0}
	mutator2 := &testMutator{createCalls: 0}

	chain := NewChainMutator(mutator1, mutator2)

	obj := &solarv1alpha1.ClusterRegistration{
		Spec: solarv1alpha1.ClusterRegistrationSpec{
			DisplayName: "Test",
		},
	}

	err := chain.MutateCreate(ctx, obj)
	require.NoError(t, err)

	assert.Equal(t, 1, mutator1.createCalls)
	assert.Equal(t, 1, mutator2.createCalls)
}

type testMutator struct {
	createCalls int
	updateCalls int
}

func (m *testMutator) MutateCreate(ctx context.Context, obj runtime.Object) error {
	m.createCalls++
	return nil
}

func (m *testMutator) MutateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	m.updateCalls++
	return nil
}
