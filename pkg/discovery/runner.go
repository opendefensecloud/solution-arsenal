// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
)

// Processor defines the interface for processing events.
type Processor[InputEvent any, OutputEvent any] interface {
	Process(context.Context, InputEvent) ([]OutputEvent, error)
}

// Option describes the available options
// for creating the Handler.
type RunnerOption[InputEvent any, OutputEvent any] func(r *Runner[InputEvent, OutputEvent])

// WithLogger sets the logger for the Runner.
func WithLogger[InputEvent any, OutputEvent any](l logr.Logger) RunnerOption[InputEvent, OutputEvent] {
	return func(r *Runner[InputEvent, OutputEvent]) {
		r.logger = l
	}
}

// WithRateLimiter sets the rate limiter for the Runner that allows events up to the given interval and burst.
func WithRateLimiter[InputEvent any, OutputEvent any](interval time.Duration, burst int) RunnerOption[InputEvent, OutputEvent] {
	return func(r *Runner[InputEvent, OutputEvent]) {
		r.rateLimiter = rate.NewLimiter(rate.Every(interval), burst)
	}
}

// WithExponentialBackoff sets an exponential backoff strategy for the Qualifier.
func WithBackoff[InputEvent any, OutputEvent any](initialInterval time.Duration, maxInterval time.Duration, maxElapsedTime time.Duration) RunnerOption[InputEvent, OutputEvent] {
	return func(r *Runner[InputEvent, OutputEvent]) {
		b := backoff.NewExponentialBackOff()
		b.InitialInterval = initialInterval
		b.MaxInterval = maxInterval
		b.MaxElapsedTime = maxElapsedTime
		r.backoff = b
	}
}

// Runner is responsible for processing events from the input channel and publishing results to the output channel.
// It supports rate limiting and backoff strategies for handling processing errors.
// The Runner can be started and stopped gracefully, ensuring that all in-flight events are processed before shutdown.
// Output events are only published if the processor returns a non-nil output and the output channel is not nil.
type Runner[InputEvent any, OutputEvent any] struct {
	processor   Processor[InputEvent, OutputEvent]
	inputChan   <-chan InputEvent
	outputChan  chan<- OutputEvent
	errChan     chan<- ErrorEvent
	logger      logr.Logger
	stopChan    chan struct{}
	wg          sync.WaitGroup
	stopped     bool
	stopMu      sync.Mutex
	rateLimiter *rate.Limiter
	backoff     backoff.BackOff
}

func NewRunner[InputEvent any, OutputEvent any](
	processor Processor[InputEvent, OutputEvent],
	inputChan <-chan InputEvent,
	outputChan chan<- OutputEvent,
	errChan chan<- ErrorEvent,
) *Runner[InputEvent, OutputEvent] {
	r := &Runner[InputEvent, OutputEvent]{
		processor:  processor,
		inputChan:  inputChan,
		outputChan: outputChan,
		errChan:    errChan,
		logger:     logr.Discard(),
		stopChan:   make(chan struct{}),
	}

	return r
}

func (r *Runner[InputEvent, OutputEvent]) Start(ctx context.Context) error {
	r.logger.Info("starting runner")
	r.wg.Add(1)
	go r.runLoop(ctx)

	return nil
}

func (r *Runner[InputEvent, OutputEvent]) Stop() {
	r.stopMu.Lock()
	defer r.stopMu.Unlock()

	if !r.stopped {
		r.logger.Info("stopping runner")
		r.stopped = true
		close(r.stopChan)
		r.wg.Wait()
		r.logger.Info("runner stopped")
	}
}

func (r *Runner[InputEvent, OutputEvent]) runLoop(ctx context.Context) {
	defer r.wg.Done()

	for {
		select {
		case <-r.stopChan:
			return
		case <-ctx.Done():
			return
		case ev := <-r.inputChan:
			r.processEvent(ctx, ev)
		}
	}
}

func (r *Runner[InputEvent, OutputEvent]) processEvent(ctx context.Context, ev InputEvent) {
	r.logger.Info("processing event", "event", ev)

	if r.rateLimiter != nil {
		if err := r.rateLimiter.Wait(ctx); err != nil {
			r.handleError(err, "rate limiter wait failed")
			return
		}
	}

	outputEvents, err := r.processor.Process(ctx, ev)
	if err != nil {
		r.handleError(err, "failed to process event", "event", ev)
		return
	}

	if outputEvents == nil {
		r.logger.Info("processor returned nil output, skipping publish", "event", ev)
		return
	}

	if r.outputChan == nil {
		r.logger.Info("output channel is nil, skipping publish", "event", ev)
		return
	}
	for _, outputEv := range outputEvents {
		r.logger.V(1).Info("publishing output event", "outputEvent", outputEv)
		Publish(&r.logger, r.outputChan, outputEv)
	}
}

func (r *Runner[InputEvent, OutputEvent]) handleError(err error, msg string, keysAndValues ...any) {
	Publish(&r.logger, r.errChan, ErrorEvent{
		Error:     err,
		Timestamp: time.Now().UTC(),
	})
	r.logger.Error(err, msg, keysAndValues...)
}

func (r *Runner[InputEvent, OutputEvent]) Logger() logr.Logger {
	return r.logger
}

func (r *Runner[InputEvent, OutputEvent]) Backoff() backoff.BackOff {
	return r.backoff
}
