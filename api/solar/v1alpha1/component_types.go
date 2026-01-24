// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentSpec defines the desired state of a Component.
type ComponentSpec struct {
	// Repository is the OCI repository URL where the component is stored.
	Repository string `json:"repository"`

	// Type defines what type of Component this is.
	// TODO enum
	Type string `json:"type"`

	// Provider is the provider or vendor of the catalog item.
	// +optional
	Provider string `json:"provider,omitempty"`
}

// ComponentStatus defines the observed state of a Component.
type ComponentStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Component represents an OCM component available in the solution catalog.
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ComponentSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ComponentStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComponentList contains a list of Component resources.
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Component `json:"items" protobuf:"bytes,2,rep,name=items"`
}
