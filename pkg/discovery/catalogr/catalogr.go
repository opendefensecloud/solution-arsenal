// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package catalogr

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"go.opendefense.cloud/solar/pkg/discovery"
	"k8s.io/client-go/kubernetes"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/repositories/ocireg"
)

type Catalogr struct {
	clientSet  kubernetes.Interface
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

func NewCatalogr(clientSet kubernetes.Interface, eventsChan <-chan discovery.RegistryEvent, opts ...Option) *Catalogr {
	c := &Catalogr{
		clientSet:  clientSet,
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
	// TODO: split before sending event => qualify early
	namespace, component, err := splitRepository(ev.Repository)
	if err != nil {
		rs.logger.V(2).Info("splitting string returned: %v", err)
		return
	}

	// Implement checking if the mediatype of the found oci image is an ocm component
	octx := ocm.FromContext(ctx)

	// TODO: remove hardcoding of http://
	repoSpec := ocireg.NewRepositorySpec(fmt.Sprintf("http://%s/%s", ev.Registry, namespace))
	repo, err := octx.RepositoryForSpec(repoSpec)
	if err != nil {
		rs.logger.Error(err, "failed to create repo spec", "registry", ev.Registry, "repository", ev.Repository)
		return
	}
	defer func() { _ = repo.Close() }()

	compVersion, err := repo.LookupComponentVersion(component, ev.Tag)
	if err != nil {
		rs.logger.Error(err, "failed to lookup component", "tag", ev.Tag)
		return
	}
	defer func() { _ = compVersion.Close() }()

	rs.logger.Info("found component version", "componentDescriptor", compVersion.GetDescriptor())
}

func splitRepository(repo string) (string, string, error) {
	parts := strings.Split(repo, "/component-descriptors/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repository is not a component descriptor: splitting '%s' at './component-descriptors/' returns %d parts, expected exactly 2", repo, len(parts))
	}
	return parts[0], parts[1], nil
}
