// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package apiwriter

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	component, err := rs.client.Components(rs.namespace).Get(ctx, ev.Source.Component, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	if component == nil || component.Name == "" {
		if err := rs.createComponent(ctx, ev.Source); err != nil {
			return err
		}
	}

	cv := buildComponentVersion(ev)
	_, err = rs.client.ComponentVersions(rs.namespace).Create(ctx, cv, metav1.CreateOptions{})

	return err
}

func (rs *APIWriter) updateComponentVersion(ctx context.Context, ev discovery.WriteAPIResourceEvent) error {
	cv := buildComponentVersion(ev)
	_, err := rs.client.ComponentVersions(rs.namespace).Update(ctx, cv, metav1.UpdateOptions{})

	return err
}

func (rs *APIWriter) deleteComponentVersion(ctx context.Context, ev discovery.WriteAPIResourceEvent) error {
	cv := discovery.ComponentVersionName(ev.Source)
	if err := rs.client.ComponentVersions(rs.namespace).Delete(ctx, cv, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// Clean up component if noone references it.
	parent := ev.Source.Component
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

func (rs *APIWriter) createComponent(ctx context.Context, ev discovery.ComponentVersionEvent) error {
	c := &solarv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name: ev.Component,
		},
		Spec: solarv1alpha1.ComponentSpec{}, // TODO
	}

	_, err := rs.client.Components(rs.namespace).Create(ctx, c, metav1.CreateOptions{})

	return err
}

func buildComponentVersion(ev discovery.WriteAPIResourceEvent) *solarv1alpha1.ComponentVersion {
	return &solarv1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: discovery.ComponentVersionName(ev.Source),
			Labels: map[string]string{
				componentLabel: ev.Source.Component,
			},
		},
		Spec: solarv1alpha1.ComponentVersionSpec{}, // TODO
	}
}
