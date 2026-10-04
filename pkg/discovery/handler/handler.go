// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/go-logr/logr"
	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/ocmv2"
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

	// Open the repository holding the component
	baseURL := fmt.Sprintf("%s/%s", registry.GetURL(), ev.Namespace)
	repo, err := ocmv2.OpenRepository(baseURL, discovery.OCMCredentials(rs.provider.GetCredentials(ev.Source.Registry)))
	if err != nil {
		rs.Logger().Error(err, "failed to open repository", "registry", ev.Source.Registry, "repository", ev.Source.Repository)
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Lookup the specific component version
	var desc *descruntime.Descriptor
	if opts := rs.RetryOptions(); opts == nil {
		desc, err = repo.GetComponentVersion(ctx, comp, version)
	} else {
		// If backoff is configured, use it to retry on transient errors
		operation := func() (*descruntime.Descriptor, error) {
			cv, err := repo.GetComponentVersion(ctx, comp, version)
			if err != nil {
				// Check if the error is a 429 or transient
				if isRetryable(err) {
					return nil, err // Returning error triggers a retry
				}

				return nil, backoff.Permanent(err) // Stops retrying for 401, 404, etc.
			}

			return cv, nil
		}
		desc, err = backoff.Retry(ctx, operation, opts...)
	}
	if err != nil {
		// A permanent failure (401/404) needs an operator to fix credentials or
		// the reference; an exhausted budget is transient and the next scan will
		// retry. Both used to log identically.
		fields, logErr := discovery.RetryFailure(err)
		rs.Logger().Error(logErr, "failed to lookup component", append(fields, "version", version)...)

		// Wrap the original error, not logErr: the cause sentinel stays in the
		// chain so callers can still match errors.Is(err, backoff.ErrPermanent).
		return nil, fmt.Errorf("failed to lookup component version %s: %w", version, err)
	}

	// Count the number of Helm chart resources in the component version and determine the handler type based on that.
	for _, res := range desc.Component.Resources {
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

	rs.Logger().V(1).Info("resolved component version", "component", comp, "version", version)

	// Process component with determined handler type.
	h, err := rs.getHandlerForType(handlerType)
	if err != nil {
		rs.Logger().Error(err, "failed to process component with handler", "handler", handlerType)
		return nil, fmt.Errorf("failed to process component with handler %q: %w", handlerType, err)
	}

	// Process component with determined handler. If processing fails, log and publish error.
	resEvent, err := h.Process(ctx, repo, &ev, desc)
	if err != nil {
		rs.Logger().Error(err, "failed to process component with handler", "handler", handlerType)
		return nil, fmt.Errorf("failed to process component with handler %q: %w", handlerType, err)
	}

	return []discovery.WriteAPIResourceEvent{*resEvent}, nil
}

// getHandlerForType returns the handler for the given type, initializing it if necessary.
func (rs *Handler) getHandlerForType(t HandlerType) (ComponentHandler, error) {
	if h, ok := rs.handler[t]; ok {

		return h, nil
	}

	if initFn, ok := handlerRegistry[t]; ok {
		h := initFn(rs.Logger().WithValues("handler", t))
		rs.handler[t] = h

		return h, nil
	}

	return nil, fmt.Errorf("no handler registered for type %v", t)
}
