// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TargetSpec defines the desired state of a Target.
// It specifies the render registry and configuration for this deployment target.
type TargetSpec struct {
	// RenderRegistryRef references the Registry to push rendered desired state to.
	// The referenced Registry must have SolarSecretRef set for rendering to succeed.
	// When Namespace is set, the Registry resides in a different namespace than this
	// Target; cross-namespace references require a ReferenceGrant in the Registry's
	// namespace that grants access to this Target's namespace.
	RenderRegistryRef ObjectReference `json:"renderRegistryRef"`
	// Userdata contains arbitrary custom data or configuration specific to this target.
	// This enables target-specific customization and deployment parameters.
	// +optional
	Userdata runtime.RawExtension `json:"userdata,omitempty"`
}

const (
	// ConditionTypeAgentReporting says whether the Target's solar-agent still
	// updates its TargetReport. Set only while the Target has a report.
	ConditionTypeAgentReporting = "AgentReporting"

	// ReasonReporting means the report is within the stale threshold.
	ReasonReporting = "Reporting"
	// ReasonNoReport means the report exists but the agent never wrote it.
	ReasonNoReport = "NoReport"
	// ReasonStale means the report is older than the stale threshold.
	ReasonStale = "Stale"

	// ConditionTypePreflightReady aggregates the preflight checks of the
	// Target's TargetReport. Set only while the Target has a report.
	ConditionTypePreflightReady = "PreflightReady"

	// ReasonChecksErroredAndFailed means at least one check failed and at least
	// one could not be evaluated.
	ReasonChecksErroredAndFailed = "ChecksErroredAndFailed"
	// ReasonChecksFailed means at least one check failed.
	ReasonChecksFailed = "ChecksFailed"
	// ReasonChecksErrored means at least one check could not be evaluated, and
	// none failed.
	ReasonChecksErrored = "ChecksErrored"
	// ReasonNoChecks means the report contains no checks.
	ReasonNoChecks = "NoChecks"
	// ReasonAllChecksPassed means every check passed.
	ReasonAllChecksPassed = "AllChecksPassed"

	// ConditionTypeReleasesDeployed rolls up the Deployed condition of the
	// Target's bindings. Set only while the Target has a report. When False,
	// its reason is the worst binding's Deployed reason.
	ConditionTypeReleasesDeployed = "ReleasesDeployed"

	// ReasonAllReady means every binding that wasn't filtered is Ready.
	ReasonAllReady = "AllReady"
)

// TargetStatus defines the observed state of a Target.
type TargetStatus struct {
	// BootstrapVersion is a monotonically increasing counter used as the bootstrap
	// chart version. It is incremented each time the bootstrap chart is re-rendered,
	// e.g. when the set of bound releases changes.
	// +optional
	BootstrapVersion int64 `json:"bootstrapVersion,omitempty"`

	// Conditions represent the latest available observations of a Target's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`

	// ResolvedBindings records what the release resolver made of each
	// ReleaseBinding that names this Target, including bindings from other
	// namespaces: accepted, filtered, or unresolved. Sorted by namespace and name.
	// +optional
	// +listType=map
	// +listMapKey=namespace
	// +listMapKey=name
	ResolvedBindings []ResolvedBinding `json:"resolvedBindings,omitempty"`
}

// ResolvedBinding is the release resolver's outcome for one ReleaseBinding.
type ResolvedBinding struct {
	// Namespace and Name identify the ReleaseBinding.
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// UniqueName is the key of this release in the bootstrap chart, and therefore
	// the name the agent reports it under. Empty if the binding's Release or
	// ComponentVersion can't be resolved.
	// +optional
	UniqueName string `json:"uniqueName,omitempty"`
	// DesiredRevision is the chart tag this binding asks for, known before the
	// chart is rendered. Empty if it can't be computed.
	// +optional
	DesiredRevision string `json:"desiredRevision,omitempty"`
	// RenderArtifact is the name of the RenderArtifact for that chart, in the
	// Target's namespace. It may not exist until the chart is rendered.
	// +optional
	RenderArtifact string `json:"renderArtifact,omitempty"`
	// Filtered is true if the resolver dropped this binding.
	// +optional
	Filtered bool `json:"filtered,omitempty"`
	// Message explains a filtered or unresolved binding.
	// +optional
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Target represents a deployment target environment.
// It defines the intended state of releases and configuration for a specific deployment target,
// such as a cluster or environment.
type Target struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   TargetSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status TargetStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TargetList contains a list of Target resources.
type TargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Target `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (t *Target) GetSingularName() string {
	return "target"
}

func (t *Target) ShortNames() []string {
	return []string{"tgt"}
}
