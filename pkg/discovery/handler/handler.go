// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"

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
	inputChan  <-chan discovery.ComponentVersionEvent
	outputChan chan<- discovery.ComponentVersionEvent
	errChan    chan<- discovery.ErrorEvent
	logger     logr.Logger
	stopChan   chan struct{}
	wg         sync.WaitGroup
	stopped    bool
	stopMu     sync.Mutex
	handler    map[HandlerType]ComponentHandler
}

// Option describes the available options
// for creating the Handler.
type Option func(r *Handler)

func WithLogger(l logr.Logger) Option {
	return func(r *Handler) {
		r.logger = l
	}
}

func NewHandler(
	inputChan <-chan discovery.ComponentVersionEvent,
	outputChan chan<- discovery.ComponentVersionEvent,
	errChan chan<- discovery.ErrorEvent,
	opts ...Option,
) *Handler {
	c := &Handler{
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

func (rs *Handler) processEvent(ctx context.Context, ev *discovery.ComponentVersionEvent) {
	rs.logger.Info("processing component version event", "event", ev)

	// Analyze resources contained in component descriptor.
	helmChartCount := 0
	handlerType := HandlerType("")
	for _, res := range ev.Descriptor.ComponentSpec.Resources {
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
	if err := h.ProcessEvent(ctx, ev); err != nil {
		rs.logger.Error(err, "failed to process component with handler", "handler", handlerType)
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Error:     fmt.Errorf("failed to process component with handler %q: %w", handlerType, err),
			Timestamp: time.Now().UTC(),
		})

		return
	}

	// Publish component event.
	discovery.Publish(&rs.logger, rs.outputChan, *ev)
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
