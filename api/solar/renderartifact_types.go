// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RenderArtifactSpec holds the OCI coordinates of a successfully pushed artifact.
type RenderArtifactSpec struct {
	// BaseURL is the registry's base URL (e.g. "registry.example.com:5000").
	// +kubebuilder:validation:MinLength=1
	BaseURL string `json:"baseURL"`
	// Repository is the repository path within the registry (e.g. "mynamespace/release-myapp").
	// +kubebuilder:validation:MinLength=1
	Repository string `json:"repository"`
	// Tag is the OCI tag that was pushed (e.g. "v0.0.3").
	// +kubebuilder:validation:MinLength=1
	Tag string `json:"tag"`
	// RenderTaskRef is the name of the RenderTask that produced this artifact.
	RenderTaskRef string `json:"renderTaskRef"`
	// PushSecretRef references a Secret containing registry credentials used to push this
	// artifact. Used for tag deletion during GC.
	// +optional
	PushSecretRef *corev1.LocalObjectReference `json:"pushSecretRef,omitempty"`
	// PushSecretNamespace is the namespace of the Secret referenced by PushSecretRef.
	// When empty, defaults to the RenderArtifact's own namespace.
	// Set when the Registry lives in a different namespace from the Target (cross-namespace).
	// +optional
	PushSecretNamespace string `json:"pushSecretNamespace,omitempty"`
	// RegistryFlavor identifies the registry implementation (e.g. "zot", "harbor").
	// +optional
	RegistryFlavor string `json:"registryFlavor,omitempty"`
	// PlainHTTP uses HTTP instead of HTTPS for OCI registry connections.
	// +optional
	PlainHTTP bool `json:"plainHTTP,omitempty"`
}

// RenderArtifactStatus holds the observed state of a RenderArtifact.
type RenderArtifactStatus struct {
	// ChartURL is the fully-qualified OCI reference for this artifact (e.g. "oci://registry.example.com/ns/release-app:v0.0.3").
	// +optional
	ChartURL string `json:"chartURL,omitempty"`
	// Conditions represent the latest available observations of a RenderArtifact's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RenderArtifact represents a successfully pushed OCI artifact produced by a RenderTask.
// It tracks the artifact's push coordinates and is ref-counted via RenderBindings.
// When the last RenderBinding referencing it is removed, the GC controller attempts to
// delete the OCI tag (best-effort) and removes this object.
type RenderArtifact struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   RenderArtifactSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status RenderArtifactStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RenderArtifactList contains a list of RenderArtifact resources.
type RenderArtifactList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []RenderArtifact `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (r *RenderArtifact) GetSingularName() string {
	return "renderartifact"
}

func (r *RenderArtifact) ShortNames() []string {
	return []string{"ra"}
}
