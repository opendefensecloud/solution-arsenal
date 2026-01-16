package webhook_server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.opendefense.cloud/solar/internal/webhook/handlers"
	"go.opendefense.cloud/solar/internal/webhook/router"
	"go.opendefense.cloud/solar/internal/webhook/service"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
)

var cmd = &cobra.Command{
	Use:   "webhook-server",
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

func NewCommand() *cobra.Command {
	return cmd
}

type Config struct {
	Registries map[string]Registry `yaml:"registries"`
}

type Registry struct {
	Webhook *Webhook `yaml:"webhook"`
}

type Webhook struct {
	Path   string
	Flavor handlers.WebhookFlavor
}

func runE(cmd *cobra.Command, args []string) error {
	ctx, cancelFn := context.WithCancel(cmd.Context())
	defer cancelFn()

	webhookService := service.New()
	httpRouter := router.NewWebhookRouter()

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

	for name, repository := range webhookConfig.Registries {
		if repository.Webhook == nil {
			log.Printf("No webhook available for registry %s, skipping", name)
			continue
		}

		if err := httpRouter.RegisterHandler(repository.Webhook.Flavor, repository.Webhook.Path); err != nil {
			return fmt.Errorf("failed to register handler: %w", err)
		}
	}

	httpServer := &http.Server{
		Addr:    cmd.Flag("listen").Value.String(),
		Handler: httpRouter,
	}

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs

		log.Println("shutting down")

		ctx, cancelTimeout := context.WithTimeout(ctx, time.Second)
		defer cancelTimeout()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Println("error shutting down http server:", err)
			return
		}

		log.Println("bye.")
	}()

	errGroup, ctx := errgroup.WithContext(ctx)

	errGroup.Go(func() error {
		return webhookService.Start(ctx)
	})

	errGroup.Go(func() error {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	})

	return errGroup.Wait()
}
