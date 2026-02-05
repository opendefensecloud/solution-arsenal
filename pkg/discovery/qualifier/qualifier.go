// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package qualifier

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

type Qualifier struct {
	provider    *discovery.RegistryProvider
	inputChan   <-chan discovery.RepositoryEvent
	outputChan  chan<- discovery.ComponentVersionEvent
	errChan     chan<- discovery.ErrorEvent
	logger      logr.Logger
	stopChan    chan struct{}
	wg          sync.WaitGroup
	stopped     bool
	stopMu      sync.Mutex
	rateLimiter *rate.Limiter
	backoff     backoff.BackOff
}

// Option describes the available options
// for creating the Qualifier.
type Option func(r *Qualifier)

func WithLogger(l logr.Logger) Option {
	return func(r *Qualifier) {
		r.logger = l
	}
}

// WithRateLimiter sets the rate limiter for the Qualifier that allows events up to the given interval and burst.
func WithRateLimiter(interval time.Duration, burst int) Option {
	return func(r *Qualifier) {
		r.rateLimiter = rate.NewLimiter(rate.Every(interval), burst)
	}
}

// WithExponentialBackoff sets an exponential backoff strategy for the Qualifier.
func WithExponentialBackoff(initialInterval time.Duration, maxInterval time.Duration, maxElapsedTime time.Duration) Option {
	return func(r *Qualifier) {
		b := backoff.NewExponentialBackOff()
		b.InitialInterval = initialInterval
		b.MaxInterval = maxInterval
		b.MaxElapsedTime = maxElapsedTime
		r.backoff = b
	}
}

func NewQualifier(
	provider *discovery.RegistryProvider,
	in <-chan discovery.RepositoryEvent,
	out chan<- discovery.ComponentVersionEvent,
	err chan<- discovery.ErrorEvent,
	opts ...Option,
) *Qualifier {
	c := &Qualifier{
		provider:   provider,
		inputChan:  in,
		outputChan: out,
		errChan:    err,
		logger:     logr.Discard(),
		stopChan:   make(chan struct{}),
	}

	for _, o := range opts {
		o(c)
	}

	return c
}

// Start begins continuous scanning of the registry in a separate goroutine.
// The scanner will continue until Stop() is called.
func (rs *Qualifier) Start(ctx context.Context) error {
	rs.logger.Info("starting qualifier")

	rs.wg.Add(1)
	go rs.catalogLoop(ctx)

	return nil
}

// Stop gracefully stops the qualifier.
func (rs *Qualifier) Stop() {
	rs.stopMu.Lock()
	defer rs.stopMu.Unlock()

	if rs.stopped {
		return
	}

	rs.logger.Info("stopping qualifier")
	rs.stopped = true
	close(rs.stopChan)
	rs.wg.Wait()
	rs.logger.Info("qualifier stopped")
}

// catalogLoop continuously reads events from the channel.
func (rs *Qualifier) catalogLoop(ctx context.Context) {
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

// isRetryable determines if we should wait and try again
func isRetryable(err error) bool {
	msg := strings.ToLower(err.Error())
	// OCM often wraps errors, so we check the string for common rate-limit indicators
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "connection refused")
}

// lookupComponentVersionAndPublish looks up a specific component version and publishes the result as event.
func (rs *Qualifier) lookupComponentVersionAndPublish(ctx context.Context, version string, comp string, event discovery.ComponentVersionEvent, repo ocm.Repository) {
	// If rate limiter is configured, wait before making the request
	if rs.rateLimiter != nil {
		if err := rs.rateLimiter.Wait(ctx); err != nil {
			rs.logger.Error(err, "rate limiter wait failed")
			return
		}
	}

	// Lookup the specific component version
	var compVersion ocm.ComponentVersionAccess
	var err error
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

	componentDescriptor := compVersion.GetDescriptor()
	rs.logger.Info("found component version",
		"componentDescriptor", componentDescriptor.GetName(),
		"version", componentDescriptor.GetVersion(),
	)
	event.Descriptor = componentDescriptor

	discovery.Publish(&rs.logger, rs.outputChan, event)
}

func (rs *Qualifier) processEvent(ctx context.Context, ev discovery.RepositoryEvent) {
	// Implement checking if the mediatype of the found oci image is an ocm component
	octx := ocm.FromContext(ctx)

	rs.logger.Info("processing event", "registry", ev.Registry, "repository", ev.Repository)

	ns, comp, err := discovery.SplitRepository(ev.Repository)
	if err != nil {
		rs.logger.V(2).Info("discovery.SplitRepository returned error", "error", err)
		return
	}

	res := discovery.ComponentVersionEvent{
		Timestamp: time.Now().UTC(),
		Source:    ev,
		Namespace: ns,
		Component: comp,
		Type:      ev.Type,
	}

	// Exit early on deletion
	if ev.Type == discovery.EventDeleted {
		discovery.Publish(&rs.logger, rs.outputChan, res)
		return
	}

	// Get registry configuration
	registry := rs.provider.Get(ev.Registry)
	if registry == nil {
		rs.logger.V(2).Info("invalid registry", "registry", ev.Registry)
		return
	}

	// Create repository for the component
	baseURL := fmt.Sprintf("%s/%s", registry.GetURL(), ns)
	repo, err := octx.RepositoryForSpec(ocireg.NewRepositorySpec(baseURL))
	if err != nil {
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Timestamp: time.Now().UTC(),
			Error:     fmt.Errorf("failed to create repo spec: %w", err),
		})
		rs.logger.Error(err, "failed to create repo spec", "registry", ev.Registry, "repository", ev.Repository)

		return
	}
	defer func() { _ = repo.Close() }()

	// If version is specified, lookup that specific version and return
	if ev.Version != "" {
		rs.lookupComponentVersionAndPublish(ctx, ev.Version, comp, res, repo)

		return
	}

	// Otherwise, lookup the component
	component, err := repo.LookupComponent(comp)
	if err != nil {
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Timestamp: time.Now().UTC(),
			Error:     fmt.Errorf("failed to lookup component: %w", err),
		})
		rs.logger.Error(err, "failed to lookup component", "component", comp)

		return
	}
	defer func() { _ = component.Close() }()

	// List all versions of the component
	componentVersions, err := component.ListVersions()
	if err != nil {
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Timestamp: time.Now().UTC(),
			Error:     fmt.Errorf("failed to list component versions: %w", err),
		})
		rs.logger.Error(err, "failed to list component versions", "component", comp)

		return
	}

	for _, version := range componentVersions {
		rs.lookupComponentVersionAndPublish(ctx, version, comp, res, repo)
	}
}
