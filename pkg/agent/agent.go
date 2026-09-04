// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// heartbeatTicks is how many unchanged intervals pass before the agent reports
// anyway. Pushing only on change would leave lastReportTime frozen on a healthy
// stable cluster, and a stale lastReportTime is what marks a Target dead
const heartbeatTicks = 10

// Agent runs the collect -> report loop on a fixed interval
type Agent struct {
	Collector *Collector
	Publisher Publisher
	Interval  time.Duration
	Log       logr.Logger

	// MaxReportAge bounds the silence between reports. Defaults to
	// heartbeatTicks * Interval.
	MaxReportAge time.Duration

	last          *TargetReport
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
	capacity, err := a.Collector.CollectCapacity(ctx)
	if err != nil {
		a.Log.Error(err, "collecting capacity")

		return
	}

	releases, err := a.Collector.CollectReleases(ctx)
	if err != nil {
		a.Log.Error(err, "collecting release status")

		return
	}

	report := TargetReport{Capacity: capacity, Releases: releases}

	if !a.changed(report) {
		return
	}

	published := report
	published.LastReportTime = metav1.Now()

	if err := a.Publisher.Publish(published); err != nil {
		a.Log.Error(err, "pushing report")

		return
	}

	a.last = &report
	a.lastPublished = time.Now()
}

func (a *Agent) changed(report TargetReport) bool {
	if a.last == nil || !reflect.DeepEqual(report, *a.last) {
		return true
	}

	return time.Since(a.lastPublished) >= a.MaxReportAge
}
