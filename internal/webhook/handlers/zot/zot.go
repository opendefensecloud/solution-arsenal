package zot

import (
	"fmt"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-logr/logr"
	"go.opendefense.cloud/solar/internal/webhook"
	"go.opendefense.cloud/solar/pkg/discovery"
	"k8s.io/apimachinery/pkg/util/json"
)

type WebhookHandler struct {
	channel chan<- discovery.RepositoryEvent
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

func NewHandler(out chan<- discovery.RepositoryEvent) http.Handler {
	wh := &WebhookHandler{
		channel: out,
	}

	return wh
}

type ZotEvent interface {
	EventType() string
}

func (wh *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := logr.FromContextOrDiscard(r.Context())

	cloudEvent, err := cloudevents.NewEventFromHTTPRequest(r)
	if err != nil {
		msg := fmt.Sprintf("failed to parse CloudEvent from request: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		logger.Info(msg)

		return
	}

	repoEvent := discovery.RepositoryEvent{
		Timestamp: cloudEvent.Time(),
	}

	var data ZotEvent
	switch cloudEvent.Type() {
	case EventTypeRepositoryCreated:
		repoEvent.Type = discovery.EventCreated
	case EventTypeImageUpdated:
		repoEvent.Type = discovery.EventUpdated
	case EventTypeImageDeleted:
		repoEvent.Type = discovery.EventDeleted
	default:
		logger.Info("unknown event type: %v", cloudEvent.Type())
		return
	}

	if err := json.Unmarshal(cloudEvent.Data(), &data); err != nil {
		msg := fmt.Sprintf("failed to parse CloudEvent.Data from request: %v", err)
		http.Error(w, msg, http.StatusBadRequest)

		return
	}

	wh.channel <- repoEvent

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(cloudEvent); err != nil {
		msg := fmt.Sprintf("failed to encode event: %v", err)
		logger.Info(msg)
	}
}
