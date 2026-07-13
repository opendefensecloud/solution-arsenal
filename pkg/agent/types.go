// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

// Package agent implements the solar-agent poll/report loop that runs on a
// registered target cluster. This is a POC: it proves the collect -> report
// shape described in docs/superpowers/specs/2026-07-07-solar-agent-design.md
// against real local-cluster data. Preflight, Helm/Flux installs and the
// TargetReport API type are intentionally out of scope -- see README in that
// spec for the full design.
package agent

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleaseLabelKey is the label the bootstrap chart sets on every
// OCIRepository/HelmRelease pair it creates (see
// pkg/renderer/template/bootstrap/templates/release.yaml).
const ReleaseLabelKey = "solar.opendefense.cloud/release"

// TargetReport is a POC stand-in for the TargetReportStatus API type
// proposed in the design doc. Shape mirrors it so swapping the log-based
// Publisher for a real apiserver client later is a straight field-for-field
// move.
type TargetReport struct {
	LastReportTime metav1.Time     `json:"lastReportTime"`
	Capacity       ClusterCapacity `json:"capacity"`
	Releases       []ReleaseStatus `json:"releases"`
}

// ClusterCapacity summarizes target-cluster node capacity and requested use.
type ClusterCapacity struct {
	NodeCount   int32               `json:"nodeCount"`
	Allocatable corev1.ResourceList `json:"allocatable"`
	Used        corev1.ResourceList `json:"used"`
}

// ReleaseStatus is one OCIRepository/HelmRelease pair's rolled-up Ready
// condition, keyed by ReleaseLabelKey.
type ReleaseStatus struct {
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}
