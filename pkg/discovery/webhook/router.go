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

	registeredHandlersMu sync.Mutex
	registeredHandlers   map[string]InitHandlerFunc
	pathMu               sync.Mutex
	paths                map[string]http.Handler
	logger               logr.Logger
}

func NewWebhookRouter(eventOuts chan<- discovery.RepositoryEvent) *WebhookRouter {
	return &WebhookRouter{
		eventOuts:          eventOuts,
		paths:              make(map[string]http.Handler),
		logger:             logr.Discard(),
		registeredHandlers: make(map[string]InitHandlerFunc),
	}
}

func (r *WebhookRouter) WithLogger(logger logr.Logger) {
	r.logger = logger
}

type InitHandlerFunc func(registry *discovery.Registry, out chan<- discovery.RepositoryEvent) http.Handler

func (r *WebhookRouter) RegisterHandler(name string, fn InitHandlerFunc) {
	r.registeredHandlersMu.Lock()
	defer r.registeredHandlersMu.Unlock()

	if fn == nil {
		panic("cannot register nil handler")
	}

	if _, exists := r.registeredHandlers[name]; exists {
		panic(fmt.Sprintf("handler %q already registered", name))
	}

	r.registeredHandlers[name] = fn
}

func (r *WebhookRouter) UnregisterHandler(name string) {
	r.registeredHandlersMu.Lock()
	defer r.registeredHandlersMu.Unlock()
	delete(r.registeredHandlers, name)
}

func (r *WebhookRouter) RegisterPath(reg *discovery.Registry) error {
	r.pathMu.Lock()
	defer r.pathMu.Unlock()

	if _, alreadyExists := r.paths[reg.WebhookPath]; alreadyExists {
		return fmt.Errorf("webhook handler for path %s already exists", reg.WebhookPath)
	}

	initFn, known := r.registeredHandlers[reg.Flavor]
	if !known {
		return fmt.Errorf("unknown flavor '%s'", reg.Flavor)
	}

	r.paths[reg.WebhookPath] = initFn(reg, r.eventOuts)

	r.logger.Info(fmt.Sprintf("registered webhook handler %s (path %s)", reg.Flavor, reg.WebhookPath))

	return nil
}

func (r *WebhookRouter) UnregisterPath(reg *discovery.Registry) {
	r.pathMu.Lock()
	defer r.pathMu.Unlock()
	delete(r.paths, reg.WebhookPath)
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

	if handler, ok := r.paths[path]; ok {
		r.logger.Info(fmt.Sprintf("found webhook handler for path %s", path))
		req = req.WithContext(logr.NewContext(req.Context(), r.logger))
		handler.ServeHTTP(w, req)

		return
	}

	r.logger.Info(fmt.Sprintf("webhook handler for path %s not found", path))
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
