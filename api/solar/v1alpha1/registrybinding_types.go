// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RegistryBindingRewrite describes how to rewrite OCI repository references
// when resources are fetched on the target cluster.
type RegistryBindingRewrite struct {
	// SourceEndpoint is the original registry host to match (e.g. "ghcr.io").
	// +optional
	SourceEndpoint string `json:"sourceEndpoint,omitempty"`
	// RepositoryPrefix is prepended to the repository path after rewriting.
	// +optional
	RepositoryPrefix string `json:"repositoryPrefix,omitempty"`
}

// RegistryBindingSpec defines the desired state of a RegistryBinding.
type RegistryBindingSpec struct {
	// TargetRef references the Target this binding applies to.
	TargetRef corev1.LocalObjectReference `json:"targetRef"`
	// RegistryRef references the Registry being bound.
	RegistryRef corev1.LocalObjectReference `json:"registryRef"`
	// Rewrite optionally describes how to rewrite OCI references
	// for this target/registry pair.
	// +optional
	Rewrite *RegistryBindingRewrite `json:"rewrite,omitempty"`
}

// RegistryBindingStatus defines the observed state of a RegistryBinding.
type RegistryBindingStatus struct {
	// Conditions represent the latest available observations of a RegistryBinding's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RegistryBinding declares that a specific Target is allowed to use a specific Registry.
type RegistryBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   RegistryBindingSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status RegistryBindingStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RegistryBindingList contains a list of RegistryBinding resources.
type RegistryBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []RegistryBinding `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (r *RegistryBinding) GetSingularName() string {
	return "registrybinding"
}

func (r *RegistryBinding) ShortNames() []string {
	return []string{"rb"}
}
