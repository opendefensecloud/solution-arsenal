// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"
)

const (
	RendererConfigTypeHydratedTarget RendererConfigType = "hydrated-target"
	RendererConfigTypeRelease        RendererConfigType = "release"
	RendererConfigTypeProfile        RendererConfigType = "profile"
)

// RendererConfigType is the output type of the renderer.
// +enum
type RendererConfigType string

// ChartConfig defines parameters for the rendered chart.
type ChartConfig struct {
	// Name is the name of the chart.
	Name string `json:"name"`
	// Description is the description of the chart.
	Description string `json:"description"`
	// Version is the version of the chart.
	Version string `json:"version"`
	// AppVersion is the version of the app.
	AppVersion string `json:"appVersion"`
}

// RendererConfig defines the configuration for the renderer.
type RendererConfig struct {
	// Type defines the output type of the renderer.
	Type RendererConfigType `json:"type"`
	// ReleaseConfig is a config for a release.
	ReleaseConfig ReleaseConfig `json:"release"`
	// HydratedTargetConfig is a config for a hydrated-target.
	HydratedTargetConfig HydratedTargetConfig `json:"hydrated-target"`
	// PushOptions defines how to push the rendered chart.
	PushOptions PushOptions `json:"push"`
}

// ReleaseConfig defines the render config for a release.
type ReleaseConfig struct {
	// Chart is the ChartConfig for the rendered chart.
	Chart ChartConfig `json:"chart"`
	// Input is the input of the release.
	Input ReleaseInput `json:"input"`
	// Values are additional values to be rendered into the release chart.
	Values runtime.RawExtension `json:"values"`
}

// ReleaseInput defines the inputs to render a release.
type ReleaseInput struct {
	// Component is a reference to the component.
	Component ReleaseComponent `json:"component"`
	// Resources is the map of resources in the component.
	Resources map[string]ResourceAccess `json:"resources"`
	// Entrypoint is the resource to be used as an entrypoint for deployment.
	Entrypoint Entrypoint `json:"entrypoint"`
}

// ReleaseComponent is a reference to a component.
type ReleaseComponent struct {
	// Name is the name of the component.
	Name string `json:"name"`
}

// HydratedTargetConfig defines the render config for a hydrated-target.
type HydratedTargetConfig struct {
	// Chart is the ChartConfig for the rendered chart.
	Chart ChartConfig `json:"chart"`
	// Input is the input of the hydrated-target.
	Input HydratedTargetInput `json:"input"`
}

// HydratedTargetInput defines the inputs to render a hydrated-target.
type HydratedTargetInput struct {
	Releases map[string]ResourceAccess `json:"releases"` // NOTE: This should be Profiles eventually
	// Userdata is additional data to be rendered into the hydrated-target chart values.
	Userdata runtime.RawExtension `json:"userdata"`
}

// PushOptions contains the configuration for pushing a helm chart to an OCI registry.
type PushOptions struct {
	// ReferenceURL is the OCI registry URL where the chart will be pushed (e.g., oci://registry.example.com/charts/mychart:v0.1.0)
	// Make sure that the tag matches the version in Chart.yaml, otherwise helm will error before pushing.
	ReferenceURL string `json:"referenceURL,omitempty"`

	// PlainHTTP allows plain HTTP connections to the registry
	PlainHTTP bool `json:"plainHTTP,omitempty"`

	// Username for basic authentication to the registry
	Username string `json:"username,omitempty"`

	// Password for basic authentication to the registry
	Password string `json:"password,omitempty"`

	// CredentialsFile is the path to a credentials file for authentication
	CredentialsFile string `json:"credentialsFile,omitempty"`
}

// RenderResult defines the Result of a render operation.
type RenderResult struct {
	// Dir is the directory the chart was rendered to.
	Dir string `json:"dir"`
}

func (r *RenderResult) Close() error {
	return os.RemoveAll(r.Dir)
}

// PushResult contains the result of a push operation.
type PushResult struct {
	// Ref is the full OCI reference of the pushed chart
	Ref string `json:"ref"`
}
