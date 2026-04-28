// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReferenceGrantFromSubject identifies the group, kind, and namespace of a resource that
// is permitted to reference resources in the namespace where the ReferenceGrant lives.
type ReferenceGrantFromSubject struct {
	// Group is the API group of the referencing resource.
	// Use "" for the core API group.
	// +kubebuilder:validation:Required
	Group string `json:"group"`

	// Kind is the kind of the referencing resource (e.g. "Profile", "Target").
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Namespace is the namespace of the referencing resource.
	// A single namespace is allowed per From entry to avoid overly broad grants.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// ReferenceGrantToTarget specifies the group and kind of resource that may be referenced.
// Resource names are intentionally excluded: a namespace-scoped grant already limits
// the blast radius, and name restrictions rarely provide meaningful security.
type ReferenceGrantToTarget struct {
	// Group is the API group of the referenced resource.
	// Use "" for the core API group.
	// +kubebuilder:validation:Required
	Group string `json:"group"`

	// Kind is the kind of the referenced resource (e.g. "Target", "Registry").
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// ReferenceGrantSpec defines the desired state of a ReferenceGrant.
type ReferenceGrantSpec struct {
	// From is the list of resources that are permitted to reference resources in this namespace.
	// Each entry specifies the group, kind, and namespace of an allowed referencing resource.
	// +kubebuilder:validation:MinItems=1
	From []ReferenceGrantFromSubject `json:"from"`

	// To is the list of resource types in this namespace that may be referenced from the
	// resources listed in From.
	// +kubebuilder:validation:MinItems=1
	To []ReferenceGrantToTarget `json:"to"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReferenceGrant grants namespaces listed in From permission to reference resource types
// listed in To within the namespace where this ReferenceGrant lives.
//
// This enables cross-namespace use-cases such as a Profile in one namespace matching
// Targets in another namespace
type ReferenceGrant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec ReferenceGrantSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReferenceGrantList contains a list of ReferenceGrant resources.
type ReferenceGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ReferenceGrant `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (r *ReferenceGrant) GetSingularName() string {
	return "referencegrant"
}

func (r *ReferenceGrant) ShortNames() []string {
	return []string{"rg"}
}
