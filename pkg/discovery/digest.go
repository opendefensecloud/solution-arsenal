// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// ocmBuildSeparator is how OCM's genericocireg stores a semver "+" in an OCI tag.
const ocmBuildSeparator = ".build-"

// ResolveManifestDigest returns the manifest digest ("sha256:<hex>") of the OCM
// component descriptor stored at <repository>:<version> in registry.
func ResolveManifestDigest(ctx context.Context, registry *solarv1alpha1.Registry, creds *RegistryCredentials, repository, version string) (string, error) {
	repo, err := remote.NewRepository(registry.Spec.Hostname + "/" + repository)
	if err != nil {
		return "", fmt.Errorf("invalid repository %s/%s: %w", registry.Spec.Hostname, repository, err)
	}
	repo.PlainHTTP = registry.Spec.PlainHTTP
	if creds != nil {
		repo.Client = &auth.Client{
			Client: http.DefaultClient,
			Credential: auth.StaticCredential(registry.Spec.Hostname, auth.Credential{
				Username: creds.Username,
				Password: creds.Password,
			}),
		}
	}

	tag := strings.ReplaceAll(version, "+", ocmBuildSeparator)
	desc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %s:%s: %w", repository, tag, err)
	}

	return desc.Digest.String(), nil
}
