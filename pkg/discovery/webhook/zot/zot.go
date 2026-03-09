// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package zot

import (
	"errors"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/json"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/discovery/webhook"
)

type WebhookHandler struct {
	registry *discovery.Registry
	channel  chan<- discovery.RepositoryEvent
}

const (
	name = "zot"

	ZotEventTypeImageUpdated      = "zotregistry.image.updated"
	ZotEventTypeImageDeleted      = "zotregistry.image.deleted"
	ZotEventTypeRepositoryCreated = "zotregistry.repository.created"
)

func init() {
	webhook.RegisterHandler(name, NewHandler)
}

func NewHandler(registry *discovery.Registry, out chan<- discovery.RepositoryEvent) http.Handler {
	wh := &WebhookHandler{
		registry: registry,
		channel:  out,
	}

	return wh
}

type ZotEventData struct {
	Name      string `json:"name"`
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
	Manifest  string `json:"manifest"`
	MediaType string `json:"mediaType"`
}

func (wh *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := logr.FromContextOrDiscard(r.Context())
	logger.Info("handling request", "path", r.URL.Path)

	cloudEvent, err := cloudevents.NewEventFromHTTPRequest(r)
	if err != nil {
		logger.Error(err, "failed to parse CloudEvent from request")
		http.Error(w, "invalid cloud event", http.StatusBadRequest)

		return
	}

	var data ZotEventData
	if err := json.Unmarshal(cloudEvent.Data(), &data); err != nil {
		logger.Error(err, "failed to parse Zot data from CloudEvent request")
		http.Error(w, "invalid data payload", http.StatusBadRequest)

		return
	}

	if data.Name == "" || data.Reference == "" {
		logger.Error(errors.New("missing fields"), "fields in Zot data from CloudEvent missing", "data", data)
		http.Error(w, "invalid data payload", http.StatusBadRequest)

		return
	}

	repoEvent := discovery.RepositoryEvent{
		Registry:   wh.registry.Name,
		Repository: data.Name,
		Version:    data.Reference,
		Timestamp:  cloudEvent.Time(),
	}

	switch cloudEvent.Type() {
	case ZotEventTypeRepositoryCreated:
		repoEvent.Type = discovery.EventCreated
	case ZotEventTypeImageUpdated:
		repoEvent.Type = discovery.EventUpdated
	case ZotEventTypeImageDeleted:
		repoEvent.Type = discovery.EventDeleted
	default:
		logger.Info("unknown event type, ignoring", "type", cloudEvent.Type())
		w.WriteHeader(http.StatusNoContent)

		return
	}

	select {
	case wh.channel <- repoEvent:
		w.WriteHeader(http.StatusAccepted)
	case <-r.Context().Done():
		logger.Error(r.Context().Err(), "request context cancelled")
		http.Error(w, "timeout", http.StatusServiceUnavailable)
	default:
		logger.Error(nil, "event channel full, dropping event")
		http.Error(w, "server busy", http.StatusServiceUnavailable)
	}
}
