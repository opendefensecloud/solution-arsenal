// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ReleaseSpec defines the desired state of a Release.
// It specifies which component version to release and its deployment configuration.
type ReleaseSpec struct {
	// ComponentVersionRef is a reference to the ComponentVersion to be released.
	// It points to the specific version of a component that this release is based on.
	ComponentVersionRef corev1.LocalObjectReference `json:"componentRef"`
	// Values contains deployment-specific values or configuration for the release.
	// These values override defaults from the component version and are used during deployment.
	// +optional
	Values runtime.RawExtension `json:"values,omitempty"`
}

// ReleaseStatus defines the observed state of a Release.
type ReleaseStatus struct {
	// Conditions represent the latest available observations of a Release's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`

	// RenderTaskRef is a reference to the RenderTask responsible for this Release.
	// +optional
	RenderTaskRef *corev1.ObjectReference `json:"renderTaskRef,omitempty"`

	// ChartURL represents the URL of where the rendered chart was pushed to.
	// +optional
	ChartURL string
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Release represents a specific deployment instance of a component.
// It combines a component version with deployment values and configuration for a particular use case.
type Release struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ReleaseSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ReleaseStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReleaseList contains a list of Release resources.
type ReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Release `json:"items" protobuf:"bytes,2,rep,name=items"`
}
