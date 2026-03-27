// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package zot

import (
	"errors"
	"net/http"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/json"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type WebhookHandler struct {
	registry *discovery.Registry
	channel  chan<- discovery.RepositoryEvent
}

const (
	ZotEventTypeImageUpdated      = "zotregistry.image.updated"
	ZotEventTypeImageDeleted      = "zotregistry.image.deleted"
	ZotEventTypeRepositoryCreated = "zotregistry.repository.created"
)

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

// isDigestReference returns true if the given reference string is a digest
// (e.g. "sha256:abc123...") rather than a version tag.
// Version tags match [a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}
// and cannot contain colons, while digests always follow the "algorithm:hex" format.
// Therefore, checking for ":" reliably distinguishes digests from tags.
func isDigestReference(ref string) bool {
	return strings.Contains(ref, ":")
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
		Digest:     data.Digest,
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

	// For create/update events, skip when the reference is a digest rather than a version tag.
	// The OCI client typically issues two PUT requests: first by digest, then by tag.
	// Zot fires an image.updated event for each. We only process the tag-based one.
	//
	// For delete events, the reference is typically always a digest. We pass these through
	// since the downstream pipeline resolves the ComponentVersion via a digest label.
	if repoEvent.Type != discovery.EventDeleted && isDigestReference(data.Reference) {
		logger.V(1).Info("skipping event with digest reference", "reference", data.Reference, "type", repoEvent.Type, "repository", data.Name)
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
