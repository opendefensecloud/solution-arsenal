// Copyright 2025 BWI GmbH and Artifact Conduit contributors
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

var (
	registeredHandlersMu sync.Mutex
	registeredHandlers   = make(map[string]InitHandlerFunc)
)

type WebhookRouter struct {
	eventOuts chan<- discovery.RepositoryEvent

	pathMu sync.Mutex
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

type InitHandlerFunc func(registry discovery.Registry, out chan<- discovery.RepositoryEvent) http.Handler

func RegisterHandler(name string, fn InitHandlerFunc) {
	registeredHandlersMu.Lock()
	defer registeredHandlersMu.Unlock()

	if fn == nil {
		panic("cannot register nil handler")
	}

	if _, exists := registeredHandlers[name]; exists {
		panic(fmt.Sprintf("handler %q already registered", name))
	}

	registeredHandlers[name] = fn
}

func (r *WebhookRouter) RegisterPath(reg discovery.Registry) error {
	r.pathMu.Lock()
	defer r.pathMu.Unlock()

	if _, alreadyExists := r.paths[reg.Webhook.Path]; alreadyExists {
		return fmt.Errorf("webhook handler for path %s already exists", reg.Webhook.Path)
	}

	initFn, known := registeredHandlers[reg.Flavor]
	if !known {
		return fmt.Errorf("unknown flavor '%s'", reg.Flavor)
	}

	r.paths[reg.Webhook.Path] = initFn(reg, r.eventOuts)

	r.logger.Info(fmt.Sprintf("registered webhook handler %s (path %s)", reg.Flavor, reg.Webhook.Path))

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

	if handler, ok := r.paths[path]; ok {
		r.logger.Info(fmt.Sprintf("found webhook handler for path %s", path))
		req = req.WithContext(logr.NewContext(req.Context(), r.logger))
		handler.ServeHTTP(w, req)

		return
	}

	r.logger.Info(fmt.Sprintf("webhook handler for path %s not found", path))
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
