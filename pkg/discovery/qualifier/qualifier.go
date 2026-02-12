// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package qualifier

import (
	"context"
	"fmt"
	"time"

	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/repositories/ocireg"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type Qualifier struct {
	*discovery.Runner[discovery.RepositoryEvent, discovery.ComponentVersionEvent]
	provider *discovery.RegistryProvider
}

func NewQualifier(
	provider *discovery.RegistryProvider,
	in <-chan discovery.RepositoryEvent,
	out chan<- discovery.ComponentVersionEvent,
	err chan<- discovery.ErrorEvent,
	opts ...discovery.RunnerOption[discovery.RepositoryEvent, discovery.ComponentVersionEvent],
) *Qualifier {
	p := &Qualifier{
		provider: provider,
	}
	p.Runner = discovery.NewRunner(p, in, out, err)
	for _, opt := range opts {
		opt(p.Runner)
	}

	return p
}

func NewQualifierOptions(opts ...discovery.RunnerOption[discovery.RepositoryEvent, discovery.ComponentVersionEvent]) []discovery.RunnerOption[discovery.RepositoryEvent, discovery.ComponentVersionEvent] {
	return opts
}

func (rs *Qualifier) Process(ctx context.Context, ev discovery.RepositoryEvent) ([]discovery.ComponentVersionEvent, error) {
	// Implement checking if the mediatype of the found oci image is an ocm component
	rs.Logger().Info("processing event", "registry", ev.Registry, "repository", ev.Repository)

	ns, comp, err := discovery.SplitRepository(ev.Repository)
	if err != nil {
		rs.Logger().V(2).Info("discovery.SplitRepository returned error", "error", err)
		return nil, fmt.Errorf("invalid repository format: %w", err)
	}

	compVerEvent := discovery.ComponentVersionEvent{
		Timestamp: time.Now().UTC(),
		Source:    ev,
		Namespace: ns,
		Component: comp,
	}

	// Exit early on deletion
	if ev.Type == discovery.EventDeleted {
		return []discovery.ComponentVersionEvent{compVerEvent}, nil
	}

	// If version is specified, lookup that specific version and return
	// Otherwise, lookup the component
	if ev.Version != "" {
		return []discovery.ComponentVersionEvent{compVerEvent}, nil
	}

	// Get registry configuration
	registry := rs.provider.Get(ev.Registry)
	if registry == nil {
		rs.Logger().V(2).Info("invalid registry", "registry", ev.Registry)
		return nil, fmt.Errorf("invalid registry: %s", ev.Registry)
	}

	// Create repository for the component
	baseURL := fmt.Sprintf("%s/%s", registry.GetURL(), ns)
	octx := ocm.FromContext(ctx)
	repo, err := octx.RepositoryForSpec(ocireg.NewRepositorySpec(baseURL))
	if err != nil {
		rs.Logger().Error(err, "failed to create repo spec", "registry", ev.Registry, "repository", ev.Repository)
		return nil, fmt.Errorf("failed to create repository spec: %w", err)
	}
	defer func() { _ = repo.Close() }()

	component, err := repo.LookupComponent(comp)
	if err != nil {
		rs.Logger().Error(err, "failed to lookup component", "component", comp)
		return nil, fmt.Errorf("failed to lookup component: %w", err)
	}
	defer func() { _ = component.Close() }()

	// List all versions of the component
	componentVersions, err := component.ListVersions()
	if err != nil {
		rs.Logger().Error(err, "failed to list component versions", "component", comp)
		return nil, fmt.Errorf("failed to list component versions: %w", err)
	}

	componentVersionEvents := make([]discovery.ComponentVersionEvent, 0, len(componentVersions))
	for _, version := range componentVersions {
		compVerEvent.Source.Version = version
		componentVersionEvents = append(componentVersionEvents, compVerEvent)
	}

	return componentVersionEvents, nil
}
