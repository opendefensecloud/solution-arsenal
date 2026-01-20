package webhook

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"go.opendefense.cloud/solar/pkg/discovery"
)

var (
	registeredHandlersMu sync.Mutex
	registeredHandlers   = make(map[string]InitHandlerFunc)
)

type WebhookRouter struct {
	eventOuts []chan<- discovery.RepositoryEvent

	pathMu sync.Mutex
	paths  map[string]http.Handler
}

func NewWebhookRouter(eventOuts ...chan<- discovery.RepositoryEvent) *WebhookRouter {
	return &WebhookRouter{
		eventOuts: eventOuts,
		paths:     make(map[string]http.Handler),
	}
}

type InitHandlerFunc func(out ...chan<- discovery.RepositoryEvent) http.Handler

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

func (r *WebhookRouter) RegisterPath(handlerName, path string) error {
	r.pathMu.Lock()
	defer r.pathMu.Unlock()

	if _, alreadyExists := r.paths[path]; alreadyExists {
		return fmt.Errorf("webhook handler for path %s already exists", path)
	}

	initFn, known := registeredHandlers[handlerName]
	if !known {
		return fmt.Errorf("unknown handler '%s'", handlerName)
	}

	r.paths[path] = initFn(r.eventOuts...)

	log.Printf("registered webhook handler %s (path %s) for %d eventOuts", handlerName, path, len(r.eventOuts))

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
	if handler, ok := r.paths[path]; ok {
		handler.ServeHTTP(w, req)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}
