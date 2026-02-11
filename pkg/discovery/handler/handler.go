// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
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
	provider    *discovery.RegistryProvider
	inputChan   <-chan discovery.ComponentVersionEvent
	outputChan  chan<- discovery.WriteAPIResourceEvent
	errChan     chan<- discovery.ErrorEvent
	logger      logr.Logger
	stopChan    chan struct{}
	wg          sync.WaitGroup
	stopped     bool
	stopMu      sync.Mutex
	handler     map[HandlerType]ComponentHandler
	rateLimiter *rate.Limiter
	backoff     backoff.BackOff
}

// Option describes the available options
// for creating the Handler.
type Option func(r *Handler)

func WithLogger(l logr.Logger) Option {
	return func(r *Handler) {
		r.logger = l
	}
}

// WithRateLimiter sets the rate limiter for the Qualifier that allows events up to the given interval and burst.
func WithRateLimiter(interval time.Duration, burst int) Option {
	return func(r *Handler) {
		r.rateLimiter = rate.NewLimiter(rate.Every(interval), burst)
	}
}

// WithExponentialBackoff sets an exponential backoff strategy for the Qualifier.
func WithExponentialBackoff(initialInterval time.Duration, maxInterval time.Duration, maxElapsedTime time.Duration) Option {
	return func(r *Handler) {
		b := backoff.NewExponentialBackOff()
		b.InitialInterval = initialInterval
		b.MaxInterval = maxInterval
		b.MaxElapsedTime = maxElapsedTime
		r.backoff = b
	}
}

func NewHandler(
	provider *discovery.RegistryProvider,
	inputChan <-chan discovery.ComponentVersionEvent,
	outputChan chan<- discovery.WriteAPIResourceEvent,
	errChan chan<- discovery.ErrorEvent,
	opts ...Option,
) *Handler {
	c := &Handler{
		provider:   provider,
		inputChan:  inputChan,
		outputChan: outputChan,
		errChan:    errChan,
		logger:     logr.Discard(),
		stopChan:   make(chan struct{}),
		handler:    make(map[HandlerType]ComponentHandler),
	}
	for _, o := range opts {
		o(c)
	}

	return c
}

func (rs *Handler) Start(ctx context.Context) error {
	rs.logger.Info("starting handler")

	rs.wg.Add(1)
	go rs.handlerLoop(ctx)

	return nil
}

// Stop gracefully stops the qualifier.
func (rs *Handler) Stop() {
	rs.stopMu.Lock()
	defer rs.stopMu.Unlock()

	if rs.stopped {
		return
	}

	rs.logger.Info("stopping handler")
	rs.stopped = true
	close(rs.stopChan)
	rs.wg.Wait()
	rs.logger.Info("handler stopped")
}

func (rs *Handler) handlerLoop(ctx context.Context) {
	defer rs.wg.Done()

	for {
		select {
		case <-rs.stopChan:
			return
		case <-ctx.Done():
			return
		case ev := <-rs.inputChan:
			rs.processEvent(ctx, &ev)
		}
	}
}

// isRetryable determines if we should wait and try again
func isRetryable(err error) bool {
	msg := strings.ToLower(err.Error())
	// OCM often wraps errors, so we check the string for common rate-limit indicators
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "connection refused")
}

func (rs *Handler) processEvent(ctx context.Context, ev *discovery.ComponentVersionEvent) {
	rs.logger.Info("processing component version event", "event", ev)
	comp := ev.Component
	version := ev.Source.Version

	// Analyze resources contained in component descriptor.
	helmChartCount := 0
	handlerType := HandlerType("")

	// If rate limiter is configured, wait before making the request
	if rs.rateLimiter != nil {
		if err := rs.rateLimiter.Wait(ctx); err != nil {
			rs.logger.Error(err, "rate limiter wait failed")
			return
		}
	}

	// Get registry configuration
	registry := rs.provider.Get(ev.Source.Registry)
	if registry == nil {
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Error:     fmt.Errorf("invalid registry: %s", ev.Source.Registry),
			Timestamp: time.Now().UTC(),
		})
		rs.logger.V(2).Info("invalid registry", "registry", ev.Source.Registry)

		return
	}

	// Create repository for the component
	baseURL := fmt.Sprintf("%s/%s", registry.GetURL(), ev.Namespace)
	octx := ocm.FromContext(ctx)
	repo, err := octx.RepositoryForSpec(ocireg.NewRepositorySpec(baseURL))
	if err != nil {
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Timestamp: time.Now().UTC(),
			Error:     fmt.Errorf("failed to create repo spec: %w", err),
		})
		rs.logger.Error(err, "failed to create repo spec", "registry", ev.Source.Registry, "namespace", ev.Namespace)

		return
	}
	defer func() { _ = repo.Close() }()

	// Lookup the specific component version
	var compVersion ocm.ComponentVersionAccess
	if rs.backoff == nil {
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
		err = backoff.Retry(operation, rs.backoff)
	}

	if err != nil {
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Timestamp: time.Now().UTC(),
			Error:     fmt.Errorf("failed to lookup component: %w", err),
		})
		rs.logger.Error(err, "failed to lookup component", "version", version)

		return
	}
	defer func() { _ = compVersion.Close() }()

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
		rs.logger.Info("no handler found for event", "event", ev)
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Error:     fmt.Errorf("no handler found for event: %v", ev),
			Timestamp: time.Now().UTC(),
		})

		return
	}

	// Process component with determined handler type.
	h, err := rs.getHandler(handlerType)
	if err != nil {
		rs.logger.Error(err, "failed to process component with handler", "handler", handlerType)
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Error:     fmt.Errorf("failed to process component with handler %q: %w", handlerType, err),
			Timestamp: time.Now().UTC(),
		})

		return
	}

	// Process component with determined handler. If processing fails, log and publish error.
	resEvent, err := h.Process(ctx, ev, compVersion)
	if err != nil {
		rs.logger.Error(err, "failed to process component with handler", "handler", handlerType)
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Error:     fmt.Errorf("failed to process component with handler %q: %w", handlerType, err),
			Timestamp: time.Now().UTC(),
		})

		return
	}

	// Publish processed component as API resource event.
	discovery.Publish(&rs.logger, rs.outputChan, *resEvent)
}

// getHandler returns the handler for the given type, initializing it if necessary.
func (rs *Handler) getHandler(t HandlerType) (ComponentHandler, error) {
	if rs.handler[HelmHandler] == nil {
		if initFn, ok := handlerRegistry[HelmHandler]; ok {
			handler := initFn(rs.logger.WithValues("handler", HelmHandler))
			rs.handler[HelmHandler] = handler

			return handler, nil
		}
	}

	return nil, fmt.Errorf("no handler registered for type %v", t)
}
