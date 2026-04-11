// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"go.opendefense.cloud/solar/pkg/ui/auth"
	"go.opendefense.cloud/solar/pkg/ui/session"
)

// resourceMap maps resource names to their GVR.
var resourceMap = map[string]schema.GroupVersionResource{
	"targets":           {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "targets"},
	"releases":          {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "releases"},
	"releasebindings":   {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "releasebindings"},
	"components":        {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "components"},
	"componentversions": {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "componentversions"},
	"registries":        {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "registries"},
	"registrybindings":  {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "registrybindings"},
	"profiles":          {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "profiles"},
	"rendertasks":       {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "rendertasks"},
}

// Handler serves the K8s API proxy routes.
type Handler struct {
	baseConfig   *rest.Config
	sessionStore *session.Store
	authProvider auth.Provider
	log          logr.Logger
}

// NewHandler creates a new API handler.
func NewHandler(kubeconfig string, store *session.Store, provider auth.Provider, log logr.Logger) (*Handler, error) {
	var cfg *rest.Config
	var err error

	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	return &Handler{
		baseConfig:   cfg,
		sessionStore: store,
		authProvider: provider,
		log:          log.WithName("api"),
	}, nil
}

// clientFor returns a dynamic client for the given session.
func (h *Handler) clientFor(r *http.Request) (dynamic.Interface, error) {
	sess := h.sessionStore.Get(r)
	cfg := h.authProvider.WrapConfig(h.baseConfig, sess)

	return dynamic.NewForConfig(cfg)
}

// HandleMe returns the current user info.
func (h *Handler) HandleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.sessionStore.GetJSON(w, r)
	}
}

// HandleList returns a handler that lists resources of the given type.
func (h *Handler) HandleList(resource string) http.HandlerFunc {
	gvr, ok := resourceMap[resource]
	if !ok {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, fmt.Sprintf("unknown resource: %s", resource), http.StatusNotFound)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")

		client, err := h.clientFor(r)
		if err != nil {
			h.log.Error(err, "failed to create client")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		list, err := client.Resource(gvr).Namespace(namespace).List(r.Context(), listOptions())
		if err != nil {
			h.log.Error(err, "failed to list resources", "resource", resource, "namespace", namespace)
			writeK8sError(w, err)

			return
		}

		writeJSON(w, list)
	}
}

// HandleGet returns a handler that gets a single resource.
func (h *Handler) HandleGet(resource string) http.HandlerFunc {
	gvr, ok := resourceMap[resource]
	if !ok {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, fmt.Sprintf("unknown resource: %s", resource), http.StatusNotFound)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		name := r.PathValue("name")

		client, err := h.clientFor(r)
		if err != nil {
			h.log.Error(err, "failed to create client")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		obj, err := client.Resource(gvr).Namespace(namespace).Get(r.Context(), name, getOptions())
		if err != nil {
			h.log.Error(err, "failed to get resource", "resource", resource, "namespace", namespace, "name", name)
			writeK8sError(w, err)

			return
		}

		writeJSON(w, obj)
	}
}

// HandleSSE returns a handler that streams resource watch events as SSE.
func (h *Handler) HandleSSE() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")

		client, err := h.clientFor(r)
		if err != nil {
			h.log.Error(err, "failed to create client")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher.Flush()

		// Watch all solar resources and multiplex into SSE via a channel
		type sseEvent struct {
			Type      string `json:"type"`
			Resource  string `json:"resource"`
			Namespace string `json:"namespace"`
		}
		events := make(chan sseEvent, 64)

		for resourceName, gvr := range resourceMap {
			go func() {
				watcher, err := client.Resource(gvr).Namespace(namespace).Watch(r.Context(), watchOptions())
				if err != nil {
					h.log.Error(err, "failed to watch", "resource", resourceName)

					return
				}
				defer watcher.Stop()

				for event := range watcher.ResultChan() {
					select {
					case events <- sseEvent{
						Type:      string(event.Type),
						Resource:  resourceName,
						Namespace: namespace,
					}:
					case <-r.Context().Done():
						return
					}
				}
			}()
		}

		// Single writer goroutine — serializes all writes and respects client disconnect
		for {
			select {
			case evt := <-events:
				b, _ := json.Marshal(evt)
				fmt.Fprintf(w, "data: %s\n\n", b)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
