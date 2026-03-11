// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
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
	skipPush      bool
	url           string
	username      string
	password      string
	passwordStdIn bool
	plainHTTP     bool
	dockerconfig  string
)

func rootFunc(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read config-file: %w", err)
	}

	config := solarv1alpha1.RendererConfig{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config-file: %w", err)
	}

	var result *solarv1alpha1.RenderResult

	switch config.Type {
	case solarv1alpha1.RendererConfigTypeRelease:
		result, err = renderer.RenderRelease(config.ReleaseConfig)
	case solarv1alpha1.RendererConfigTypeHydratedTarget:
		result, err = renderer.RenderHydratedTarget(config.HydratedTargetConfig)
	default:
		return fmt.Errorf("unknown type specified in config: %s", config.Type)
	}
	if err != nil {
		return fmt.Errorf("failed to render %s: %w", config.Type, err)
	}
	defer func() { _ = result.Close() }()

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rendered %s to %s\n", config.Type, result.Dir)
	if skipPush {
		return nil
	}

	if passwordStdIn {
		if _, err := fmt.Scanln(&password); err != nil {
			return err
		}
	}

	dockerconfig, _ = os.LookupEnv("DOCKER_CONFIG")
	if dockerconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dockerconfig = path.Join(home, ".docker", "config.json")
	}

	clientOpts := []registry.ClientOption{}

	if plainHTTP {
		clientOpts = append(clientOpts, registry.ClientOptPlainHTTP())
	}

	// Decide authentication method
	// CLI flags take precedence over env vars
	var authUsername, authPassword string

	if username != "" {
		authUsername = username
	}
	if password != "" {
		authPassword = password
	}

	// Fall back to env vars if flags weren't provided
	if authUsername == "" {
		authUsername = os.Getenv("REGISTRY_USERNAME")
	}
	if authPassword == "" {
		authPassword = os.Getenv("REGISTRY_PASSWORD")
	}

	// Use basic auth if we have both credentials, otherwise use credentials file
	if authUsername != "" && authPassword != "" {
		clientOpts = append(clientOpts, registry.ClientOptBasicAuth(authUsername, authPassword))
	} else {
		clientOpts = append(clientOpts, registry.ClientOptCredentialsFile(dockerconfig))
	}

	po := renderer.PushOptions{
		ReferenceURL:  url,
		ClientOptions: clientOpts,
	}

	pushResult, err := renderer.PushChart(result, po)
	if err != nil {
		return fmt.Errorf("failed to push result: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pushed result to %s\n", pushResult.Ref)

	return nil
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
