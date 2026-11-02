// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solarclientset "go.opendefense.cloud/solar/client-go/clientset/versioned"
)

// Publisher delivers a TargetReport somewhere.
type Publisher interface {
	Publish(ctx context.Context, r solarv1alpha1.TargetReport) error
}

// LogPublisher logs the report instead of sending it anywhere.
type LogPublisher struct {
	Log logr.Logger
}

func (l LogPublisher) Publish(_ context.Context, r solarv1alpha1.TargetReport) error {
	l.Log.Info("target report",
		"releases", r.Spec.Releases,
		"checks", r.Spec.Preflight.Checks,
	)

	return nil
}

// KubePublisher publishes TargetReports to the solar-apiserver via the solar clientset.
// It only updates: solar-controller-manager pre-creates each Target's report, and the
// agent's Role grants get and update on it, never create.
type KubePublisher struct {
	Client solarclientset.Interface
}

func (k KubePublisher) Publish(ctx context.Context, r solarv1alpha1.TargetReport) error {
	existing, err := k.Client.SolarV1alpha1().TargetReports(r.Namespace).Get(ctx, r.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting TargetReport %s/%s: %w", r.Namespace, r.Name, err)
	}

	// Keep the stored metadata (ownerReferences, labels, annotations set by the
	// controller) and replace only the spec.
	existing.Spec = r.Spec
	if _, err := k.Client.SolarV1alpha1().TargetReports(r.Namespace).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating TargetReport %s/%s: %w", r.Namespace, r.Name, err)
	}

	return nil
}
