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

type RendererConfigType string

type ChartConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	AppVersion  string `json:"appVersion"`
}

type RendererConfig struct {
	Type                 RendererConfigType   `json:"type"`
	ReleaseConfig        ReleaseConfig        `json:"release"`
	HydratedTargetConfig HydratedTargetConfig `json:"hydrated-target"`
	PushOptions          PushOptions          `json:"push"`
}

type ReleaseConfig struct {
	Chart  ChartConfig          `json:"chart"`
	Input  ReleaseInput         `json:"input"`
	Values runtime.RawExtension `json:"values"`
}

type ReleaseInput struct {
	Component ReleaseComponent          `json:"component"`
	Helm      ResourceAccess            `json:"helm"`
	KRO       ResourceAccess            `json:"kro"`
	Resources map[string]ResourceAccess `json:"resources"`
}

type ReleaseComponent struct {
	Name string `json:"name"`
}

type HydratedTargetConfig struct {
	Chart ChartConfig         `json:"chart"`
	Input HydratedTargetInput `json:"input"`
}

type HydratedTargetInput struct {
	Releases map[string]ResourceAccess `json:"releases"` // NOTE: This should be Profiles eventually
	Userdata runtime.RawExtension      `json:"userdata"`
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

	// CertFile is the path to a client certificate file for mTLS
	CertFile string `json:"certFile,omitempty"`

	// KeyFile is the path to a client key file for mTLS
	KeyFile string `json:"keyFile,omitempty"`

	// CAFile is the path to a CA certificate file for TLS verification
	CAFile string `json:"caFile,omitempty"`

	// InsecureSkipTLSVerify skips TLS certificate verification
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`

	// CredentialsFile is the path to a credentials file for authentication
	CredentialsFile string `json:"credentialsFile,omitempty"`
}

type RenderResult struct {
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
