// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ProfileSpec defines the desired state of a Profile.
// It points to a Release and defines target selection criteria for
// Targets this Release is intended to be deployed to.
type ProfileSpec struct {
	// ReleaseRef is a reference to a Release.
	// It points to the Release that is intended to be deployed to all Targets identified
	// by the TargetSelector.
	// +kubebuilder:validation:Required
	ReleaseRef corev1.LocalObjectReference `json:"releaseRef"`

	// TargetSelector is a label-based filter to identify the Targets this Release is
	// intended to be deployed to.
	// +optional
	TargetSelector metav1.LabelSelector `json:"targetSelector,omitempty"`

	// Userdata contains arbitrary custom data or configuration which is passed to all
	// Targets associated with this Profile.
	// +optional
	Userdata runtime.RawExtension `json:"userdata,omitempty"`
}

// ProfileStatus defines the observed state of a Profile.
type ProfileStatus struct {
	// MatchedTargets is the total number of Targets matching the target selection criteria.
	// +optional
	MatchedTargets int `json:"matchedTargets,omitempty"`

	// Conditions represent the latest available observations of the Profile's state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

// Profile represents the link between a Release and a set of matching Targets the Release is
// intended to be deployed to.
type Profile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ProfileSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ProfileStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProfileList contains a list of Profile resources.
type ProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Profile `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (c *Profile) GetSingularName() string {
	return "profile"
}

func (c *Profile) ShortNames() []string {
	return []string{"prf"}
}
