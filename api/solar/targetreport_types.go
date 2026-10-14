// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TargetReportSpec is one target cluster's state as observed by its agent.
// It is a full snapshot, never append-only: written exclusively by the agent,
// read exclusively by the targetreport controller.
type TargetReportSpec struct {
	// LastReportTime is set by the agent at publish time. The agent owns it, and
	// it must advance on every update.
	LastReportTime metav1.Time `json:"lastReportTime"`
	// Releases are the OCIRepository/HelmRelease pairs the agent observed:
	// a snapshot of what is currently deployed. Observational only — it never
	// gates anything.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +listMapKey=namespace
	Releases []ReleaseReport `json:"releases,omitempty"`
	// Preflight holds the agent's health-check results that gate new releases:
	// failed checks block deploying to this target. Unlike Releases, it says
	// nothing about what is currently deployed.
	Preflight PreflightReport `json:"preflight"`
}

// ReleasePhase is the mutually-exclusive lifecycle state of one bound Release.
// +kubebuilder:validation:Enum=Pending;Progressing;Ready;Degraded;Failed
type ReleasePhase string

const (
	// ReleasePending means no OCIRepository/HelmRelease pair exists yet for the Release.
	ReleasePending ReleasePhase = "Pending"
	// ReleaseProgressing means Flux is still working towards the desired state.
	ReleaseProgressing ReleasePhase = "Progressing"
	// ReleaseReady means both halves of the pair report Ready=True.
	ReleaseReady ReleasePhase = "Ready"
	// ReleaseDegraded means not ready, but a retry is still coming.
	ReleaseDegraded ReleasePhase = "Degraded"
	// ReleaseFailed means not ready and no retry is coming.
	ReleaseFailed ReleasePhase = "Failed"
)

// ReleaseReport is one bound Release's rolled-up state as reported by the agent.
type ReleaseReport struct {
	// Name is the untruncated Release name.
	Name string `json:"name"`
	// Namespace is where the OCIRepository/HelmRelease pair lives.
	Namespace string `json:"namespace"`
	// Phase is the mutually-exclusive lifecycle state.
	Phase ReleasePhase `json:"phase"`
	// Revision is the chart version actually live, if known.
	// +optional
	Revision string `json:"revision,omitempty"`
	// SourceConditions and HelmConditions are the verbatim Flux conditions of
	// the pair, kept apart so a fetch/verify failure stays distinguishable
	// from an apply failure.
	// +optional
	// +listType=map
	// +listMapKey=type
	SourceConditions []metav1.Condition `json:"sourceConditions,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=type
	HelmConditions []metav1.Condition `json:"helmConditions,omitempty"`
}

// PreflightReport holds the agent's health-check results.
type PreflightReport struct {
	// Checks are sorted by Type so equal reports compare equal.
	// +optional
	// +listType=map
	// +listMapKey=type
	Checks []PreflightCheck `json:"checks,omitempty"`
}

// PreflightCheck is one health-check result. It is a metav1.Condition minus
// ObservedGeneration and LastTransitionTime: a pushed snapshot knows neither.
type PreflightCheck struct {
	// Type is a stable PascalCase identifier (e.g. FluxCrds). Checks stay in the
	// report; every check feeds the Target's PreflightReady aggregate.
	Type string `json:"type"`
	// Status maps onto metav1.Condition: Unknown means the check could not be
	// evaluated, distinct from a check that ran and failed.
	Status metav1.ConditionStatus `json:"status"`
	// Reason is a PascalCase machine reason; always set.
	Reason string `json:"reason"`
	// Message is the human-readable detail.
	// +optional
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TargetReport is one target cluster's state as observed by its solar-agent.
type TargetReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec TargetReportSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TargetReportList contains a list of TargetReport resources.
type TargetReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []TargetReport `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (t *TargetReport) GetSingularName() string {
	return "targetreport"
}

func (t *TargetReport) ShortNames() []string {
	return []string{"trp"}
}
