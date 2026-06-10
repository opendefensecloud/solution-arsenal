// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
)

// Processor defines the interface for processing events.
type Processor[InputEvent any, OutputEvent any] interface {
	Process(context.Context, InputEvent) ([]OutputEvent, error)
}

// RunnerOption describes the available options
// for creating the Handler.
type RunnerOption[InputEvent any, OutputEvent any] func(r *Runner[InputEvent, OutputEvent])

// WithLogger sets the logger for the Runner.
func WithLogger[InputEvent any, OutputEvent any](l logr.Logger) RunnerOption[InputEvent, OutputEvent] {
	return func(r *Runner[InputEvent, OutputEvent]) {
		pt := reflect.TypeOf(r.Processor).String()
		r.logger = l.WithValues("processor", pt)
	}
}

// WithRateLimiter sets the rate limiter for the Runner that allows events up to the given interval and burst.
func WithRateLimiter[InputEvent any, OutputEvent any](interval time.Duration, burst int) RunnerOption[InputEvent, OutputEvent] {
	return func(r *Runner[InputEvent, OutputEvent]) {
		r.rateLimiter = rate.NewLimiter(rate.Every(interval), burst)
	}
}

// backoffConfig groups the exponential-backoff tuning values stored on a
// Runner. A nil *backoffConfig means no backoff is configured.
type backoffConfig struct {
	initialInterval time.Duration
	maxInterval     time.Duration
	maxElapsedTime  time.Duration
}

// WithBackoff sets an exponential backoff strategy for the Qualifier.
// Non-positive interval inputs are replaced with the backoff library's defaults
// and initialInterval is clamped to maxInterval, so misconfiguration cannot
// turn RetryOptions() into a hot retry loop.
func WithBackoff[InputEvent any, OutputEvent any](initialInterval time.Duration, maxInterval time.Duration, maxElapsedTime time.Duration) RunnerOption[InputEvent, OutputEvent] {
	if initialInterval <= 0 {
		initialInterval = backoff.DefaultInitialInterval
	}
	if maxInterval <= 0 {
		maxInterval = backoff.DefaultMaxInterval
	}
	if maxElapsedTime <= 0 {
		maxElapsedTime = backoff.DefaultMaxElapsedTime
	}
	if initialInterval > maxInterval {
		initialInterval = maxInterval
	}

	return func(r *Runner[InputEvent, OutputEvent]) {
		r.backoff = &backoffConfig{
			initialInterval: initialInterval,
			maxInterval:     maxInterval,
			maxElapsedTime:  maxElapsedTime,
		}
	}
}

// Runner is responsible for processing events from the input channel and publishing results to the output channel.
// It supports rate limiting and backoff strategies for handling processing errors.
// The Runner can be started and stopped gracefully, ensuring that all in-flight events are processed before shutdown.
// Output events are only published if the processor returns a non-nil output and the output channel is not nil.
type Runner[InputEvent any, OutputEvent any] struct {
	Processor   Processor[InputEvent, OutputEvent]
	inputChan   <-chan InputEvent
	outputChan  chan<- OutputEvent
	errChan     chan<- ErrorEvent
	logger      logr.Logger
	stopChan    chan struct{}
	wg          sync.WaitGroup
	stopped     bool
	stopMu      sync.Mutex
	rateLimiter *rate.Limiter
	backoff     *backoffConfig
}

func NewRunner[InputEvent any, OutputEvent any](
	processor Processor[InputEvent, OutputEvent],
	inputChan <-chan InputEvent,
	outputChan chan<- OutputEvent,
	errChan chan<- ErrorEvent,
) *Runner[InputEvent, OutputEvent] {
	r := &Runner[InputEvent, OutputEvent]{
		Processor:  processor,
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
			r.logger.Error(err, "rate limiter wait failed")
			return
		}
	}

	outputEvents, err := r.Processor.Process(ctx, ev)
	if err != nil {
		r.logger.Error(err, "failed to process event", "event", ev)
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

func (r *Runner[InputEvent, OutputEvent]) Logger() logr.Logger {
	return r.logger
}

// RetryOptions returns the backoff RetryOptions configured for this Runner, or
// nil if no backoff is configured. A fresh ExponentialBackOff is constructed on
// each call so callers do not share mutable state.
func (r *Runner[InputEvent, OutputEvent]) RetryOptions() []backoff.RetryOption {
	if r.backoff == nil {
		return nil
	}
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = r.backoff.initialInterval
	b.MaxInterval = r.backoff.maxInterval

	return []backoff.RetryOption{
		backoff.WithBackOff(b),
		backoff.WithMaxElapsedTime(r.backoff.maxElapsedTime),
	}
}
