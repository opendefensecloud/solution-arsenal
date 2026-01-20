package zot

import (
	"fmt"
	"log"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.opendefense.cloud/solar/internal/webhook"
	"go.opendefense.cloud/solar/pkg/discovery"
	"k8s.io/apimachinery/pkg/util/json"
)

type WebhookHandler struct {
	channels []chan<- discovery.RepositoryEvent
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

func NewHandler(out ...chan<- discovery.RepositoryEvent) http.Handler {
	return WebhookHandler{
		channels: out,
	}
}

func (wh WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cloudEvent, err := cloudevents.NewEventFromHTTPRequest(r)
	if err != nil {
		msg := fmt.Sprintf("failed to parse CloudEvent from request: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		log.Println(msg)

		return
	}

	var data map[string]interface{}
	if err := json.Unmarshal(cloudEvent.Data(), &data); err != nil {
		msg := fmt.Sprintf("failed to parse CloudEvent.Data from request: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		log.Println(msg)

		return
	}

	repoEvent := discovery.RepositoryEvent{
		Repository: data["name"].(string),
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
		log.Printf("unknown event type: %v", cloudEvent.Type())
		return
	}

	for _, ch := range wh.channels {
		log.Printf("sending event to channel: %v", ch)
		ch <- repoEvent
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(cloudEvent); err != nil {
		msg := fmt.Sprintf("failed to encode event: %v", err)
		log.Println(msg)
	}
}
