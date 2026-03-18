// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"
)

const (
	RendererConfigTypeBootstrap RendererConfigType = "bootstrap"
	RendererConfigTypeRelease   RendererConfigType = "release"
	RendererConfigTypeProfile   RendererConfigType = "profile"
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
	// BootstrapConfig is a config for a bootstrap.
	BootstrapConfig BootstrapConfig `json:"bootstrap"`
}

// ReleaseConfig defines the render config for a release.
type ReleaseConfig struct {
	// Chart is the ChartConfig for the rendered chart.
	Chart ChartConfig `json:"chart"`
	// Input is the input of the release.
	Input ReleaseInput `json:"input"`
	// TargetNamespace is the namespace the Component gets dpeloyed to.
	TargetNamespace string `json:"targetNamespace"`
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

// BootstrapConfig defines the render config for a bootstrap.
type BootstrapConfig struct {
	// Chart is the ChartConfig for the rendered chart.
	Chart ChartConfig `json:"chart"`
	// Input is the input of the bootstrap.
	Input BootstrapInput `json:"input"`
}

// BootstrapInput defines the inputs to render a bootstrap.
type BootstrapInput struct {
	Releases map[string]ResourceAccess `json:"releases"` // NOTE: This should be Profiles eventually
	// Userdata is additional data to be rendered into the bootstrap chart values.
	Userdata runtime.RawExtension `json:"userdata"`
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
