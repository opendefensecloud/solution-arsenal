// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
)

type Filter struct {
	*discovery.Runner[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent]
	solarClient v1alpha1.SolarV1alpha1Interface
	namespace   string
}

func NewFilter(
	solarClient v1alpha1.SolarV1alpha1Interface,
	namespace string,
	in <-chan discovery.ComponentVersionEvent,
	out chan<- discovery.ComponentVersionEvent,
	err chan<- discovery.ErrorEvent,
	opts ...discovery.RunnerOption[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent],
) *Filter {
	p := &Filter{
		solarClient: solarClient,
		namespace:   namespace,
	}
	p.Runner = discovery.NewRunner(p, in, out, err)
	for _, opt := range opts {
		opt(p.Runner)
	}

	return p
}

func NewFilterOptions(opts ...discovery.RunnerOption[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent]) []discovery.RunnerOption[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent] {
	return opts
}

func (rs *Filter) Process(ctx context.Context, ev discovery.ComponentVersionEvent) ([]discovery.ComponentVersionEvent, error) {
	// We have to check if the component version already exists in the cluster to avoid creating duplicate component versions.
	_, err := rs.solarClient.ComponentVersions(rs.namespace).Get(ctx, discovery.ComponentVersionName(ev), metav1.GetOptions{})
	switch {
	case err == nil:
		// Component version already exists, skip creating it again
		rs.Logger().V(2).Info("component version already exists, skipping", "component", ev.Component, "version", ev.Source.Version)
		return nil, nil
	case errors.IsNotFound(err):
		// Component version does not exist, we can create it
		return []discovery.ComponentVersionEvent{ev}, nil
	default:
		// An error occurred while trying to get the component version, return the error
		return nil, fmt.Errorf("failed to get component version from API: %w", err)
	}
}
