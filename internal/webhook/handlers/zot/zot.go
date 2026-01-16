package zot

import (
	"fmt"
	"log"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.opendefense.cloud/solar/internal/webhook/service"
)

type WebhookHandler struct {
	channels []chan<- service.RepositoryEvent
}

func NewWebhookHandler(eventOuts ...chan<- service.RepositoryEvent) *WebhookHandler {
	return &WebhookHandler{
		channels: eventOuts,
	}
}

func (wh WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	event, err := cloudevents.NewEventFromHTTPRequest(r)
	if err != nil {
		msg := fmt.Sprintf("failed to parse CloudEvent from request: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		log.Println(msg)

		return
	}

	for _, ch := range wh.channels {
		ch <- service.RepositoryEvent{Payload: event.Data()}
	}

	w.WriteHeader(http.StatusOK)
}
