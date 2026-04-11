// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/go-logr/logr"

	"go.opendefense.cloud/solar/pkg/ui/api"
	"go.opendefense.cloud/solar/pkg/ui/auth"
	"go.opendefense.cloud/solar/pkg/ui/session"
)

//go:embed all:static
var staticFS embed.FS

// Server is the solar-ui HTTP server.
type Server struct {
	cfg    Config
	log    logr.Logger
	server *http.Server
}

// NewServer creates a new solar-ui server.
func NewServer(cfg Config, log logr.Logger) (*Server, error) {
	sessionStore, err := session.NewStore(cfg.SessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}

	var authProvider auth.Provider
	if cfg.OIDCIssuer != "" {
		authProvider, err = auth.NewOIDCProvider(auth.OIDCConfig{
			Issuer:       cfg.OIDCIssuer,
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  cfg.OIDCRedirectURL,
			AuthMode:     auth.AuthMode(cfg.AuthMode),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
		}
	} else {
		authProvider = auth.NewNoopProvider()
	}

	k8sHandler, err := api.NewHandler(cfg.Kubeconfig, sessionStore, authProvider, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create API handler: %w", err)
	}

	mux := http.NewServeMux()

	// Auth routes — always accessible (no auth required)
	mux.HandleFunc("POST /api/auth/login", authProvider.HandleLogin(sessionStore))
	mux.HandleFunc("GET /api/auth/login", authProvider.HandleLogin(sessionStore))
	mux.HandleFunc("GET /api/auth/callback", authProvider.HandleCallback(sessionStore))
	mux.HandleFunc("GET /api/auth/me", k8sHandler.HandleMe())
	mux.HandleFunc("DELETE /api/auth/session", func(w http.ResponseWriter, r *http.Request) {
		sessionStore.Clear(w)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		sessionStore.Clear(w)
		http.Redirect(w, r, "/", http.StatusFound)
	})

	// K8s resource routes — require authentication
	requireAuth := authMiddleware(sessionStore)
	mux.Handle("GET /api/namespaces/{namespace}/targets", requireAuth(k8sHandler.HandleList("targets")))
	mux.Handle("GET /api/namespaces/{namespace}/targets/{name}", requireAuth(k8sHandler.HandleGet("targets")))
	mux.Handle("GET /api/namespaces/{namespace}/releases", requireAuth(k8sHandler.HandleList("releases")))
	mux.Handle("GET /api/namespaces/{namespace}/releases/{name}", requireAuth(k8sHandler.HandleGet("releases")))
	mux.Handle("GET /api/namespaces/{namespace}/releasebindings", requireAuth(k8sHandler.HandleList("releasebindings")))
	mux.Handle("GET /api/namespaces/{namespace}/components", requireAuth(k8sHandler.HandleList("components")))
	mux.Handle("GET /api/namespaces/{namespace}/components/{name}", requireAuth(k8sHandler.HandleGet("components")))
	mux.Handle("GET /api/namespaces/{namespace}/componentversions", requireAuth(k8sHandler.HandleList("componentversions")))
	mux.Handle("GET /api/namespaces/{namespace}/registries", requireAuth(k8sHandler.HandleList("registries")))
	mux.Handle("GET /api/namespaces/{namespace}/profiles", requireAuth(k8sHandler.HandleList("profiles")))
	mux.Handle("GET /api/namespaces/{namespace}/rendertasks", requireAuth(k8sHandler.HandleList("rendertasks")))

	// SSE events
	mux.Handle("GET /api/namespaces/{namespace}/events", requireAuth(k8sHandler.HandleSSE()))

	// SPA — either proxy to Vite dev server or serve embedded static files
	if cfg.DevViteURL != "" {
		viteURL, err := url.Parse(cfg.DevViteURL)
		if err != nil {
			return nil, fmt.Errorf("invalid dev-vite-url: %w", err)
		}

		proxy := httputil.NewSingleHostReverseProxy(viteURL)
		log.Info("proxying non-API requests to Vite dev server", "url", cfg.DevViteURL)
		mux.Handle("/", proxy)
	} else {
		staticContent, err := fs.Sub(staticFS, "static")
		if err != nil {
			return nil, fmt.Errorf("failed to create sub filesystem: %w", err)
		}

		mux.Handle("/", spaFileServer(http.FS(staticContent)))
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &Server{cfg: cfg, log: log, server: srv}, nil
}

// Run starts the server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.log.Info("starting solar-ui", "addr", s.cfg.ListenAddr)
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.log.Info("shutting down solar-ui")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return s.server.Shutdown(shutdownCtx)
	}
}

// authMiddleware returns 401 for unauthenticated requests.
func authMiddleware(store *session.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sess := store.Get(r); sess == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// spaFileServer serves static files and falls back to index.html for unknown paths.
func spaFileServer(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly
		f, err := fsys.Open(r.URL.Path)
		if err != nil {
			// File not found — serve index.html for SPA routing
			r.URL.Path = "/"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(w, r)
	})
}
