// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleaseBindingSpec defines the desired state of a ReleaseBinding.
type ReleaseBindingSpec struct {
	// TargetRef references the Target this release is bound to. When Namespace is set, the
	// Target resides in a different namespace than this ReleaseBinding; cross-namespace
	// references require a ReferenceGrant in the target's namespace that grants access to
	// this ReleaseBinding's namespace.
	TargetRef ObjectReference `json:"targetRef"`
	// ReleaseRef references the Release to deploy.
	ReleaseRef corev1.LocalObjectReference `json:"releaseRef"`
}

// ReleaseBindingStatus defines the observed state of a ReleaseBinding.
type ReleaseBindingStatus struct {
	// Conditions represent the latest available observations of a ReleaseBinding's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`
	// Deployment is what the Target's agent last reported for this binding.
	// solar-controller-manager writes it from the Target's TargetReport.
	// +optional
	Deployment *ReleaseDeployment `json:"deployment,omitempty"`
}

// ReleaseDeployment is the binding's release as deployed on its Target.
type ReleaseDeployment struct {
	// Phase is the last state the agent reported, kept while the agent is silent,
	// so the UI can show "last seen Failed". Empty until a report covers this binding.
	// +optional
	Phase ReleasePhase `json:"phase,omitempty"`
	// LiveRevision is the chart version live on the cluster, as the agent reported it,
	// including the digest Flux appends and without the tag's leading "v", e.g.
	// 0.0.3-ab12+0cc9a8446c95.
	// +optional
	LiveRevision string `json:"liveRevision,omitempty"`
	// DesiredRevision is the rendered chart tag this binding currently asks for.
	// +optional
	DesiredRevision string `json:"desiredRevision,omitempty"`
	// DesiredDigest is the manifest digest of that chart, from its RenderArtifact.
	// Empty until it is known.
	// +optional
	DesiredDigest string `json:"desiredDigest,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReleaseBinding declares that a Release should be deployed to a Target.
type ReleaseBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ReleaseBindingSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ReleaseBindingStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReleaseBindingList contains a list of ReleaseBinding resources.
type ReleaseBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ReleaseBinding `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (r *ReleaseBinding) GetSingularName() string {
	return "releasebinding"
}

func (r *ReleaseBinding) ShortNames() []string {
	return []string{"rlb"}
}
