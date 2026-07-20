// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Agent runs the collect -> report loop on a fixed interval
type Agent struct {
	Collector *Collector
	Publisher Publisher
	Interval  time.Duration
	Log       logr.Logger
}

func (a *Agent) Run(ctx context.Context) {
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

	report := TargetReport{
		LastReportTime: metav1.Now(),
		Capacity:       capacity,
		Releases:       releases,
	}

	if err := a.Publisher.Publish(report); err != nil {
		a.Log.Error(err, "pushing report")
	}
}
