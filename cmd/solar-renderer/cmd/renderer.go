// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.opendefense.cloud/solar/pkg/renderer"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func init() {
	rootCmd.Flags().String("type", "", "Defines the type of resource to render")
	if err := rootCmd.MarkFlagRequired("type"); err != nil {
		fatal(rootCmd, "Could not set flag as required: %v", err) // NOTE: would it be valid to panic?
	}

	rootCmd.Flags().String("config", "", "Path to the solar-renderer config file")
	if err := rootCmd.MarkFlagRequired("config"); err != nil {
		fatal(rootCmd, "Could not set flag as required: %v", err) // NOTE: would it be valid to panic?
	}

	rootCmd.Flags().Bool("push", true, "Wether to push the generated chart after rendering or not")
}

func run(cmd *cobra.Command, args []string) {
	typ, err := cmd.Flags().GetString("type")
	if err != nil {
		fatal(cmd, "Could not get value for flag %v", err)
	}

	rendererConfig, err := getRendererConfig(cmd)
	if err != nil {
		fatal(cmd, "Could not get renderer config: %v", err)
	}

	var result *renderer.RenderResult

	switch typ {
	case "release":
		result, err = renderer.RenderRelease(rendererConfig.ReleaseConfig)
		if err != nil {
			fatal(cmd, "Could not render release: %v", err)
		}
	default:
		fatal(cmd, "Unknown type: %s", typ)
	}

	if result == nil {
		fatal(cmd, "Unexpected empty result")
	}

	push, err := cmd.Flags().GetBool("push")
	if err != nil {
		fatal(cmd, "Could not get flag: %v", err)
	}
	if !push {
		fmt.Printf("Successfully renderered %s to %s\n", typ, result.Dir)
		return
	}

	pushResult, err := renderer.PushChart(result, rendererConfig.PushOptions)
	if err != nil {
		fatal(cmd, "Could not push the result: %v", err)
	}

	if pushResult == nil {
		fatal(cmd, "Unexpected empty push result")
	}
}

func getRendererConfig(cmd *cobra.Command) (*RendererConfig, error) {
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return nil, err
	}

	configData, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	rendererConfig := RendererConfig{}
	if err := yaml.Unmarshal(configData, &rendererConfig); err != nil {
		return nil, err
	}

	return &rendererConfig, nil
}

func fatal(cmd *cobra.Command, format string, i ...any) {
	cmd.PrintErrf(format+"\n", i...)
	os.Exit(1)
}
