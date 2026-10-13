// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

// Package agent implements the solar-agent poll/report loop that runs on a
// target cluster.
//
// It reports the real solarv1alpha1.TargetReport: the Flux conditions on the
// OCIRepository/HelmRelease pair the bootstrap chart creates are enough to
// derive each Release's lifecycle state, without the agent tracking any rollout
// state of its own. The OAuth credential is still out of scope, and the
// capacity collection is kept but no longer wired into the report.
package agent

import (
	corev1 "k8s.io/api/core/v1"
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

// ClusterCapacity summarizes target-cluster node capacity and requested use.
// Kept: it is not part of the TargetReport API type, so the loop
// no longer collects it.
type ClusterCapacity struct {
	NodeCount   int32               `json:"nodeCount"`
	Allocatable corev1.ResourceList `json:"allocatable"`
	Used        corev1.ResourceList `json:"used"`
}
