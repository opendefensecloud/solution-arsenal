// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package zot

import (
	"fmt"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/json"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/discovery/webhook"
)

type WebhookHandler struct {
	registry discovery.Registry
	channel  chan<- discovery.RepositoryEvent
}

const (
	name = "zot"

	EventTypeImageUpdated      = "zotregistry.image.updated"
	EventTypeImageDeleted      = "zotregistry.image.deleted"
	EventTypeImageLintFailed   = "zotregistry.image.lint_failed"
	EventTypeRepositoryCreated = "zotregistry.repository.created"
)

func init() {
	webhook.RegisterHandler(name, NewHandler)
}

func NewHandler(registry discovery.Registry, out chan<- discovery.RepositoryEvent) http.Handler {
	wh := &WebhookHandler{
		registry: registry,
		channel:  out,
	}

	return wh
}

type ZotEventData struct {
	Name      string   `json:"name"`
	Reference string   `json:"reference"`
	Digest    string   `json:"digest"`
	Manifest  Manifest `json:"manifest"`
	MediaType string   `json:"mediaType"`
}

type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType,omitempty"`
	Config        Config `json:"config"`
}

type Config struct {
	MediaType string  `json:"mediaType,omitempty"`
	Size      int     `json:"size,omitempty"`
	Digest    string  `json:"digest,omitempty"`
	Layers    []Layer `json:"layers,omitempty"`
}

type Layer struct {
	MediaType string `json:"mediaType,omitempty"`
	Size      int    `json:"size,omitempty"`
	Digest    string `json:"digest,omitempty"`
}

func (wh *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := logr.FromContextOrDiscard(r.Context())
	logger.Info(fmt.Sprintf("this is zot handling request to %s", r.URL.Path))

	cloudEvent, err := cloudevents.NewEventFromHTTPRequest(r)
	if err != nil {
		msg := fmt.Sprintf("failed to parse CloudEvent from request: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		logger.Info(msg)

		return
	}

	var data ZotEventData
	if err := json.Unmarshal(cloudEvent.Data(), &data); err != nil {
		msg := fmt.Sprintf("failed to parse CloudEvent.Data from request: %v", err)
		http.Error(w, msg, http.StatusBadRequest)

		return
	}

	repoEvent := discovery.RepositoryEvent{
		Registry:   wh.registry.Name,
		Repository: data.Name,
		Version:    data.Reference,
		Timestamp:  cloudEvent.Time(),
	}

	switch cloudEvent.Type() {
	case EventTypeRepositoryCreated:
		repoEvent.Type = discovery.EventCreated
	case EventTypeImageUpdated:
		repoEvent.Type = discovery.EventUpdated
	case EventTypeImageDeleted:
		repoEvent.Type = discovery.EventDeleted
	default:
		logger.Info(fmt.Sprintf("unknown event type '%s'", cloudEvent.Type()))
		return
	}

	logger.Info(string(cloudEvent.Data()))

	wh.channel <- repoEvent

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(cloudEvent); err != nil {
		msg := fmt.Sprintf("failed to encode event: %v", err)
		logger.Info(msg)
	}
}
