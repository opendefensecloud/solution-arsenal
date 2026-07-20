// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solarclientset "go.opendefense.cloud/solar/client-go/clientset/versioned"
)

// Registrar ensures this agent's Target exists on solar-apiserver, creating
// it from Spec on first run
type Registrar struct {
	Client    solarclientset.Interface
	Namespace string
	Name      string
	Spec      solarv1alpha1.TargetSpec
}

// EnsureTarget returns the agent's Target, creating it if it doesn't exist yet.
func (r *Registrar) EnsureTarget(ctx context.Context) (*solarv1alpha1.Target, error) {
	existing, err := r.Client.SolarV1alpha1().Targets(r.Namespace).Get(ctx, r.Name, metav1.GetOptions{})
	if err == nil {
		return existing, nil
	}

	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("getting target %s/%s: %w", r.Namespace, r.Name, err)
	}

	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{Name: r.Name, Namespace: r.Namespace},
		Spec:       r.Spec,
	}

	created, err := r.Client.SolarV1alpha1().Targets(r.Namespace).Create(ctx, target, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating target %s/%s: %w", r.Namespace, r.Name, err)
	}

	return created, nil
}
