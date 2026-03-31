// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"fmt"
	"net/http"
	"sync"

	"go.opendefense.cloud/solar/pkg/discovery"
)

var (
	registeredHandlersMu sync.Mutex
	registeredHandlers   = make(map[string]InitHandlerFunc)
)

type InitHandlerFunc func(registry *discovery.Registry, out chan<- discovery.RepositoryEvent) http.Handler

// RegisterHandler registers a webhook initialization handler under the provided name.
// It panics if fn is nil or if a handler with the same name is already registered.
// The function is safe for concurrent use.
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

// UnregisterHandler removes the handler registered under name from the global registry.
// It returns an error if no handler with that name is registered.
func UnregisterHandler(name string) error {
	registeredHandlersMu.Lock()
	defer registeredHandlersMu.Unlock()

	if _, exists := registeredHandlers[name]; !exists {
		return fmt.Errorf("handler %q not registered", name)
	}

	delete(registeredHandlers, name)

	return nil
}

// UnregisterAllHandlers removes all registered webhook initialization handlers from the global registry.
func UnregisterAllHandlers() {
	registeredHandlersMu.Lock()
	defer registeredHandlersMu.Unlock()

	registeredHandlers = make(map[string]InitHandlerFunc)
}
