// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package apiwriter

import (
	"context"
	"fmt"
	"net/url"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"ocm.software/ocm/api/datacontext"
	"ocm.software/ocm/api/oci"
	"ocm.software/ocm/api/ocm"
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
}

func NewAPIWriter(
	client v1alpha1.SolarV1alpha1Interface,
	namespace string,
	in <-chan discovery.WriteAPIResourceEvent,
	err chan<- discovery.ErrorEvent,
	opts ...discovery.RunnerOption[discovery.WriteAPIResourceEvent, any],
) *APIWriter {

	p := &APIWriter{
		client:    client,
		namespace: namespace,
	}
	p.Runner = discovery.NewRunner(p, in, nil, err)

	for _, opt := range opts {
		opt(p.Runner)
	}

	return p
}

func (rs *APIWriter) Process(ctx context.Context, ev discovery.WriteAPIResourceEvent) ([]any, error) {
	rs.Logger().Info("processing WriteAPIResourceEvent")

	typ := ev.Source.Source.Type

	switch typ {
	case discovery.EventCreated:
		return nil, rs.createComponentVersion(ctx, ev)
	case discovery.EventUpdated:
		return nil, rs.updateComponentVersion(ctx, ev)
	case discovery.EventDeleted:
		return nil, rs.deleteComponentVersion(ctx, ev)
	default:
		return nil, fmt.Errorf("SHOULD NOT HAPPEN: Invalid event type: %s", typ)
	}
}

func (rs *APIWriter) createComponentVersion(ctx context.Context, ev discovery.WriteAPIResourceEvent) error {
	component, err := rs.client.Components(rs.namespace).Get(ctx, discovery.SanitizeWithHash(ev.Source.Component), metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	if component == nil || component.Name == "" {
		if err := rs.createComponent(ctx, ev); err != nil {
			return err
		}
	}

	cv, err := buildComponentVersion(ev)
	if err != nil {
		return err
	}
	_, err = rs.client.ComponentVersions(rs.namespace).Create(ctx, cv, metav1.CreateOptions{})

	return err
}

func (rs *APIWriter) updateComponentVersion(ctx context.Context, ev discovery.WriteAPIResourceEvent) error {
	cv, err := buildComponentVersion(ev)
	if err != nil {
		return err
	}
	_, err = rs.client.ComponentVersions(rs.namespace).Update(ctx, cv, metav1.UpdateOptions{})

	return err
}

func (rs *APIWriter) deleteComponentVersion(ctx context.Context, ev discovery.WriteAPIResourceEvent) error {
	cv := discovery.ComponentVersionName(ev.Source)
	if err := rs.client.ComponentVersions(rs.namespace).Delete(ctx, cv, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// Clean up component if noone references it.
	parent := discovery.SanitizeWithHash(ev.Source.Component)
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
		return rs.client.Components(rs.namespace).Delete(ctx, parent, metav1.DeleteOptions{})
	}

	return nil
}

func (rs *APIWriter) createComponent(ctx context.Context, ev discovery.WriteAPIResourceEvent) error {
	ref, err := oci.ParseRef(ev.ResolvedComponentVersionURL)
	if err != nil {
		return err
	}

	c := &solarv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name: discovery.SanitizeWithHash(ev.Source.Component),
		},
		Spec: solarv1alpha1.ComponentSpec{
			Scheme:     ref.Scheme,
			Registry:   ref.Host,
			Repository: ref.Repository,
		},
	}
	_, err = rs.client.Components(rs.namespace).Create(ctx, c, metav1.CreateOptions{})

	return err
}

func buildComponentVersion(ev discovery.WriteAPIResourceEvent) (*solarv1alpha1.ComponentVersion, error) {
	octx := ocm.New(datacontext.MODE_SHARED)
	defer func() { _ = octx.Finalize() }()

	ref, err := oci.ParseRef(ev.ResolvedComponentVersionURL)
	if err != nil {
		return nil, err
	}

	version := ref.Version()

	// Get Resources
	resources := map[string]solarv1alpha1.ResourceAccess{}
	for _, res := range ev.ComponentSpec.Resources {
		ra := solarv1alpha1.ResourceAccess{}

		acc, err := octx.AccessSpecForSpec(res.GetAccess())
		if err != nil {
			return nil, fmt.Errorf("failed to parse access spec for resource %s: %w", res.Name, err)
		}

		switch typed := acc.(type) {
		case *ociartifact.AccessSpec:
			ref, err := oci.ParseRef(typed.ImageReference)
			if err != nil {
				return nil, err
			}
			repository, err := url.JoinPath(ref.Host, ref.Repository)
			if err != nil {
				return nil, err
			}
			ra.Repository = fmt.Sprintf("%s://%s", ref.Scheme, repository)
			ra.Tag = ref.Version()

		default:
			return nil, fmt.Errorf("unsupported access type: %s", acc.GetKind())
		}

		resources[res.Name] = ra
	}

	// TODO Get Entrypoint
	entrypoint := solarv1alpha1.Entrypoint{
		Type:         solarv1alpha1.EntrypointTypeHelm,
		ResourceName: "foo",
	}

	return &solarv1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: discovery.ComponentVersionName(ev.Source),
			Labels: map[string]string{
				componentLabel: discovery.SanitizeWithHash(ev.Source.Component),
			},
		},
		Spec: solarv1alpha1.ComponentVersionSpec{
			ComponentRef: v1.LocalObjectReference{
				Name: ev.Source.Component,
			},
			Tag:        version,
			Resources:  resources,
			Entrypoint: entrypoint,
		},
	}, nil
}
