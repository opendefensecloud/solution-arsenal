// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package apiwriter

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cenkalti/backoff/v4"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"ocm.software/ocm/api/datacontext"
	"ocm.software/ocm/api/oci"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociartifact"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
)

const (
	componentLabel = "solar.opendefense.cloud/component"
)

var _ discovery.Processor[discovery.WriteAPIResourceEvent, any] = &APIWriter{}

type APIWriter struct {
	*discovery.Runner[discovery.WriteAPIResourceEvent, any]

	client    v1alpha1.SolarV1alpha1Interface
	namespace string
	provider  *discovery.RegistryProvider
}

func NewAPIWriter(
	client v1alpha1.SolarV1alpha1Interface,
	namespace string,
	provider *discovery.RegistryProvider,
	in <-chan discovery.WriteAPIResourceEvent,
	err chan<- discovery.ErrorEvent,
	opts ...discovery.RunnerOption[discovery.WriteAPIResourceEvent, any],
) *APIWriter {

	p := &APIWriter{
		client:    client,
		namespace: namespace,
		provider:  provider,
	}
	p.Runner = discovery.NewRunner(p, in, nil, err)

	for _, opt := range opts {
		opt(p.Runner)
	}

	return p
}

func (rs *APIWriter) Process(ctx context.Context, ev discovery.WriteAPIResourceEvent) ([]any, error) {
	var op backoff.Operation

	switch ev.Source.Source.Type {
	case discovery.EventCreated, discovery.EventUpdated:
		ref, err := rs.getOciRef(ev)
		if err != nil {
			return nil, err
		}
		spec := ev.ComponentSpec
		op = func() error { return rs.ensureComponentVersion(ctx, ref, spec, ev) }
	case discovery.EventDeleted:
		ref, err := rs.getOciRef(ev)
		if err != nil {
			return nil, err
		}
		op = func() error { return rs.deleteComponentVersion(ctx, ref, ev.Source.Component) }
	default:
		return nil, fmt.Errorf("SHOULD NOT HAPPEN: Invalid event type: %s", ev.Source.Source.Type)
	}

	// Retry selected operation if a backoff is configured
	if rs.Backoff() != nil {
		return nil, backoff.Retry(op, rs.Backoff())
	}

	return nil, op()
}

func (rs *APIWriter) ensureComponentVersion(ctx context.Context, ref oci.RefSpec, spec compdesc.ComponentSpec, ev discovery.WriteAPIResourceEvent) error {
	if err := rs.ensureComponent(ctx, ref, spec); err != nil {
		return err
	}

	octx := ocm.New(datacontext.MODE_SHARED)
	defer func() { _ = octx.Finalize() }()

	// Get Resources
	resources := map[string]solarv1alpha1.ResourceAccess{}
	for _, res := range spec.Resources {
		ra := solarv1alpha1.ResourceAccess{}

		acc, err := octx.AccessSpecForSpec(res.GetAccess())
		if err != nil {
			return fmt.Errorf("failed to parse access spec for resource %s: %w", res.Name, err)
		}

		switch typed := acc.(type) {
		// NOTE: Currently only OCI is supported
		case *ociartifact.AccessSpec:
			ociref, err := oci.ParseRef(typed.ImageReference)
			if err != nil {
				return err
			}
			repository, err := url.JoinPath(ociref.Host, ociref.Repository)
			if err != nil {
				return err
			}
			ra.Repository = fmt.Sprintf("%s://%s", ociref.Scheme, repository)
			ra.Tag = ociref.Version()

		default:
			return fmt.Errorf("unsupported access type: %s", acc.GetType())
		}

		resources[res.Name] = ra
	}

	// Get Entrypoint
	entrypoint := solarv1alpha1.Entrypoint{}
	if ev.HelmDiscovery.ResourceName != "" {
		entrypoint.ResourceName = ev.HelmDiscovery.ResourceName
		entrypoint.Type = solarv1alpha1.EntrypointTypeHelm
	}
	// NOTE: Currently only helm is supported as Entrypoint

	// Validate Entrypoint
	if _, ok := resources[entrypoint.ResourceName]; entrypoint.ResourceName != "" && !ok {
		return fmt.Errorf("entrypoint `%s` was not provided in resource map", entrypoint.ResourceName)
	}

	comp := discovery.SanitizeWithHash(spec.Name)

	cv := &solarv1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: discovery.ComponentVersionName(spec.Name, ref.Version()),
			Labels: map[string]string{
				componentLabel: comp,
			},
		},
		Spec: solarv1alpha1.ComponentVersionSpec{
			ComponentRef: v1.LocalObjectReference{
				Name: comp,
			},
			Tag:        ref.Version(),
			Resources:  resources,
			Entrypoint: entrypoint,
		},
	}

	_, err := rs.client.ComponentVersions(rs.namespace).Create(ctx, cv, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		_, err = rs.client.ComponentVersions(rs.namespace).Update(ctx, cv, metav1.UpdateOptions{})
	}

	return err
}

func (rs *APIWriter) deleteComponentVersion(ctx context.Context, ref oci.RefSpec, component string) error {
	cv := discovery.ComponentVersionName(component, ref.Version())
	if err := client.IgnoreNotFound(rs.client.ComponentVersions(rs.namespace).Delete(ctx, cv, metav1.DeleteOptions{})); err != nil {
		return err
	}

	// Clean up component if noone references it.
	parent := discovery.SanitizeWithHash(component)
	matchLabels := map[string]string{
		componentLabel: parent,
	}
	cvList, err := rs.client.ComponentVersions(rs.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(matchLabels).String(),
	})
	if err != nil {
		return err
	}
	if len(cvList.Items) == 0 {
		return client.IgnoreNotFound(rs.client.Components(rs.namespace).Delete(ctx, parent, metav1.DeleteOptions{}))
	}

	return nil
}

func (rs *APIWriter) ensureComponent(ctx context.Context, ref oci.RefSpec, spec compdesc.ComponentSpec) error {
	c := &solarv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name: discovery.SanitizeWithHash(spec.Name),
		},
		Spec: solarv1alpha1.ComponentSpec{
			Scheme:     ref.Scheme,
			Registry:   ref.Host,
			Repository: ref.Repository,
		},
	}
	_, err := rs.client.Components(rs.namespace).Create(ctx, c, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		_, err = rs.client.Components(rs.namespace).Update(ctx, c, metav1.UpdateOptions{})
	}

	return err
}

func (rs *APIWriter) getOciRef(ev discovery.WriteAPIResourceEvent) (oci.RefSpec, error) {
	registry := rs.provider.Get(ev.Source.Source.Registry)
	if registry == nil {
		rs.Logger().V(2).Info("invalid registry", "registry", ev.Source.Source.Registry)
		return oci.RefSpec{}, fmt.Errorf("invalid registry: %s", ev.Source.Source.Registry)
	}
	cvURL := fmt.Sprintf("%s/%s/%s:%s", registry.GetURL(), ev.Source.Namespace, ev.Source.Component, ev.Source.Source.Version)

	return oci.ParseRef(cvURL)
}
