// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// heartbeatTicks is how many unchanged intervals pass before the agent reports
// anyway. Pushing only on change would leave lastReportTime frozen on a healthy
// stable cluster, and a stale lastReportTime is what marks a Target dead
const heartbeatTicks = 10

// Agent runs the collect -> report loop on a fixed interval
type Agent struct {
	Collector *Collector
	Checks    []Check
	Publisher Publisher
	Interval  time.Duration
	Log       logr.Logger

	// TargetNamespace and TargetName name the TargetReport object this agent
	// owns.
	TargetNamespace string
	TargetName      string

	// MaxReportAge bounds the silence between reports. Defaults to
	// heartbeatTicks * Interval.
	MaxReportAge time.Duration

	last          *solarv1alpha1.TargetReport
	lastPublished time.Time
}

func (a *Agent) Run(ctx context.Context) {
	if a.MaxReportAge == 0 {
		a.MaxReportAge = heartbeatTicks * a.Interval
	}

	ticker := time.NewTicker(a.Interval)
	defer ticker.Stop()

	a.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.tick(ctx)
		}
	}
}

func (a *Agent) tick(ctx context.Context) {
	checks := runChecks(ctx, a.Checks)

	releases, err := a.Collector.CollectReleases(ctx)
	if err != nil {
		a.Log.Error(err, "collecting release status")
		if a.last != nil {
			releases = a.last.Spec.Releases
		}
	}

	report := solarv1alpha1.TargetReport{
		Name:      a.TargetName,
		Namespace: a.TargetNamespace,
		Spec: solarv1alpha1.TargetReportSpec{
			Releases: releases,
			Preflight: solarv1alpha1.PreflightReport{
				Checks: checks,
			},
		},
	}

	if !a.changed(report) {
		return
	}

	published := report
	published.Spec.LastReportTime = metav1.Now()

	if err := a.Publisher.Publish(ctx, published); err != nil {
		a.Log.Error(err, "pushing report")

		return
	}

	// save the report without LastReportTime
	a.last = &report
	a.lastPublished = time.Now()
}

func (a *Agent) changed(report solarv1alpha1.TargetReport) bool {
	if a.last == nil || !reflect.DeepEqual(report, *a.last) {
		return true
	}

	return time.Since(a.lastPublished) >= a.MaxReportAge
}
