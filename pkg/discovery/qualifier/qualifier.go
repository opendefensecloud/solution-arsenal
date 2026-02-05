// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package qualifier

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/repositories/ocireg"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type Qualifier struct {
	provider   *discovery.RegistryProvider
	inputChan  <-chan discovery.RepositoryEvent
	outputChan chan<- discovery.ComponentVersionEvent
	errChan    chan<- discovery.ErrorEvent
	logger     logr.Logger
	stopChan   chan struct{}
	wg         sync.WaitGroup
	stopped    bool
	stopMu     sync.Mutex
}

// Option describes the available options
// for creating the Qualifier.
type Option func(r *Qualifier)

func WithLogger(l logr.Logger) Option {
	return func(r *Qualifier) {
		r.logger = l
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

// lookupComponentVersionAndPublish looks up a specific component version and publishes the result as event.
func (rs *Qualifier) lookupComponentVersionAndPublish(version string, comp string, res discovery.ComponentVersionEvent, repo ocm.Repository) {
	compVersion, err := repo.LookupComponentVersion(comp, version)
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

	res.Descriptor = componentDescriptor
	discovery.Publish(&rs.logger, rs.outputChan, res)
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
		rs.lookupComponentVersionAndPublish(ev.Version, comp, res, repo)

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
		rs.lookupComponentVersionAndPublish(version, comp, res, repo)
	}
}
