// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import "github.com/go-logr/logr"

// Publisher delivers a TargetReport somewhere. LogPublisher is the POC
// stand-in for a client that pushes to the TargetReport resource on
// solar-apiserver (proposed in the design doc but not implemented here).
type Publisher interface {
	Publish(r TargetReport) error
}

// LogPublisher logs the report instead of sending it anywhere.
type LogPublisher struct {
	Log logr.Logger
}

func (l LogPublisher) Publish(r TargetReport) error {
	l.Log.Info("target report",
		"nodeCount", r.Capacity.NodeCount,
		"allocatable", r.Capacity.Allocatable,
		"used", r.Capacity.Used,
		"releases", r.Releases,
	)

	return nil
}
