// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"context"
	"errors"
	"fmt"
	"net"

	"go.opendefense.cloud/ocm-kit/compver"
	"go.opendefense.cloud/ocm-kit/helmvalues"
	"ocm.software/ocm/api/credentials"
	"ocm.software/ocm/api/credentials/extensions/repositories/dockerconfig"
	ocireg "ocm.software/ocm/api/oci/extensions/repositories/ocireg"
	"ocm.software/ocm/api/ocm"
	ocmreg "ocm.software/ocm/api/ocm/extensions/repositories/ocireg"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/ociregistry"
)

// SourceCredentials are the credentials for reading the OCM component a release
// is built from. A nil *SourceCredentials means anonymous access.
//
// Exactly one shape is populated, matching the shape of the Registry's
// solarSecretRef: either basic auth, or a path to a docker config file.
type SourceCredentials struct {
	Username string
	Password string `json:"-"`
	// DockerConfigPath points at a docker config file holding the source
	// registry's credentials. Used when solarSecretRef is a
	// kubernetes.io/dockerconfigjson Secret.
	DockerConfigPath string
}

// renderValuesTemplate resolves the OCM component behind cfg.Input.Component.Ref
// and renders the helm values template it ships, with the target's registry pull
// secrets in scope so charts can emit imagePullSecrets.
//
// It returns "" when the release carries no component reference or the component
// ships no values template.
func renderValuesTemplate(ctx context.Context, cfg solarv1alpha1.ReleaseConfig, creds *SourceCredentials) (string, error) {
	if cfg.Input.Component.Ref == "" {
		return "", nil
	}

	cvr, err := compver.SplitRef(cfg.Input.Component.Ref)
	if err != nil {
		return "", fmt.Errorf("failed to parse component reference: %w", err)
	}

	octx, err := ocmContextWithCreds(ctx, cvr.Host, creds)
	if err != nil {
		return "", err
	}

	repo, err := octx.RepositoryForSpec(ocmreg.NewRepositorySpec(cvr.BaseURL()))
	if err != nil {
		return "", fmt.Errorf("failed to create repository spec for %s: %w", cvr.BaseURL(), err)
	}
	defer func() { _ = repo.Close() }()

	compVersion, err := repo.LookupComponentVersion(cvr.ComponentName, cvr.Version)
	if err != nil {
		return "", fmt.Errorf("failed to resolve component version %s: %w", cfg.Input.Component.Ref, err)
	}
	defer func() { _ = compVersion.Close() }()

	return renderValuesFrom(compVersion, cfg)
}

// renderValuesFrom fetches and renders the values template from a component version.
func renderValuesFrom(compVersion ocm.ComponentVersionAccess, cfg solarv1alpha1.ReleaseConfig) (string, error) {
	tmpl, err := helmvalues.GetHelmValuesTemplate(compVersion, cfg.Input.Entrypoint.ResourceName)
	if err != nil {
		// optional, not an error.
		if errors.Is(err, helmvalues.ErrNotFound) {
			return "", nil
		}

		return "", fmt.Errorf("failed to get helm values template: %w", err)
	}

	input, err := helmvalues.GetRenderingInput(compVersion)
	if err != nil {
		return "", fmt.Errorf("failed to build helm values rendering input: %w", err)
	}

	input.PullSecrets = pullSecretsFrom(cfg.Input)

	// Validate the output here rather than letting invalid YAML surface later,
	// when Flux tries to consume the generated ConfigMap.
	rendered, err := helmvalues.Render(tmpl, input, helmvalues.WithYAMLValidation())
	if err != nil {
		return "", fmt.Errorf("failed to render helm values template: %w", err)
	}

	return rendered, nil
}

// pullSecretsFrom builds ocm-kit's registry-host to pull-secret mapping for the
// template.
//
// Two sources are merged. Input.PullSecrets is the target's complete
// RegistryBinding lookup and is authoritative — a values template may call
// pullSecretFor on any host, including one no component resource references, so
// the resource-derived entries alone would leave such a binding unreachable.
// The per-resource names are folded in first so a RenderTask written by an older
// controller, before Input.PullSecrets existed, still resolves the hosts its own
// resources use.
func pullSecretsFrom(input solarv1alpha1.ReleaseInput) helmvalues.PullSecrets {
	secrets := helmvalues.PullSecrets{}

	for _, res := range input.Resources {
		if res.PullSecretName == "" {
			continue
		}

		secrets[ociregistry.Host(res.Repository)] = res.PullSecretName
	}

	for host, name := range input.PullSecrets {
		if name == "" {
			continue
		}

		secrets[ociregistry.Host(host)] = name
	}

	return secrets
}

// ocmContextWithCreds returns an OCM context with creds registered for host.
//
// Without credentials the registry is accessed anonymously. There is no implicit
// docker-config fallback, a docker config must be passed explicitly via DockerConfigPath.
//
// host may or may not carry a port; the consumer identity omits the port
// attribute when there is none, so hosts like "ghcr.io" still match.
func ocmContextWithCreds(ctx context.Context, host string, creds *SourceCredentials) (ocm.Context, error) {
	octx := ocm.FromContext(ctx)
	if creds == nil {
		return octx, nil
	}

	if creds.DockerConfigPath != "" {
		// Propagate consumer identities so the file's registry entries are
		// matched against the repository being resolved. A docker config that
		// cannot be loaded is an error, these credentials were requested explicitly.
		spec := dockerconfig.NewRepositorySpec(creds.DockerConfigPath, true)
		if _, err := octx.CredentialsContext().RepositoryForSpec(spec); err != nil {
			return nil, fmt.Errorf("failed to load source docker config %s: %w", creds.DockerConfigPath, err)
		}

		return octx, nil
	}

	if creds.Username == "" {
		return octx, nil
	}

	id := credentials.ConsumerIdentity{
		credentials.ATTR_TYPE: ocireg.Type,
	}

	if hostname, port, err := net.SplitHostPort(host); err == nil {
		id["hostname"] = hostname
		id["port"] = port
	} else {
		id["hostname"] = host
	}

	octx.CredentialsContext().SetCredentialsForConsumer(id, credentials.NewCredentials(map[string]string{
		credentials.ATTR_USERNAME: creds.Username,
		credentials.ATTR_PASSWORD: creds.Password,
	}))

	return octx, nil
}
