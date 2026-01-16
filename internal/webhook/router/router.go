package router

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"go.opendefense.cloud/solar/internal/webhook/handlers"
	"go.opendefense.cloud/solar/internal/webhook/handlers/zot"
	"go.opendefense.cloud/solar/internal/webhook/service"
)

type WebhookRouter struct {
	eventOut []chan<- service.RepositoryEvent
	handlers map[string]handlers.WebhookHTTPHandler
}

func NewWebhookRouter() *WebhookRouter {
	return &WebhookRouter{
		handlers: make(map[string]handlers.WebhookHTTPHandler),
	}
}

func (r *WebhookRouter) RegisterHandler(flavor handlers.WebhookFlavor, path string) error {
	if _, alreadyExists := r.handlers[path]; alreadyExists {
		return fmt.Errorf("webhook handler for path %s already exists", path)
	}

	if flavor != handlers.WebhookFlavorZot {
		return fmt.Errorf("unsupported webhook flavor: %s", flavor)
	}

	r.handlers[path] = zot.NewWebhookHandler(r.eventOut...)

	log.Printf("registered webhook handler for flavor %s (path %s)", flavor, path)

	return nil
}

func (r *WebhookRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Printf("%s %s", req.Method, req.URL)

	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	path := req.URL.Path

	if !strings.HasPrefix(path, "/webhook") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	path = strings.TrimPrefix(path, "/webhook/")

	log.Printf("%s", path)
	if handler, ok := r.handlers[path]; ok {
		handler.ServeHTTP(w, req)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}
