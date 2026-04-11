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

	"go.opendefense.cloud/solar/pkg/ui"
)

var cmd = &cobra.Command{
	Use:   "solar-ui",
	Short: "SolAr UI — web frontend and BFF for the SolAr Kubernetes extension API",
	RunE:  runE,
}

func init() {
	cmd.Flags().StringP("listen", "l", "0.0.0.0:8090", "Address to listen on")
	cmd.Flags().String("oidc-issuer", "", "OIDC issuer URL (e.g. https://dex.example.com)")
	cmd.Flags().String("oidc-client-id", "solar-ui", "OIDC client ID")
	cmd.Flags().String("oidc-client-secret", "", "OIDC client secret")
	cmd.Flags().String("oidc-redirect-url", "http://localhost:8090/api/auth/callback", "OIDC redirect URL")
	cmd.Flags().String("session-key", "", "Session encryption key (32 bytes, hex-encoded). Generated if empty.")
	cmd.Flags().String("kubeconfig", "", "Path to kubeconfig (defaults to in-cluster config)")
	cmd.Flags().String("auth-mode", "token", "How to convey OIDC identity to K8s: 'token' (forward id_token) or 'impersonate'")
	cmd.Flags().String("dev-vite-url", "", "Proxy non-API requests to Vite dev server (e.g. http://localhost:5173)")
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

	addr, _ := cmd.Flags().GetString("listen")
	oidcIssuer, _ := cmd.Flags().GetString("oidc-issuer")
	oidcClientID, _ := cmd.Flags().GetString("oidc-client-id")
	oidcClientSecret, _ := cmd.Flags().GetString("oidc-client-secret")
	oidcRedirectURL, _ := cmd.Flags().GetString("oidc-redirect-url")
	sessionKey, _ := cmd.Flags().GetString("session-key")
	kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
	authMode, _ := cmd.Flags().GetString("auth-mode")
	devViteURL, _ := cmd.Flags().GetString("dev-vite-url")

	cfg := ui.Config{
		ListenAddr:       addr,
		OIDCIssuer:       oidcIssuer,
		OIDCClientID:     oidcClientID,
		OIDCClientSecret: oidcClientSecret,
		OIDCRedirectURL:  oidcRedirectURL,
		SessionKey:       sessionKey,
		Kubeconfig:       kubeconfig,
		AuthMode:         authMode,
		DevViteURL:       devViteURL,
	}

	server, err := ui.NewServer(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	return server.Run(ctx)
}

func main() {
	if err := cmd.Execute(); err != nil {
		if _, err := fmt.Fprintln(os.Stderr, err); err != nil {
			panic(err)
		}

		os.Exit(1)
	}
}
