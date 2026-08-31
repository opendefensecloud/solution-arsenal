// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
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
	} else if exists && canSkip(cmd, config) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Chart already exists at %s, skipping render and push\n", url)

		return nil
	}

	result, err := render(cmd.Context(), config)
	if err != nil {
		return err
	}
	defer func() { _ = result.Close() }()

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rendered %s to %s\n", config.Type, result.Dir)

	pushResult, err := renderer.PushChart(result, pushOpts)
	if err != nil {
		return fmt.Errorf("failed to push result: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pushed result to %s\n", pushResult.Ref)

	if config.Signing == nil {
		return nil
	}

	// An unsigned artifact must never count as a successful render, so a
	// signing failure fails the whole command and with it the RenderTask.
	if err := renderer.SignChart(buildSignOptions(cmd.Context(), pushResult.Ref, config.Signing)); err != nil {
		return fmt.Errorf("failed to sign artifact: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Signed %s\n", pushResult.Ref)

	return nil
}

// canSkip reports whether the render, push and sign work for an already
// existing chart can be skipped. Without signing, the chart being there is
// enough. With signing, it must already carry a signature made by this task's
// key: each target signs with its own key, so a chart pushed by another target
// carries no signature this target can verify.
func canSkip(cmd *cobra.Command, config solarv1alpha1.RendererConfig) bool {
	if config.Signing == nil {
		return true
	}

	signed, err := renderer.SignatureExists(buildSignOptions(cmd.Context(), url, config.Signing))
	if err != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Could not check for existing signature, proceeding with render: %v\n", err)

		return false
	}

	return signed
}

// buildSignOptions mirrors buildPushOptions for cosign: helm's registry client
// options are opaque, so the same credentials are re-expressed as
// go-containerregistry options. The key password follows cosign's own
// convention of the COSIGN_PASSWORD environment variable.
func buildSignOptions(ctx context.Context, ref string, signing *solarv1alpha1.SigningConfig) renderer.SignOptions {
	opts := renderer.SignOptions{
		Reference:     ref,
		KeyPath:       signing.KeyPath,
		KeyPassword:   []byte(os.Getenv("COSIGN_PASSWORD")),
		RemoteOptions: []ggcrremote.Option{ggcrremote.WithContext(ctx)},
	}

	if plainHTTP {
		opts.NameOptions = append(opts.NameOptions, name.Insecure)
	}

	if username != "" && password != "" {
		opts.RemoteOptions = append(opts.RemoteOptions,
			ggcrremote.WithAuth(&authn.Basic{Username: username, Password: password}))
	} else {
		opts.RemoteOptions = append(opts.RemoteOptions,
			ggcrremote.WithAuthFromKeychain(authn.DefaultKeychain))
	}

	return opts
}

func render(ctx context.Context, config solarv1alpha1.RendererConfig) (*solarv1alpha1.RenderResult, error) {
	switch config.Type {
	case solarv1alpha1.RendererConfigTypeRelease:
		return renderer.RenderRelease(ctx, config.ReleaseConfig, sourceCredentials())
	case solarv1alpha1.RendererConfigTypeBootstrap:
		return renderer.RenderBootstrap(config.BootstrapConfig)
	default:
		return nil, fmt.Errorf("unknown type specified in config: %s", config.Type)
	}
}

// sourceCredentials reads the credentials for the registry holding the OCM
// component being rendered. The RenderTask controller supplies them from the
// source Registry's solarSecretRef, in whichever of the two shapes that Secret
// has. Nil means anonymous access.
func sourceCredentials() *renderer.SourceCredentials {
	if path := os.Getenv("SOURCE_DOCKER_CONFIG"); path != "" {
		return &renderer.SourceCredentials{DockerConfigPath: path}
	}

	username := os.Getenv("SOURCE_REGISTRY_USERNAME")
	password := os.Getenv("SOURCE_REGISTRY_PASSWORD")
	if username == "" || password == "" {
		return nil
	}

	return &renderer.SourceCredentials{
		Username: username,
		Password: password,
	}
}

func renderOnly(cmd *cobra.Command, config solarv1alpha1.RendererConfig) error {
	result, err := render(cmd.Context(), config)
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

	return renderer.PushOptions{
		Reference:     url,
		ClientOptions: clientOpts,
	}
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
