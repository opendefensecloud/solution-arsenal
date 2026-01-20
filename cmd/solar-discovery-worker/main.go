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
	"go.opendefense.cloud/solar/internal/webhook"
	_ "go.opendefense.cloud/solar/internal/webhook/handlers/zot"
	"go.opendefense.cloud/solar/internal/webhook/service"
	"go.opendefense.cloud/solar/pkg/discovery"
	scanner "go.opendefense.cloud/solar/pkg/discovery/scanner"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
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

type Config struct {
	Registries map[string]Registry `yaml:"registries"`
}

type Registry struct {
	URL     string   `yaml:"url"`
	Webhook *Webhook `yaml:"webhook"`
}

type Webhook struct {
	Path   string
	Flavor string
}

func runE(cmd *cobra.Command, args []string) error {
	ctx, cancelFn := context.WithCancel(cmd.Context())
	defer cancelFn()

	var log logr.Logger

	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}
	log = zapr.NewLogger(zapLog)

	webhookService := service.New()
	httpRouter := webhook.NewWebhookRouter(webhookService.Channel())

	configFilePath := cmd.Flag("config").Value.String()
	if configFilePath == "" {
		return fmt.Errorf("--config is required")
	}

	configFile, err := os.Open(configFilePath)
	if err != nil {
		return err
	}

	var webhookConfig Config
	if err := yaml.NewDecoder(configFile).Decode(&webhookConfig); err != nil {
		return fmt.Errorf("failed to decode configuration: %w", err)
	}

	eventsChan := make(chan discovery.RepositoryEvent)
	errChan := make(chan discovery.ErrorEvent)

	errGroup, ctx := errgroup.WithContext(ctx)

	for name, repository := range webhookConfig.Registries {
		if repository.Webhook == nil {
			log.Info(fmt.Sprintf("No webhook available for registry %s, skipping", name))
			continue
		}

		if err := httpRouter.RegisterPath(repository.Webhook.Flavor, repository.Webhook.Path); err != nil {
			return fmt.Errorf("failed to register handler: %w", err)
		}

		regScanner := scanner.NewRegistryScanner(repository.URL, eventsChan, errChan, scanner.WithLogger(log))
		errGroup.Go(func() error {
			return regScanner.Start(ctx)
		})
	}

	httpServer := &http.Server{
		Addr:    cmd.Flag("listen").Value.String(),
		Handler: httpRouter,
	}

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs

		cancelFn()

		log.Info("shutting down")

		ctx, cancelTimeout := context.WithTimeout(ctx, time.Second)
		defer cancelTimeout()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Info("error shutting down http server:", err)
			return
		}

		log.Info("bye.")
	}()

	errGroup.Go(func() error {
		// return webhookService.Start(ctx)
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
