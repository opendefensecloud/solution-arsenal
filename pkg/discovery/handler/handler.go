// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"go.opendefense.cloud/solar/pkg/discovery"
)

var (
	// handlerRegistry is a map of handler types to their corresponding handlers.
	handlerRegistry = make(map[HandlerType]ComponentHandler)
)

type Handler struct {
	client.Client
	inputChan <-chan discovery.ComponentVersionEvent
	errChan   chan<- discovery.ErrorEvent
	logger    logr.Logger
	stopChan  chan struct{}
	wg        sync.WaitGroup
	stopped   bool
	stopMu    sync.Mutex
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
	k8sClient client.Client,
	inputChan <-chan discovery.ComponentVersionEvent,
	errChan chan<- discovery.ErrorEvent,
	opts ...Option,
) *Handler {
	c := &Handler{
		Client:    k8sClient,
		inputChan: inputChan,
		errChan:   errChan,
		logger:    logr.Discard(),
		stopChan:  make(chan struct{}),
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
			rs.processEvent(ctx, ev)
		}
	}
}

func (rs *Handler) processEvent(ctx context.Context, ev discovery.ComponentVersionEvent) {
	rs.logger.Info("processing component version event", "event", ev)

	// TODO: Implement the logic to determine which handler should process the event and forward it to it.
	if handler, ok := handlerRegistry[HelmHandler]; ok {
		handler.ProcessEvent(ctx, ev)
	}
}
