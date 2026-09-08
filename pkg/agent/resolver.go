// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solarclientset "go.opendefense.cloud/solar/client-go/clientset/versioned"
)

// TargetResolver fetches the Target this agent reports for.
// The Get doubles as the agent's startup verification: it proves the endpoint is
// a solar-apiserver, that the credential is accepted, and that the Target exists.
type TargetResolver struct {
	Client    solarclientset.Interface
	Namespace string
	Name      string
}

// ResolveTarget returns the Target this agent belongs to.
func (r *TargetResolver) ResolveTarget(ctx context.Context) (*solarv1alpha1.Target, error) {
	target, err := r.Client.SolarV1alpha1().Targets(r.Namespace).Get(ctx, r.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("resolving target %s/%s: %w", r.Namespace, r.Name, err)
	}

	return target, nil
}
