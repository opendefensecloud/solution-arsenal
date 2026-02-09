// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/discovery/handler"
	"go.opendefense.cloud/solar/pkg/discovery/qualifier"
	scanner "go.opendefense.cloud/solar/pkg/discovery/scanner"
	"go.opendefense.cloud/solar/pkg/discovery/webhook"
	_ "go.opendefense.cloud/solar/pkg/discovery/webhook/zot"
)

var cmd = &cobra.Command{
	Use:   "discovery-worker",
	Short: "Solar Webhook Server",
	Long:  "Solar Webhook Server receives incoming webhook requests and passes them as events",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if config := cmd.Flag("config").Value.String(); config == "" {
			return fmt.Errorf("config is required")
		}

		return nil
	},
	RunE: runE,
}

func init() {
	cmd.Flags().StringP("listen", "l", "127.0.0.1:8080", "Address to listen on")
	cmd.Flags().StringP("config", "c", "", "Path to configuration file")
}

func runE(cmd *cobra.Command, _ []string) error {
	ctx, cancelFn := context.WithCancel(cmd.Context())
	defer cancelFn()

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

	registries := discovery.NewRegistryProvider()
	if err := registries.Unmarshall(configFilePath); err != nil {
		log.Error(err, "failed to load registries")
		return err
	}

	repoEventsChan := make(chan discovery.RepositoryEvent, 1000)
	componentVersionEventsChan := make(chan discovery.ComponentVersionEvent, 1000)
	errChan := make(chan discovery.ErrorEvent)

	errGroup, ctx := errgroup.WithContext(ctx)

	httpRouter := webhook.NewWebhookRouter(repoEventsChan)
	httpRouter.WithLogger(log)

	for _, registry := range registries.GetAll() {
		if registry.WebhookPath != "" {
			if err := httpRouter.RegisterPath(registry); err != nil {
				return fmt.Errorf("failed to register handler: %w", err)
			}
		}

		if registry.ScanInterval > 0 {
			regScanner := scanner.NewRegistryScanner(registry, repoEventsChan, errChan,
				scanner.WithScanInterval(registry.ScanInterval),
				scanner.WithLogger(log),
			)
			errGroup.Go(func() error {
				return regScanner.Start(ctx)
			})
		}
	}

	handler := handler.NewHandler(componentVersionEventsChan, errChan, handler.WithLogger(log))
	errGroup.Go(func() error {
		return handler.Start(ctx)
	})

	qual := qualifier.NewQualifier(registries, repoEventsChan, componentVersionEventsChan, errChan, qualifier.WithLogger(log), qualifier.WithRateLimiter(time.Second, 1))
	errGroup.Go(func() error {
		return qual.Start(ctx)
	})

	addr := cmd.Flag("listen").Value.String()
	if addr == "" {
		addr = "127.0.0.1:8080"
		log.Info(fmt.Sprintf("no listen address specified, using fallback '%s'", addr))
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           httpRouter,
		ReadHeaderTimeout: time.Second * 3,
	}

	log.Info("configuring webhook server", "listen", cmd.Flag("listen").Value.String())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	errGroup.Go(func() error {
		<-sigs
		log.Info("shutting down")
		cancelFn()

		ctx, cancelTimeout := context.WithTimeout(ctx, time.Second)
		defer cancelTimeout()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Error(err, "error shutting down http server")
			return err
		}

		return nil
	})

	errGroup.Go(func() error {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	})

	if err := errGroup.Wait(); !errors.Is(err, context.Canceled) {
		return err
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
