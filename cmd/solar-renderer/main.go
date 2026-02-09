// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.opendefense.cloud/solar/pkg/renderer"
	"sigs.k8s.io/yaml"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func newRootCmd() *cobra.Command {
	var (
		pushEnabled bool
	)

	rootCmd := &cobra.Command{
		Use:   "solar-renderer [config-file]",
		Short: "utility to render and push oci charts for SolAr",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to read config-file: %w", err)
			}

			config := solarv1alpha1.RenderTaskSpec{}
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

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Renderered %s to %s\n", config.Type, result.Dir)
			if !pushEnabled {
				return nil
			}

			defer func() { _ = result.Close() }()
			pushResult, err := renderer.PushChart(result, config.PushOptions)
			if err != nil {
				return fmt.Errorf("failed to push result: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pushed result to %s\n", pushResult.Ref)
			return nil
		},
	}

	flags := rootCmd.Flags()
	flags.BoolVar(&pushEnabled, "push", true, "whether the rendered output should be pushed to a registry")

	return rootCmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Printf("Failed with: %s\n", err)
		os.Exit(1)
	}
}
