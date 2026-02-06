// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RenderConfigStatus defines the inputs for the rendering process
type RenderConfigSpec struct {
	Config runtime.RawExtension
}

// RenderConfigStatus holds the status of the rendering process
type RenderConfigStatus struct {
	// Conditions represent the latest available observations of a RenderConfig's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`

	// JobRef is a reference to the Job that is executing the rendering.
	// +optional
	JobRef *corev1.ObjectReference `json:"jobRef,omitempty"`

	// ConfigSecretRef is a reference to the Secret containing the renderer configuration.
	// +optional
	ConfigSecretRef *corev1.ObjectReference `json:"configSecretRef,omitempty"`

	// ChartURL represents the URL of where the rendered chart was pushed to.
	// +optional
	ChartURL string
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RenderConfig manages a rendering job
type RenderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   RenderConfigSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status RenderConfigStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReleaseList contains a list of RenderConfig resources.
type RenderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []RenderConfig `json:"items" protobuf:"bytes,2,rep,name=items"`
}
