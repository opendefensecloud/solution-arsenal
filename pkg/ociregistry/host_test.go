// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ociregistry_test

import (
	"testing"

	"go.opendefense.cloud/solar/pkg/ociregistry"
)

func TestHost(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		want       string
	}{
		{"simple host/repo", "registry.example.com/foo/bar", "registry.example.com"},
		{"host with port", "registry.example.com:5000/foo/bar", "registry.example.com:5000"},
		{"oci:// prefix", "oci://registry.example.com/foo/bar", "registry.example.com"},
		{"oci:// prefix with port", "oci://registry.example.com:5000/charts/my-chart", "registry.example.com:5000"},
		{"bare host (no path)", "registry.example.com", "registry.example.com"},
		{"bare host with oci://", "oci://registry.example.com", "registry.example.com"},
		{"deeply nested path", "ghcr.io/org/sub/repo/chart", "ghcr.io"},
		{"uppercase host normalised", "Registry.Example.COM:5000/foo/bar", "registry.example.com:5000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ociregistry.Host(tt.repository); got != tt.want {
				t.Errorf("Host(%q) = %q, want %q", tt.repository, got, tt.want)
			}
		})
	}
}
