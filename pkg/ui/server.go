// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
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
		sessionStore.Clear(w, r)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		sessionStore.Clear(w, r)
		http.Redirect(w, r, "/", http.StatusFound)
	})

	// K8s resource routes — require authentication
	requireAuth := authMiddleware(sessionStore)

	// Admin-only "preview as" impersonation. There is no allowlist: the BFF
	// forwards whatever the admin enters and lets K8s RBAC decide whether
	// the resulting requests succeed. The "is admin" gate is delegated to
	// K8s as well — only users allowed to impersonate users at the cluster
	// level can call these routes.
	requireAdmin := func(next http.HandlerFunc) http.Handler {
		return requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, err := k8sHandler.CanImpersonate(r.Context(), r)
			if err != nil {
				log.Error(err, "impersonate access review failed")
				http.Error(w, "internal error", http.StatusInternalServerError)

				return
			}
			if !allowed {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next(w, r)
		}))
	}

	mux.Handle("PUT /api/auth/impersonate", requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string   `json:"username"`
			Groups   []string `json:"groups,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
			http.Error(w, "invalid request body: username is required", http.StatusBadRequest)
			return
		}
		sessionStore.SetImpersonation(r, req.Username, req.Groups)
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.Handle("DELETE /api/auth/impersonate", requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		sessionStore.ClearImpersonation(r)
		w.WriteHeader(http.StatusNoContent)
	}))

	// List namespaces visible to the user (RBAC-filtered).
	mux.Handle("GET /api/namespaces", requireAuth(k8sHandler.HandleListNamespaces()))

	// Cluster-wide list routes ("all namespaces"). Same handler — empty
	// {namespace} path value makes the dynamic client list across all.
	mux.Handle("GET /api/targets", requireAuth(k8sHandler.HandleList("targets")))
	mux.Handle("GET /api/releases", requireAuth(k8sHandler.HandleList("releases")))
	mux.Handle("GET /api/releasebindings", requireAuth(k8sHandler.HandleList("releasebindings")))
	mux.Handle("GET /api/components", requireAuth(k8sHandler.HandleList("components")))
	mux.Handle("GET /api/componentversions", requireAuth(k8sHandler.HandleList("componentversions")))
	mux.Handle("GET /api/registries", requireAuth(k8sHandler.HandleList("registries")))
	mux.Handle("GET /api/profiles", requireAuth(k8sHandler.HandleList("profiles")))
	mux.Handle("GET /api/rendertasks", requireAuth(k8sHandler.HandleList("rendertasks")))

	// Namespace-scoped list and get routes.
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

	// SSE events: cluster-wide and namespace-scoped variants share the
	// same handler. The cluster-wide route opens watches across all
	// namespaces, with K8s RBAC silently dropping any the user can't see.
	mux.Handle("GET /api/events", requireAuth(k8sHandler.HandleSSE()))
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

		return s.server.Shutdown(shutdownCtx) //nolint:contextcheck // ctx is cancelled; need a fresh context with deadline for graceful shutdown
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
		// Clean the request path before probing the filesystem so that
		// "../" segments can't escape the static root (path traversal).
		// This mirrors what http.FileServer does internally.
		upath := r.URL.Path
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
		}
		cleaned := path.Clean(upath)
		name := strings.TrimPrefix(cleaned, "/")

		// Reject traversal-like paths; serve index.html for SPA routing.
		if name == ".." || strings.HasPrefix(name, "../") {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Try to serve the file directly
		f, err := fsys.Open(name)
		if err != nil {
			// File not found — serve index.html for SPA routing
			r.URL.Path = "/"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(w, r)
	})
}
