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

func UnregisterHandler(name string) error {
	registeredHandlersMu.Lock()
	defer registeredHandlersMu.Unlock()

	if _, exists := registeredHandlers[name]; !exists {
		return fmt.Errorf("handler %q not registered", name)
	}

	delete(registeredHandlers, name)

	return nil
}

func UnregisterAllHandlers() {
	registeredHandlersMu.Lock()
	defer registeredHandlersMu.Unlock()

	registeredHandlers = make(map[string]InitHandlerFunc)
}
