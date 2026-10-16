// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// ocmBuildSeparator is how OCM's genericocireg stores a semver "+" in an OCI tag.
const ocmBuildSeparator = ".build-"

// resolveTimeout bounds one digest resolution so a hanging registry cannot stall
// the handler stage, whose context lives as long as the pipeline.
const resolveTimeout = 10 * time.Second

// ResolveManifestDigest returns the manifest digest ("sha256:<hex>") of the OCM
// component descriptor stored at <repository>:<version> in registry.
func ResolveManifestDigest(ctx context.Context, registry *solarv1alpha1.Registry, creds *RegistryCredentials, repository, version string) (string, error) {
	repo, err := remote.NewRepository(registry.Spec.Hostname + "/" + repository)
	if err != nil {
		return "", fmt.Errorf("invalid repository %s/%s: %w", registry.Spec.Hostname, repository, err)
	}
	repo.PlainHTTP = registry.Spec.PlainHTTP
	// oras' default clients have no timeout, so always supply one of our own.
	httpClient := &http.Client{Timeout: resolveTimeout}
	authClient := &auth.Client{Client: httpClient}
	if creds != nil {
		authClient.Credential = auth.StaticCredential(registry.Spec.Hostname, auth.Credential{
			Username: creds.Username,
			Password: creds.Password,
		})
	}
	repo.Client = authClient

	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	tag := strings.ReplaceAll(version, "+", ocmBuildSeparator)
	desc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %s:%s: %w", repository, tag, err)
	}

	return desc.Digest.String(), nil
}
