// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"strings"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// registryBindingInfo combines a RegistryBinding with its resolved Registry.
type registryBindingInfo struct {
	binding  *solarv1alpha1.RegistryBinding
	registry *solarv1alpha1.Registry
}

// extractHost returns the host (with optional port) from a repository string.
// It handles both "host/path" and "host:port/path" formats.
func extractHost(repository string) string {
	// Strip oci:// prefix if present
	repo := strings.TrimPrefix(repository, "oci://")

	// Split on first slash to get host portion
	if before, _, ok := strings.Cut(repo, "/"); ok {
		return before
	}

	return repo
}

// rewriteRepository replaces the host in a repository string with the bound
// registry's hostname and optionally prepends a repository prefix.
func rewriteRepository(repository string, targetHostname string, prefix string) string {
	// Strip oci:// prefix if present, we'll work without it
	repo := strings.TrimPrefix(repository, "oci://")

	// Split host from path
	path := ""
	if _, after, ok := strings.Cut(repo, "/"); ok {
		path = after
	}

	// Build new repository: hostname / prefix / path
	parts := []string{targetHostname}
	if prefix != "" {
		parts = append(parts, strings.Trim(prefix, "/"))
	}
	if path != "" {
		parts = append(parts, path)
	}

	return strings.Join(parts, "/")
}

// resolveResources rewrites resource repositories and attaches pull secret names
// based on RegistryBindings for the target.
//
// Algorithm (per ADR 010):
//  1. For each resource, match its repository host against rewrite.sourceEndpoint
//     on each RegistryBinding. If matched, rewrite to the bound Registry's endpoint.
//  2. If no rewrite match, look for a RegistryBinding whose Registry hostname
//     matches the resource host (identity binding). Use its pullSecretName.
//  3. If no matching binding is found, return an error.
func resolveResources(resources map[string]solarv1alpha1.ResourceAccess, bindings []registryBindingInfo) (map[string]solarv1alpha1.ResourceAccess, error) {
	resolved := make(map[string]solarv1alpha1.ResourceAccess, len(resources))

	for name, res := range resources {
		host := extractHost(res.Repository)

		var matched bool

		// First pass: check rewrite bindings
		for _, bi := range bindings {
			if bi.binding.Spec.Rewrite == nil || bi.binding.Spec.Rewrite.SourceEndpoint == "" {
				continue
			}

			if bi.binding.Spec.Rewrite.SourceEndpoint != host {
				continue
			}

			// Rewrite match found
			resolved[name] = solarv1alpha1.ResourceAccess{
				Repository:     rewriteRepository(res.Repository, bi.registry.Spec.Hostname, bi.binding.Spec.Rewrite.RepositoryPrefix),
				Insecure:       bi.registry.Spec.PlainHTTP,
				Tag:            res.Tag,
				PullSecretName: bi.registry.Spec.TargetPullSecretName,
			}

			matched = true

			break
		}

		if matched {
			continue
		}

		// Second pass: identity binding (registry hostname matches resource host)
		for _, bi := range bindings {
			registryHost := bi.registry.Spec.Hostname

			// Normalize: compare just hostnames, ignoring schemes
			registryHost = strings.TrimPrefix(registryHost, "https://")
			registryHost = strings.TrimPrefix(registryHost, "http://")
			registryHost, _, _ = strings.Cut(registryHost, "/")

			if registryHost != host {
				continue
			}

			resolved[name] = solarv1alpha1.ResourceAccess{
				Repository:     res.Repository,
				Insecure:       res.Insecure,
				Tag:            res.Tag,
				PullSecretName: bi.registry.Spec.TargetPullSecretName,
			}

			matched = true

			break
		}

		if !matched {
			return nil, fmt.Errorf("resource %q (host %q) has no matching RegistryBinding", name, host)
		}
	}

	return resolved, nil
}

// resolveBootstrapReleases rewrites resolved release chart URLs using the
// RegistryBindings for the target. Bootstrap releases point at the render
// registry, so we match against its hostname.
func resolveBootstrapReleases(releases map[string]solarv1alpha1.ResourceAccess, bindings []registryBindingInfo) map[string]solarv1alpha1.ResourceAccess {
	resolved := make(map[string]solarv1alpha1.ResourceAccess, len(releases))

	for name, res := range releases {
		host := extractHost(res.Repository)

		matched := false

		for _, bi := range bindings {
			registryHost := bi.registry.Spec.Hostname
			registryHost = strings.TrimPrefix(registryHost, "https://")
			registryHost = strings.TrimPrefix(registryHost, "http://")
			registryHost, _, _ = strings.Cut(registryHost, "/")

			if registryHost != host {
				continue
			}

			resolved[name] = solarv1alpha1.ResourceAccess{
				Repository:     res.Repository,
				Insecure:       res.Insecure,
				Tag:            res.Tag,
				PullSecretName: bi.registry.Spec.TargetPullSecretName,
			}

			matched = true

			break
		}

		if !matched {
			// No binding — pass through without pullSecretName
			resolved[name] = res
		}
	}

	return resolved
}
