// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

// Package agent implements the solar-agent poll/report loop that runs on a
// target cluster.
//
// It is a POC for one claim in
// docs/developer-guide/adrs/018-Solar-Agent-Architecture.md: that the Flux
// conditions on the OCIRepository/HelmRelease pair the bootstrap chart creates
// are enough to derive a Release's lifecycle state, without the agent tracking
// any rollout state of its own. Everything else the ADR describes, i.e. the
// TargetReport API type, the OAuth credential, capacity preflight are deliberately
// left out.
package agent

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleaseLabelKey is the label the bootstrap chart sets on every
// OCIRepository/HelmRelease pair it creates (see
// pkg/renderer/template/bootstrap/templates/release.yaml). Use it to select the
// objects; read the name from ReleaseAnnotationKey, since the label value is
// sha-truncated at 63 characters.
const ReleaseLabelKey = "solar.opendefense.cloud/release"

// ReleaseAnnotationKey is the annotation the bootstrap chart sets alongside
// ReleaseLabelKey, carrying the untruncated Release name. Reports key off this
// so long Release names stay joinable to their ReleaseBinding.
const ReleaseAnnotationKey = "solar.opendefense.cloud/release"

// ReleasePhase is the mutually-exclusive lifecycle state of one bound Release.
// The orthogonal states (verified, rolled back, test failed, drifted) are not
// phases -- a Release can be ReleaseReady and drifted at the same time -- and
// are read from the conditions instead.
type ReleasePhase string

const (
	// ReleasePending means no OCIRepository/HelmRelease pair exists yet for a
	// bound Release: the applied bootstrap chart is older than the ReleaseBinding set.
	ReleasePending ReleasePhase = "Pending"
	// ReleaseProgressing means Flux is still working towards the desired state.
	ReleaseProgressing ReleasePhase = "Progressing"
	// ReleaseReady means both halves of the pair report Ready=True.
	ReleaseReady ReleasePhase = "Ready"
	// ReleaseDegraded means not ready, but Flux will retry.
	ReleaseDegraded ReleasePhase = "Degraded"
	// ReleaseFailed means not ready and no retry is coming: Stalled=True on the
	// OCIRepository, or remediation retries exhausted on the HelmRelease.
	ReleaseFailed ReleasePhase = "Failed"
)

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

// ReleaseStatus is one OCIRepository/HelmRelease pair's rolled-up state,
// selected by ReleaseLabelKey and named by ReleaseAnnotationKey.
type ReleaseStatus struct {
	// Name is the untruncated Release name, read from the solar.opendefense.cloud/release
	// annotation. The label of the same name is sha-truncated at 63 characters and would
	// make long names unjoinable to their ReleaseBinding.
	Name string `json:"name"`
	// Namespace is where the pair lives. Name alone is not unique when the agent
	// watches all namespaces, so it is part of the pair's identity, not decoration.
	Namespace string `json:"namespace"`
	// Phase is the mutually-exclusive lifecycle state derived per the table above.
	// +kubebuilder:validation:Enum=Pending;Progressing;Ready;Degraded;Failed
	Phase ReleasePhase `json:"phase"`
	// Revision is the chart version actually live, from HelmRelease
	// status.history[0].chartVersion. After a rollback this lags the desired version,
	// which is the only way to see that from the report.
	Revision string `json:"revision,omitempty"`
	// SourceConditions and HelmConditions are the verbatim Flux conditions of the pair,
	// kept apart so a fetch/verify failure stays distinguishable from an apply failure.
	// The orthogonal states (SourceVerified, Remediated, TestSuccess, Drifted) are read
	// from here rather than duplicated into fields.
	SourceConditions []metav1.Condition `json:"sourceConditions,omitempty"`
	HelmConditions   []metav1.Condition `json:"helmConditions,omitempty"`
}
