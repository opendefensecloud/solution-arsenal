// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleaseBindingSpec defines the desired state of a ReleaseBinding.
type ReleaseBindingSpec struct {
	// TargetRef references the Target this release is bound to.
	TargetRef corev1.LocalObjectReference `json:"targetRef"`
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
