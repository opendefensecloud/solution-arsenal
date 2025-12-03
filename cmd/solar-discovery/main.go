/*
Copyright 2024 Open Defense Cloud Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package main is the entrypoint for solar-discovery, which scans OCI
// registries for OCM packages and creates CatalogItem resources.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/opendefensecloud/solution-arsenal/internal/discovery/config"
	"github.com/opendefensecloud/solution-arsenal/internal/discovery/controller"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	cmd := newRootCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "solar-discovery",
		Short: "Solar Discovery - OCI Registry Scanner for OCM Components",
		Long: `Solar Discovery scans OCI registries for Open Component Model (OCM)
packages and creates CatalogItem resources in the solar-index API.

It periodically scans configured registries, parses OCM component descriptors,
and synchronizes the discovered components to the catalog.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildTime),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(configFile)
		},
	}

	cmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to configuration file")

	// Add subcommands
	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newValidateCommand())

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("solar-discovery %s\n", version)
			fmt.Printf("  commit:  %s\n", commit)
			fmt.Printf("  built:   %s\n", buildTime)
		},
	}
}

func newValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [config-file]",
		Short: "Validate configuration file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(args[0])
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("validating config: %w", err)
			}

			fmt.Println("Configuration is valid")
			fmt.Printf("  Registries: %d\n", len(cfg.Registries))
			fmt.Printf("  Scan interval: %s\n", cfg.ScanInterval.Duration())
			fmt.Printf("  Concurrency: %d\n", cfg.Concurrency)

			return nil
		},
	}
}

func run(configFile string) error {
	// Load configuration
	var cfg *config.Config
	var err error

	if configFile != "" {
		cfg, err = config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	klog.InfoS("Starting solar-discovery",
		"version", version,
		"registries", len(cfg.Registries),
		"scanInterval", cfg.ScanInterval.Duration(),
	)

	// Create catalog store
	// TODO: Replace with actual Kubernetes client store that creates CatalogItems
	store := controller.NewMemoryCatalogStore()

	// Convert configuration to controller config
	registries, opts := cfg.ToControllerConfig()

	// Create and start controller
	ctrl := controller.NewController(registries, store, opts...)

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		klog.InfoS("Received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Start the controller
	if err := ctrl.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("controller error: %w", err)
	}

	klog.InfoS("Solar discovery stopped")
	return nil
}
