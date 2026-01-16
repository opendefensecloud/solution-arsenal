// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.opendefense.cloud/solar/cmd/solar-webhook-server/subcmd/webhook-server"
)

var rootCmd = &cobra.Command{}

func init() {
	rootCmd.AddCommand(webhook_server.NewCommand())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
