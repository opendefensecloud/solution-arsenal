// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package catalogr

import (
	"context"
	"fmt"
	"sync"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/repositories/ocireg"
)

type Catalogr struct {
	client.Client
	eventsChan <-chan discovery.RegistryEvent
	logger     logr.Logger
	stopChan   chan struct{}
	wg         sync.WaitGroup
	stopped    bool
	stopMu     sync.Mutex
}

// Option describes the available options
// for creating the Catalogr.
type Option func(r *Catalogr)

func WithLogger(l logr.Logger) Option {
	return func(r *Catalogr) {
		r.logger = l
	}
}

func NewCatalogr(eventsChan <-chan discovery.RegistryEvent, opts ...Option) *Catalogr {
	c := &Catalogr{
		eventsChan: eventsChan,
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
func (rs *Catalogr) Start(ctx context.Context) error {
	rs.logger.Info("starting catalogr")

	rs.wg.Add(1)
	go rs.catalogLoop(ctx)

	return nil
}

// Stop gracefully stops the catalogr.
func (rs *Catalogr) Stop() {
	rs.stopMu.Lock()
	defer rs.stopMu.Unlock()

	if rs.stopped {
		return
	}

	rs.logger.Info("stopping catalogr")
	rs.stopped = true
	close(rs.stopChan)
	rs.wg.Wait()
	rs.logger.Info("catalogr stopped")
}

// catalogLoop continuously reads events from the channel.
func (rs *Catalogr) catalogLoop(ctx context.Context) {
	defer rs.wg.Done()

	for {
		select {
		case <-rs.stopChan:
			return
		case <-ctx.Done():
			return
		case ev := <-rs.eventsChan:
			rs.processEvent(ctx, ev)
		}
	}
}

func (rs *Catalogr) processEvent(ctx context.Context, ev discovery.RegistryEvent) {
	// Implement checking if the mediatype of the found oci image is an ocm component
	octx := ocm.FromContext(ctx)

	rs.logger.Info("processing event", "registry", ev.Registry, "repository", ev.Repository, "tag", ev.Tag)

	baseURL := fmt.Sprintf("%s://%s/%s", ev.Schema, ev.Registry, ev.Namespace)
	repoSpec := ocireg.NewRepositorySpec(baseURL)
	repo, err := octx.RepositoryForSpec(repoSpec)
	if err != nil {
		rs.logger.Error(err, "failed to create repo spec", "registry", ev.Registry, "repository", ev.Repository)
		return
	}
	defer func() { _ = repo.Close() }()

	compVersion, err := repo.LookupComponentVersion(ev.Component, ev.Tag)
	if err != nil {
		rs.logger.Error(err, "failed to lookup component", "tag", ev.Tag)
		return
	}
	defer func() { _ = compVersion.Close() }()

	componentDescriptor := compVersion.GetDescriptor()
	rs.logger.Info("found component version", "componentDescriptor", componentDescriptor.GetName(), "version", componentDescriptor.GetVersion())

	ci, err := rs.getOrCreateCatalogItem(ctx, componentDescriptor.GetName())
	if err != nil {
		rs.logger.Error(err, "failed to get or create CatalogItem", "name", componentDescriptor.GetName())
		return
	}
	ci.Spec.Provider = string(componentDescriptor.Provider.Name)
	ci.Spec.CreationTime = v1.NewTime(componentDescriptor.CreationTime.Time())

	// Discover resources contained in the component descriptor
	for _, r := range componentDescriptor.Resources {
		rs.logger.Info("discovered resource", "name", r.Name, "version", r.Version, "type", r.Type, "accessType", r.Access.GetType())
	}
}

// getOrCreateCatalogItem retrieves or creates a CatalogItem by name.
func (rs *Catalogr) getOrCreateCatalogItem(ctx context.Context, name string) (*v1alpha1.CatalogItem, error) {
	ci := &v1alpha1.CatalogItem{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
	}
	if err := rs.Get(ctx, client.ObjectKey{Name: name}, ci); client.IgnoreNotFound(err) != nil {
		return nil, err
	}
	return ci, nil
}
