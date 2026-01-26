// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opendefense.cloud/solar/pkg/renderer"
)

// releaseCmd represents the release command
var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "renders a release",
	Run:   renderRelease,
}

func init() {
	renderCmd.AddCommand(releaseCmd)
}

func renderRelease(cmd *cobra.Command, args []string) {
	releaseConfig := renderer.ReleaseConfig{}

	if err := viper.UnmarshalKey("release", &releaseConfig); err != nil {
		cmd.PrintErrf("Could not unmarshal release config: %v", err)
		os.Exit(1)
	}

	result, err := renderer.RenderRelease(releaseConfig)
	if err != nil {
		cmd.PrintErrln(err)
		os.Exit(1)
	}

	if result == nil {
		cmd.PrintErrln("Rendered Successfully but result was empty.")
		os.Exit(1)
	}

	fmt.Printf("Successfully rendered release to %s\n", result.Dir)
}
