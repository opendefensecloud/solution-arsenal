// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TargetSpec defines the desired state of a Component.
type TargetSpec struct {
	Releases map[string]corev1.LocalObjectReference `json:"releases"`
	Userdata runtime.RawExtension                   `json:"userdata,omitempty"`
}

// TargetStatus defines the observed state of a Target.
type TargetStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Target represents an OCM component available in the solution catalog.
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
