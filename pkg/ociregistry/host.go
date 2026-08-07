// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ociregistry

import "strings"

// Host extracts the registry host from a repository string and normalises it to
// lower-case (hostnames are case-insensitive per RFC 4343). For example,
// "Registry.Example.COM:5000/foo/bar" returns "registry.example.com:5000".
func Host(repository string) string {
	repo := strings.TrimPrefix(repository, "oci://")
	if before, _, ok := strings.Cut(repo, "/"); ok {
		return strings.ToLower(before)
	}

	return strings.ToLower(repo)
}
