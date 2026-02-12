// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package qualifier

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/repositories/ocireg"

	"go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
)

type Qualifier struct {
	*discovery.Runner[discovery.RepositoryEvent, discovery.ComponentVersionEvent]
	provider    *discovery.RegistryProvider
	solarClient v1alpha1.SolarV1alpha1Interface
	namespace   string
}

func NewQualifier(
	provider *discovery.RegistryProvider,
	solarClient v1alpha1.SolarV1alpha1Interface,
	namespace string,
	in <-chan discovery.RepositoryEvent,
	out chan<- discovery.ComponentVersionEvent,
	err chan<- discovery.ErrorEvent,
	opts ...discovery.RunnerOption[discovery.RepositoryEvent, discovery.ComponentVersionEvent],
) *Qualifier {
	p := &Qualifier{
		provider:    provider,
		solarClient: solarClient,
		namespace:   namespace,
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

	// If version is specified, we can skip the lookup and just return the event as-is
	// Otherwise, lookup the component
	if ev.Version != "" {
		// We have to check if the component version already exists in the cluster to avoid creating duplicate component versions.
		_, err := rs.solarClient.ComponentVersions(rs.namespace).Get(ctx, discovery.SanitizeName(fmt.Sprintf("%s-%s", comp, ev.Version)), metav1.GetOptions{})
		switch {
		case err == nil:
			// Component version already exists, skip creating it again
			rs.Logger().V(2).Info("component version already exists, skipping", "component", comp, "version", ev.Version)
			return nil, nil
		case errors.IsNotFound(err):
			// Component version does not exist, we can create it
			return []discovery.ComponentVersionEvent{compVerEvent}, nil
		default:
			// An error occurred while trying to get the component version, return the error
			return nil, fmt.Errorf("failed to get component version from API: %w", err)
		}
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

	// Lookup component to verify it exists and get metadata
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

	// Create a ComponentVersionEvent for each version of the component and return them as output events. The handler will then process each version separately.
	componentVersionEvents := make([]discovery.ComponentVersionEvent, 0, len(componentVersions))
	for _, version := range componentVersions {
		// We have to check if the component version already exists in the cluster to avoid creating duplicate component versions.

		_, err := rs.solarClient.ComponentVersions(rs.namespace).Get(ctx, discovery.SanitizeWithHash(fmt.Sprintf("%s-%s", comp, version)), metav1.GetOptions{})
		switch {
		case err == nil:
			// Component version already exists, skip creating it again
			rs.Logger().V(2).Info("component version already exists, skipping", "component", comp, "version", ev.Version)
			return nil, nil
		case errors.IsNotFound(err):
			compVerEvent.Source.Version = version
			componentVersionEvents = append(componentVersionEvents, compVerEvent)
		default:
			// An error occurred while trying to get the component version, return the error
			return nil, fmt.Errorf("failed to get component version from API: %w", err)
		}
	}

	return componentVersionEvents, nil
}
