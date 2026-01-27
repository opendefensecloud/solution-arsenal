// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceAccess struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

// ComponentVersionSpec defines the desired state of a ComponentVersion.
type ComponentVersionSpec struct {
	ComponentRef corev1.LocalObjectReference `json:"componentRef"`
	Tag          string                      `json:"tag"`
	Resources    map[string]ResourceAccess   `json:"resources"`
	Helm         ResourceAccess              `json:"helm"`
	KRO          ResourceAccess              `json:"kro"`
}

// ComponentVersionStatus defines the observed state of a ComponentVersion.
type ComponentVersionStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComponentVersion represents an OCM component available in the solution catalog.
type ComponentVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ComponentVersionSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ComponentVersionStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComponentVersionList contains a list of ComponentVersion resources.
type ComponentVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ComponentVersion `json:"items" protobuf:"bytes,2,rep,name=items"`
}
