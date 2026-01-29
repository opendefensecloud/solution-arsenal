// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/registry"
	"sigs.k8s.io/yaml"
)

// PushOptions contains the configuration for pushing a helm chart to an OCI registry.
type PushOptions struct {
	// ReferenceURL is the OCI registry URL where the chart will be pushed (e.g., oci://registry.example.com/charts/mychart:v0.1.0)
	// Make sure that the tag matches the version in Chart.yaml, otherwise helm will error before pushing.
	ReferenceURL string

	// PlainHTTP allows plain HTTP connections to the registry
	PlainHTTP bool

	// Username for basic authentication to the registry
	Username string

	// Password for basic authentication to the registry
	Password string

	// CertFile is the path to a client certificate file for mTLS
	CertFile string

	// KeyFile is the path to a client key file for mTLS
	KeyFile string

	// CAFile is the path to a CA certificate file for TLS verification
	CAFile string

	// InsecureSkipTLSVerify skips TLS certificate verification
	InsecureSkipTLSVerify bool

	// CredentialsFile is the path to a credentials file for authentication
	CredentialsFile string
}

// PushResult contains the result of a push operation.
type PushResult struct {
	// Ref is the full OCI reference of the pushed chart
	Ref string
}

// PushChart packages a rendered helm chart and pushes it to an OCI registry.
// The RenderResult directory should contain a valid Helm chart (Chart.yaml, values.yaml, templates/).
// The chart is packaged into a .tgz file, then pushed to the specified OCI registry.
//
// Parameters:
//   - result: the RenderResult from RenderRelease containing the chart directory
//   - opts: configuration for the push operation, including registry URL and credentials
//
// Returns:
//   - PushResult: contains the reference and digest of the pushed chart
//   - error: if packaging or pushing fails
func PushChart(result *RenderResult, opts PushOptions) (*PushResult, error) {
	if result == nil || result.Dir == "" {
		return nil, fmt.Errorf("invalid RenderResult: directory is empty")
	}

	if opts.ReferenceURL == "" {
		return nil, fmt.Errorf("registry URL is required")
	}

	// Verify the chart directory exists and contains Chart.yaml
	chartYamlPath := filepath.Join(result.Dir, "Chart.yaml")
	if _, err := os.Stat(chartYamlPath); err != nil {
		return nil, fmt.Errorf("chart directory is invalid: Chart.yaml not found: %w", err)
	}

	// Read Chart.yaml to extract the version
	chartYamlData, err := os.ReadFile(chartYamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Chart.yaml: %w", err)
	}

	var chartMeta map[string]any
	if err := yaml.Unmarshal(chartYamlData, &chartMeta); err != nil {
		return nil, fmt.Errorf("failed to parse Chart.yaml: %w", err)
	}

	version, ok := chartMeta["version"].(string)
	if !ok || version == "" {
		return nil, fmt.Errorf("chart version not found in Chart.yaml")
	}

	// Create a temporary directory for the packaged chart
	tmpDir, err := os.MkdirTemp("", "helm-package")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Package the chart using helm package
	packagePath, err := packageChart(result.Dir, tmpDir, version)
	if err != nil {
		return nil, fmt.Errorf("failed to package chart: %w", err)
	}

	// Push the packaged chart to the OCI registry
	ref, err := pushChartToRegistry(packagePath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to push chart to registry: %w", err)
	}

	return &PushResult{
		Ref: ref,
	}, nil
}

// packageChart packages a helm chart directory into a .tgz file.
// The chart version from Chart.yaml is used during packaging.
func packageChart(chartDir string, outputDir string, version string) (string, error) {
	client := action.NewPackage()
	client.Destination = outputDir
	client.Version = version

	packagedPath, err := client.Run(chartDir, nil)
	if err != nil {
		return "", fmt.Errorf("helm package failed: %w", err)
	}

	return packagedPath, nil
}

// pushChartToRegistry pushes a packaged helm chart to an OCI registry.
// It handles authentication and registry configuration based on PushOptions.
func pushChartToRegistry(packagePath string, opts PushOptions) (string, error) {
	var registryClient *registry.Client
	var err error

	// Handle TLS configuration
	if opts.CertFile != "" && opts.KeyFile != "" {
		// Use certificate-based authentication
		registryClient, err = registry.NewRegistryClientWithTLS(
			os.Stderr,
			opts.CertFile,
			opts.KeyFile,
			opts.CAFile,
			opts.InsecureSkipTLSVerify,
			"",    // registryConfig - empty string to use default
			false, // debug
		)
		if err != nil {
			return "", fmt.Errorf("failed to create registry client with TLS: %w", err)
		}
	} else {
		// Build registry client options
		var clientOpts []registry.ClientOption

		// Add PlainHTTP option if needed
		if opts.PlainHTTP {
			clientOpts = append(clientOpts, registry.ClientOptPlainHTTP())
		}

		// Add basic auth if provided
		if opts.Username != "" && opts.Password != "" {
			clientOpts = append(clientOpts, registry.ClientOptBasicAuth(opts.Username, opts.Password))
		}

		// Add credentials file if provided
		if opts.CredentialsFile != "" {
			clientOpts = append(clientOpts, registry.ClientOptCredentialsFile(opts.CredentialsFile))
		}

		// Create the registry client
		registryClient, err = registry.NewClient(clientOpts...)
		if err != nil {
			return "", fmt.Errorf("failed to create registry client: %w", err)
		}
	}

	return performPush(registryClient, packagePath, opts)
}

// performPush performs the actual push operation to the registry.
func performPush(registryClient *registry.Client, packagePath string, opts PushOptions) (string, error) {
	// Read the packaged chart file
	chartData, err := os.ReadFile(packagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read packaged chart: %w", err)
	}

	// Push the chart to the registry
	pushResult, err := registryClient.Push(chartData, opts.ReferenceURL)
	if err != nil {
		return "", fmt.Errorf("failed to push to registry: %w", err)
	}

	return pushResult.Ref, nil
}
