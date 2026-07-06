// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"
	"helm.sh/helm/v4/pkg/registry"
	"sigs.k8s.io/yaml"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/renderer"
)

var (
	skipPush        bool
	url             string
	username        string
	password        string
	passwordStdIn   bool
	plainHTTP       bool
	dockerconfig    string
	signingKey      string
	signingPassword string
)

func rootFunc(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read config-file: %w", err)
	}

	config := solarv1alpha1.RendererConfig{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config-file: %w", err)
	}

	if skipPush {
		return renderOnly(cmd, config)
	}

	if passwordStdIn {
		if _, err := fmt.Scanln(&password); err != nil {
			return err
		}
	}

	pushOpts := buildPushOptions()

	// Check if the chart already exists in the registry before doing any work.
	// This allows multiple targets sharing the same release to create their own
	// RenderTasks without redundant rendering and pushing.
	exists, err := renderer.ChartExists(pushOpts)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Could not check for existing chart, proceeding with render: %v\n", err)
	} else if exists {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Chart already exists at %s, skipping render and push\n", url)

		return nil
	}

	result, err := render(config)
	if err != nil {
		return err
	}
	defer func() { _ = result.Close() }()

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rendered %s to %s\n", config.Type, result.Dir)

	pushResult, err := renderer.PushChart(ctx, result, pushOpts)
	if err != nil {
		return fmt.Errorf("failed to push result: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pushed result to %s\n", pushResult.Ref)

	return nil
}

func render(config solarv1alpha1.RendererConfig) (*solarv1alpha1.RenderResult, error) {
	switch config.Type {
	case solarv1alpha1.RendererConfigTypeRelease:
		return renderer.RenderRelease(config.ReleaseConfig)
	case solarv1alpha1.RendererConfigTypeBootstrap:
		return renderer.RenderBootstrap(config.BootstrapConfig)
	default:
		return nil, fmt.Errorf("unknown type specified in config: %s", config.Type)
	}
}

func renderOnly(cmd *cobra.Command, config solarv1alpha1.RendererConfig) error {
	result, err := render(config)
	if err != nil {
		return fmt.Errorf("failed to render %s: %w", config.Type, err)
	}
	defer func() { _ = result.Close() }()

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rendered %s to %s (skip-push)\n", config.Type, result.Dir)

	return nil
}

func buildPushOptions() renderer.PushOptions {
	dockerconfig, _ = os.LookupEnv("DOCKER_CONFIG")
	if dockerconfig == "" {
		home, _ := os.UserHomeDir()
		dockerconfig = path.Join(home, ".docker", "config.json")
	}

	clientOpts := []registry.ClientOption{}

	if plainHTTP {
		clientOpts = append(clientOpts, registry.ClientOptPlainHTTP())
	}

	// CLI flags take precedence over env vars
	if username == "" {
		username = os.Getenv("REGISTRY_USERNAME")
	}

	if password == "" {
		password = os.Getenv("REGISTRY_PASSWORD")
	}

	// Use basic auth if we have both credentials, otherwise use credentials file
	if username != "" && password != "" {
		clientOpts = append(clientOpts, registry.ClientOptBasicAuth(username, password))
	} else {
		clientOpts = append(clientOpts, registry.ClientOptCredentialsFile(dockerconfig))
	}

	opts := renderer.PushOptions{
		Reference:     url,
		ClientOptions: clientOpts,
		Username:      username,
		Password:      password,
		PlainHTTP:     plainHTTP,
	}

	// Configure signing if a signing key was provided
	if signingKey != "" {
		p := signingPassword
		if p == "" {
			p = os.Getenv("SIGNING_PASSWORD")
		}
		opts.Signing = &renderer.SigningConfig{
			KeyPath:  signingKey,
			Password: p,
		}
	}

	return opts
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "solar-renderer [flags] [config-file]",
		Short: "utility to render and push oci charts for SolAr",
		Args:  cobra.ExactArgs(1),
		RunE:  rootFunc,
	}

	flags := rootCmd.Flags()
	flags.StringVar(&url, "url", "", "url to push the rendered chart to")

	flags.BoolVar(&skipPush, "skip-push", false, "whether the rendered output should be pushed to a registry")
	flags.BoolVar(&plainHTTP, "plain-http", false, "whether to use plain http to push to a registry")
	flags.BoolVar(&passwordStdIn, "password-stdin", false, "read password for basic auth from stdin")

	flags.StringVar(&username, "username", "", "username for basic auth")
	flags.StringVar(&password, "password", "", "password for basic auth")

	flags.StringVar(&signingKey, "signing-key", "", "path to cosign private key for OCI artifact signing")
	flags.StringVar(&signingPassword, "signing-password", "", "password for the cosign private key (SIGNING_PASSWORD env var)")

	return rootCmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		if _, err := fmt.Fprintln(os.Stderr, "Failed with:", err); err != nil {
			panic(err)
		}

		os.Exit(1)
	}
}
