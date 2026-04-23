// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/discovery/pipeline"
	_ "go.opendefense.cloud/solar/pkg/discovery/webhook/zot"
)

var cmd = &cobra.Command{
	Use:   "solar-discovery",
	Short: "Scans an OCI registry or receives requests from an OCI registry for relevant OCM packages and writes a coresponding Component or ComponentVersion to a K8s cluster",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if config := cmd.Flag("config").Value.String(); config == "" {
			return fmt.Errorf("config is required")
		}

		return nil
	},
	RunE: runE,
}

func init() {
	cmd.Flags().StringP("listen", "l", "0.0.0.0:8080", "Address to listen on")
	cmd.Flags().StringP("config", "c", "", "Path to configuration file")
	cmd.Flags().StringP("namespace", "n", "default", "Namespace the worker is running in")
}

func runE(cmd *cobra.Command, _ []string) error {

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var log logr.Logger

	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}
	log = zapr.NewLogger(zapLog)
	ctx = logr.NewContext(ctx, log)

	configFilePath := cmd.Flag("config").Value.String()
	if configFilePath == "" {
		return fmt.Errorf("--config is required")
	}

	namespace := cmd.Flag("namespace").Value.String()
	if namespace == "" {
		return fmt.Errorf("--namespace is required")
	}

	registries := discovery.NewRegistryProvider()
	if err := registries.Unmarshal(configFilePath); err != nil {
		return fmt.Errorf("failed to load registries: %w", err)
	}

	addr := cmd.Flag("listen").Value.String()
	if addr == "" {
		addr = "127.0.0.1:8080"
		log.Info(fmt.Sprintf("no listen address specified, using fallback '%s'", addr))
	}

	errChan := make(chan discovery.ErrorEvent, 1)

	p, err := pipeline.NewPipeline(namespace, registries, addr, errChan, log, config.GetConfigOrDie())
	if err != nil {
		return fmt.Errorf("failed to create discovery pipeline: %w", err)
	}
	if err := p.Start(ctx); err != nil {
		return fmt.Errorf("failed to start discovery pipeline: %w", err)
	}

	select {
	case err := <-errChan:
		defer p.Stop(ctx)
		return fmt.Errorf("non-recoverable error occurred in discovery pipeline: %w", err.Error)
	case <-ctx.Done():
		p.Stop(ctx)
	}

	return nil
}

func main() {
	if err := cmd.Execute(); err != nil {
		if _, err := fmt.Fprintln(os.Stderr, err); err != nil {
			panic(err)
		}

		os.Exit(1)
	}
}
