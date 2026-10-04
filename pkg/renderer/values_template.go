// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-component-model/community/ocm-kit/compver"
	"github.com/open-component-model/community/ocm-kit/helmvalues"
	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/ociregistry"
	"go.opendefense.cloud/solar/pkg/ocmv2"
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

	repo, err := ocmv2.OpenRepository(cvr.BaseURL(), ocmCredentials(creds))
	if err != nil {
		return "", fmt.Errorf("failed to open repository %s: %w", cvr.BaseURL(), err)
	}
	defer func() { _ = repo.Close() }()

	desc, err := repo.GetComponentVersion(ctx, cvr.ComponentName, cvr.Version)
	if err != nil {
		return "", fmt.Errorf("failed to resolve component version %s: %w", cfg.Input.Component.Ref, err)
	}

	return renderValuesFrom(ctx, repo, desc, cfg)
}

// renderValuesFrom fetches and renders the values template from a component version.
func renderValuesFrom(
	ctx context.Context,
	repo *ocmv2.Repository,
	desc *descruntime.Descriptor,
	cfg solarv1alpha1.ReleaseConfig,
) (string, error) {
	tmpl, err := helmvalues.GetHelmValuesTemplate(ctx, repo, desc, cfg.Input.Entrypoint.ResourceName)
	if err != nil {
		// optional, not an error.
		if errors.Is(err, helmvalues.ErrNotFound) {
			return "", nil
		}

		return "", fmt.Errorf("failed to get helm values template: %w", err)
	}

	input, err := renderingInput(desc, repo.BaseURL())
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

// renderingInput builds ocm-kit's rendering input, resolving every resource
// through SolAr's own resolver rather than ocm-kit's.
//
// This keeps .OCIResources identical to what discovery wrote into the catalog,
// including the access form ocm-kit does not resolve on its own: a local blob
// addressed under the component descriptor path. Letting the two paths resolve
// differently is how a chart ends up deployed with an image the catalog never
// listed.
func renderingInput(desc *descruntime.Descriptor, repoBaseURL string) (*helmvalues.RenderingInput, error) {
	resources := make(map[string]helmvalues.ImageReference, len(desc.Component.Resources))

	for i := range desc.Component.Resources {
		res := desc.Component.Resources[i]

		ref, ok, err := ocmv2.ResolveOCIReference(res, repoBaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve OCI reference for resource %s: %w", res.Name, err)
		}
		if !ok {
			continue
		}

		resources[res.Name] = ref
	}

	return &helmvalues.RenderingInput{
		OCIResources: resources,
		Component:    &desc.Component,
	}, nil
}

// ocmCredentials converts the renderer's source credentials into the form
// pkg/ocmv2 opens repositories with. A nil creds yields nil, meaning anonymous
// access; there is no implicit docker-config fallback.
func ocmCredentials(creds *SourceCredentials) *ocmv2.Credentials {
	if creds == nil {
		return nil
	}

	return &ocmv2.Credentials{
		Username:         creds.Username,
		Password:         creds.Password,
		DockerConfigPath: creds.DockerConfigPath,
	}
}
