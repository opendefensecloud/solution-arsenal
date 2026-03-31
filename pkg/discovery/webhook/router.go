// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/go-logr/logr"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type WebhookRouter struct {
	eventOuts chan<- discovery.RepositoryEvent

	pathMu sync.RWMutex
	paths  map[string]http.Handler

	logger logr.Logger
}

func NewWebhookRouter(eventOuts chan<- discovery.RepositoryEvent) *WebhookRouter {
	return &WebhookRouter{
		eventOuts: eventOuts,
		paths:     make(map[string]http.Handler),
		logger:    logr.Discard(),
	}
}

func (r *WebhookRouter) WithLogger(logger logr.Logger) {
	r.logger = logger
}

// RegisterPath registers the given discovery.Registry with the WebhookRouter, using
// the registry's flavor (aka handler type) and WebhookPath. If the WebhookPath is
// already used by a registry or the given flavor is not known (see RegisterHandler),
// an error is returned.
func (r *WebhookRouter) RegisterPath(reg *discovery.Registry) error {
	registeredHandlersMu.RLock()
	defer registeredHandlersMu.RUnlock()

	initFn, known := registeredHandlers[reg.Flavor]
	if !known {
		return fmt.Errorf("unknown flavor '%s'", reg.Flavor)
	}

	r.pathMu.Lock()
	defer r.pathMu.Unlock()

	if _, alreadyExists := r.paths[reg.WebhookPath]; alreadyExists {
		return fmt.Errorf("webhook handler for path %s already exists", reg.WebhookPath)
	}

	r.paths[reg.WebhookPath] = initFn(reg, r.eventOuts)

	r.logger.Info(fmt.Sprintf("registered webhook handler %s (path %s)", reg.Flavor, reg.WebhookPath))

	return nil
}

func (r *WebhookRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.logger.Info(fmt.Sprintf("webhook handler %s %s", req.Method, req.URL.Path))

	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		r.logger.Info(fmt.Sprintf("invalid method %s", req.Method))

		return
	}

	path := req.URL.Path

	if !strings.HasPrefix(path, "/webhook") {
		w.WriteHeader(http.StatusNotFound)
		r.logger.Info(fmt.Sprintf("invalid path %s", path))

		return
	}

	path = strings.TrimPrefix(path, "/webhook/")

	r.pathMu.RLock()
	handler, ok := r.paths[path]
	r.pathMu.RUnlock()

	if ok {
		req = req.WithContext(logr.NewContext(req.Context(), r.logger))
		handler.ServeHTTP(w, req)

		return
	}

	r.logger.Info(fmt.Sprintf("webhook handler for path %s not found", path))
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
