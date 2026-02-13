// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-logr/logr"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/repositories/ocireg"

	"go.opendefense.cloud/solar/pkg/discovery"
)

var (
	// handlerRegistry is a map of handler types to their corresponding handlers.
	handlerRegistry = make(map[HandlerType]InitHandlerFunc)
)

type InitHandlerFunc func(log logr.Logger) ComponentHandler

func RegisterComponentHandler(t HandlerType, fn InitHandlerFunc) {
	if fn == nil {
		panic("cannot register nil handler")
	}

	if _, exists := handlerRegistry[t]; exists {
		panic(fmt.Sprintf("handler %q already registered", t))
	}

	handlerRegistry[t] = fn
}

type Handler struct {
	*discovery.Runner[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent]
	provider *discovery.RegistryProvider
	handler  map[HandlerType]ComponentHandler
}

func NewHandlerOptions(opts ...discovery.RunnerOption[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent]) []discovery.RunnerOption[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent] {
	return opts
}

func NewHandler(
	provider *discovery.RegistryProvider,
	in <-chan discovery.ComponentVersionEvent,
	out chan<- discovery.WriteAPIResourceEvent,
	err chan<- discovery.ErrorEvent,
	opts ...discovery.RunnerOption[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent],
) *Handler {
	p := &Handler{
		provider: provider,
		handler:  make(map[HandlerType]ComponentHandler),
	}
	p.Runner = discovery.NewRunner(p, in, out, err)
	for _, opt := range opts {
		opt(p.Runner)
	}

	return p
}

// isRetryable determines if we should wait and try again
func isRetryable(err error) bool {
	msg := strings.ToLower(err.Error())
	// OCM often wraps errors, so we check the string for common rate-limit indicators
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "connection refused")
}

func (rs *Handler) Process(ctx context.Context, ev discovery.ComponentVersionEvent) ([]discovery.WriteAPIResourceEvent, error) {
	rs.Logger().Info("processing component version event", "event", ev)
	comp := ev.Component
	version := ev.Source.Version

	// Analyze resources contained in component descriptor.
	helmChartCount := 0
	handlerType := HandlerType("")

	// Exit early on deletion
	if ev.Source.Type == discovery.EventDeleted {
		return []discovery.WriteAPIResourceEvent{{
			Source:    ev,
			Timestamp: time.Now().UTC(),
		}}, nil
	}

	// Get registry configuration
	registry := rs.provider.Get(ev.Source.Registry)
	if registry == nil {
		rs.Logger().V(2).Info("invalid registry", "registry", ev.Source.Registry)
		return nil, fmt.Errorf("invalid registry: %s", ev.Source.Registry)
	}

	// Create repository for the component
	baseURL := fmt.Sprintf("%s/%s", registry.GetURL(), ev.Namespace)
	octx := ocm.FromContext(ctx)
	repo, err := octx.RepositoryForSpec(ocireg.NewRepositorySpec(baseURL))
	if err != nil {
		rs.Logger().Error(err, "failed to create repo spec", "registry", ev.Source.Registry, "repository", ev.Source.Repository)
		return nil, fmt.Errorf("failed to create repository spec: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Lookup the specific component version
	var compVersion ocm.ComponentVersionAccess
	if rs.Backoff() == nil {
		compVersion, err = repo.LookupComponentVersion(comp, version)
	} else {
		// If backoff is configured, use it to retry on transient errors
		operation := func() error {
			var err error
			compVersion, err = repo.LookupComponentVersion(comp, version)
			if err != nil {
				// Check if the error is a 429 or transient
				if isRetryable(err) {
					return err // Returning error triggers a retry
				}

				return backoff.Permanent(err) // Stops retrying for 401, 404, etc.
			}

			return nil
		}
		err = backoff.Retry(operation, rs.Backoff())
	}
	if err != nil {
		rs.Logger().Error(err, "failed to lookup component", "version", version)
		return nil, fmt.Errorf("failed to lookup component version %s: %w", version, err)
	}
	defer func() { _ = compVersion.Close() }()

	// Count the number of Helm chart resources in the component version and determine the handler type based on that.
	for _, res := range compVersion.GetDescriptor().ComponentSpec.Resources {
		if res.Type == string(HelmResource) {
			helmChartCount++
		}
	}

	// Classify component based on contained resources as helm chart and send it to the corresponding handler.
	if helmChartCount == 1 {
		handlerType = HelmHandler
	}

	// If no handler type could be determined, log and publish error.
	if handlerType == "" {
		// No handler found for event, log and publish error.
		rs.Logger().Info("no handler found for event", "event", ev)
		return nil, fmt.Errorf("no handler found for component version event: %v", ev)
	}

	// Process component with determined handler type.
	h, err := rs.getHandler(handlerType)
	if err != nil {
		rs.Logger().Error(err, "failed to process component with handler", "handler", handlerType)
		return nil, fmt.Errorf("failed to process component with handler %q: %w", handlerType, err)
	}

	// Process component with determined handler. If processing fails, log and publish error.
	resEvent, err := h.Process(ctx, &ev, compVersion)
	if err != nil {
		rs.Logger().Error(err, "failed to process component with handler", "handler", handlerType)
		return nil, fmt.Errorf("failed to process component with handler %q: %w", handlerType, err)
	}

	return []discovery.WriteAPIResourceEvent{*resEvent}, nil
}

// getHandler returns the handler for the given type, initializing it if necessary.
func (rs *Handler) getHandler(t HandlerType) (ComponentHandler, error) {
	if rs.handler[HelmHandler] == nil {
		if initFn, ok := handlerRegistry[HelmHandler]; ok {
			handler := initFn(rs.Logger().WithValues("handler", HelmHandler))
			rs.handler[HelmHandler] = handler

			return handler, nil
		}
	}

	return nil, fmt.Errorf("no handler registered for type %v", t)
}
