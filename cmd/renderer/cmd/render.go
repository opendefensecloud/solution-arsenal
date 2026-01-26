// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import "github.com/spf13/cobra"

// renderCmd represents the render command
var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "renders a resource",
}

func init() {
	rootCmd.AddCommand(renderCmd)
}
