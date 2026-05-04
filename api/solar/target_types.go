// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TargetSpec defines the desired state of a Target.
// It specifies the render registry and configuration for this deployment target.
type TargetSpec struct {
	// RenderRegistryRef references the Registry to push rendered desired state to.
	// The referenced Registry must have SolarSecretRef set for rendering to succeed.
	RenderRegistryRef corev1.LocalObjectReference `json:"renderRegistryRef"`
	// RenderRegistryNamespace is the namespace of the Registry when it resides in a different
	// namespace than this Target. If empty, the Registry is assumed to be in the same namespace.
	// Cross-namespace references require a ReferenceGrant in the registry's namespace that grants
	// access to this Target's namespace.
	// +optional
	RenderRegistryNamespace string `json:"renderRegistryNamespace,omitempty"`
	// Userdata contains arbitrary custom data or configuration specific to this target.
	// This enables target-specific customization and deployment parameters.
	// +optional
	Userdata runtime.RawExtension `json:"userdata,omitempty"`
}

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
