// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
type KubePublisher struct {
	Client solarclientset.Interface
}

func (k KubePublisher) Publish(ctx context.Context, r solarv1alpha1.TargetReport) error {
	existing, err := k.Client.SolarV1alpha1().TargetReports(r.Namespace).Get(ctx, r.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = k.Client.SolarV1alpha1().TargetReports(r.Namespace).Create(ctx, &r, metav1.CreateOptions{})
			if err != nil {
				return err
			}

			return nil
		}

		return err
	}

	r.ResourceVersion = existing.ResourceVersion
	_, err = k.Client.SolarV1alpha1().TargetReports(r.Namespace).Update(ctx, &r, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
