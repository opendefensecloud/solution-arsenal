// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"context"
	"embed"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

//go:embed template/release/*
var releaseFS embed.FS

// releaseRenderData is the dot context for the release wrapper chart. It embeds
// ReleaseConfig so templates keep addressing .Input, .Values and .TargetNamespace
// directly, and adds data derived per render.
type releaseRenderData struct {
	solarv1alpha1.ReleaseConfig
	// RenderedValues is the component's helm values template after rendering
	// against the target's registry pull secrets. Empty when the component ships
	// no template, which suppresses the values ConfigMap and its valuesFrom entry.
	RenderedValues string
	// AgentValues are the solar-agent connection values for one Target,
	// emitted as a Secret the HelmRelease reads through valuesFrom. Empty
	// unless the release carries an AgentCredential.
	AgentValues string
}

// RenderRelease renders the release wrapper chart for c. When the component
// ships a helm values template it is fetched from the source registry and
// rendered with the target's registry pull secrets in scope, then emitted as a
// values ConfigMap the generated HelmRelease reads through valuesFrom.
//
// creds may be nil for an anonymous source registry. agentToken is the Target
// credential token; it is required when c.AgentCredential is set.
func RenderRelease(ctx context.Context, c solarv1alpha1.ReleaseConfig, creds *SourceCredentials, agentToken string) (*solarv1alpha1.RenderResult, error) {
	renderedValues, err := renderValuesTemplate(ctx, c, creds)
	if err != nil {
		return nil, err
	}

	return renderRelease(c, renderedValues, agentToken)
}

// renderRelease renders the release wrapper chart. renderedValues is the
// component's helm values template after rendering; empty suppresses the values
// ConfigMap and the HelmRelease's valuesFrom entry.
func renderRelease(c solarv1alpha1.ReleaseConfig, renderedValues, agentToken string) (*solarv1alpha1.RenderResult, error) {
	var agent string
	if c.AgentCredential != nil {
		var err error
		if agent, err = agentValues(c.AgentCredential, agentToken); err != nil {
			return nil, err
		}
	}

	r := renderer{
		OutputName:  "solar-release",
		TemplateFS:  releaseFS,
		TemplateDir: "template/release",
		Data: releaseRenderData{
			ReleaseConfig:  c,
			RenderedValues: renderedValues,
			AgentValues:    agent,
		},
	}

	return r.render()
}
