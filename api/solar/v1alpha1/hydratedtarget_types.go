// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// HydratedTargetSpec defines the desired state of a Component.
type HydratedTargetSpec struct {
	Releases map[string]corev1.LocalObjectReference `json:"releases"`
	Userdata runtime.RawExtension                   `json:"userdata,omitempty"`
}

// HydratedTargetStatus defines the observed state of a HydratedTarget.
type HydratedTargetStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HydratedTarget represents an OCM component available in the solution catalog.
type HydratedTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   HydratedTargetSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status HydratedTargetStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HydratedTargetList contains a list of HydratedTarget resources.
type HydratedTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []HydratedTarget `json:"items" protobuf:"bytes,2,rep,name=items"`
}
