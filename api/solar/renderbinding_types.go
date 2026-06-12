// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RenderBindingSpec links a consumer resource to a RenderArtifact for ref-counting.
type RenderBindingSpec struct {
	// RenderArtifactRef is the name of the RenderArtifact in the same namespace.
	RenderArtifactRef corev1.LocalObjectReference `json:"renderArtifactRef"`
	// OwnerKind is the kind of the consuming resource (e.g. "Target").
	// +kubebuilder:validation:MinLength=1
	OwnerKind string `json:"ownerKind"`
	// OwnerName is the name of the consuming resource.
	// +kubebuilder:validation:MinLength=1
	OwnerName string `json:"ownerName"`
	// OwnerNamespace is the namespace of the consuming resource.
	// +kubebuilder:validation:MinLength=1
	OwnerNamespace string `json:"ownerNamespace"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RenderBinding declares that a consumer resource (e.g. a Target) is using a RenderArtifact.
// RenderBindings are the ref-count mechanism for RenderArtifacts: when the last RenderBinding
// referencing a RenderArtifact is removed, the GC controller cleans up the OCI artifact
// and deletes the RenderArtifact object.
type RenderBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec RenderBindingSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RenderBindingList contains a list of RenderBinding resources.
type RenderBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []RenderBinding `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (r *RenderBinding) GetSingularName() string {
	return "renderbinding"
}

func (r *RenderBinding) ShortNames() []string {
	return []string{"rbin"}
}
