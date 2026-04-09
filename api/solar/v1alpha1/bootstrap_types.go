// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// BootstrapSpec defines the desired state of a Bootstrap.
// It contains the concrete releases, profiles, and deployment configuration for a target environment.
type BootstrapSpec struct {
	// Releases is a map of release names to their corresponding Release object references.
	// Each entry represents a component release that will be deployed to the target.
	Releases map[string]corev1.LocalObjectReference `json:"releases"`
	// Profiles is a map of profile names to their corresponding Profile object references.
	// It points to profiles that match the target, e.g. through the label selector of the Profile
	Profiles map[string]corev1.LocalObjectReference `json:"profiles"`
	// Userdata contains arbitrary custom data or configuration for the target deployment.
	// This allows providing target-specific parameters or settings.
	// +optional
	Userdata runtime.RawExtension `json:"userdata,omitempty"`
}

// BootstrapStatus defines the observed state of a Bootstrap.
type BootstrapStatus struct {
	// Conditions represent the latest available observations of a Bootstrap's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`

	// RenderTaskRef is a reference to the RenderTask responsible for this Bootstrap.
	// +optional
	RenderTaskRef *corev1.ObjectReference `json:"renderTaskRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bootstrap represents the entrypoint for the gitless gitops configuration.
// It resolves the implicit matching of profiles to produce a concrete set of releases and profiles.
type Bootstrap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   BootstrapSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status BootstrapStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BootstrapList contains a list of Bootstrap resources.
type BootstrapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Bootstrap `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (h *Bootstrap) GetSingularName() string {
	return "bootstrap"
}

func (h *Bootstrap) ShortNames() []string {
	return []string{"bs"}
}
