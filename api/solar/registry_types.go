// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RegistrySpec defines the desired state of a Registry.
type RegistrySpec struct {
	// Hostname is the registry endpoint (e.g. "registry.example.com:5000").
	Hostname string `json:"hostname"`
	// PlainHTTP uses HTTP instead of HTTPS for connections to this registry.
	// +optional
	PlainHTTP bool `json:"plainHTTP,omitempty"`
	// SolarSecretRef references a Secret in the same namespace with credentials
	// to access this registry from the SolAr cluster. Required if this registry
	// is used as a render target.
	// +optional
	SolarSecretRef *corev1.LocalObjectReference `json:"solarSecretRef,omitempty"`
	// TargetSecretRef describes where the credentials secret lives in the target cluster.
	// Used by the target agent for pull access.
	// +optional
	TargetSecretRef *TargetSecretReference `json:"targetSecretRef,omitempty"`
}

// TargetSecretReference is a reference to a Secret in a target cluster.
type TargetSecretReference struct {
	// Name is the name of the Secret.
	Name string `json:"name"`
	// Namespace is the namespace of the Secret.
	Namespace string `json:"namespace"`
}

// RegistryStatus defines the observed state of a Registry.
type RegistryStatus struct {
	// Conditions represent the latest available observations of a Registry's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Registry represents an OCI registry that can be used as a source or destination for artifacts.
type Registry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   RegistrySpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status RegistryStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RegistryList contains a list of Registry resources.
type RegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Registry `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (r *Registry) GetSingularName() string {
	return "registry"
}

func (r *Registry) ShortNames() []string {
	return []string{"reg"}
}
