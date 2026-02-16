// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package generic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/discovery/webhook"
)

type WebhookHandler struct {
	registry *discovery.Registry
	channel  chan<- discovery.RepositoryEvent
}

const (
	name = "generic"
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

type Envelope struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Type      EventType       `json:"type"`
	Data      json.RawMessage `json:"data"`
}

type Data struct {
	Repository string  `json:"repository"`
	Version    *string `json:"version"`
}

type EventType string

const (
	EventTypeRepositoryCreated EventType = "repository.created"
	EventTypeRepositoryDeleted EventType = "repository.deleted"
	EventTypeImageUpdated      EventType = "image.updated"
	EventTypeImageDeleted      EventType = "image.deleted"
)

func (wh *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := logr.FromContextOrDiscard(r.Context())
	logger.Info(fmt.Sprintf("this is zot handling request to %s", r.URL.Path))

	var envelope Envelope
	if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
		msg := fmt.Sprintf("failed to parse CloudEvent.Data from request: %v", err)
		http.Error(w, msg, http.StatusBadRequest)

		return
	}

	var data Data
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		const msg = "failed to parse .data from request"
		logger.Info(msg, "error", err)
		http.Error(w, msg, http.StatusBadRequest)

		return
	}

	var version string
	if data.Version != nil {
		version = *data.Version
	}

	mappedEvent, err := mapEventType(envelope.Type)
	if err != nil {
		msg := fmt.Sprintf("failed to map event type: %v", err)
		http.Error(w, msg, http.StatusBadRequest)

		return
	}

	repoEvent := discovery.RepositoryEvent{
		Type:       mappedEvent,
		Registry:   wh.registry.Name,
		Repository: data.Repository,
		Version:    version,
		Timestamp:  envelope.Timestamp,
	}

	wh.channel <- repoEvent

	w.WriteHeader(http.StatusNoContent)
}

func mapEventType(event EventType) (discovery.EventType, error) {
	switch event {
	case EventTypeRepositoryCreated:
		return discovery.EventCreated, nil
	case EventTypeRepositoryDeleted:
		return discovery.EventDeleted, nil
	case EventTypeImageUpdated:
		return discovery.EventUpdated, nil
	case EventTypeImageDeleted:
		return discovery.EventDeleted, nil
	default:
		return "", fmt.Errorf("unknown event type: %s", event)
	}
}
