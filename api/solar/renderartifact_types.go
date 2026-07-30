// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
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
	// RegistryRef references the Registry that owns the credentials used to push (and
	// later delete) this artifact's OCI tag. When Namespace is empty, the Registry is
	// resolved in the RenderArtifact's own namespace; a non-empty Namespace identifies a
	// different namespace and requires a ReferenceGrant there permitting access, mirroring
	// how Target resolves its RenderRegistryRef. That grant is the Target's grant: it must
	// list from[].kind "Target" (not "RenderArtifact") with the RenderArtifact's namespace
	// and to[].kind "Registry". This field is controller-owned — it is copied from a
	// RenderBinding that the Target controller populated from Target.Spec.RenderRegistryRef
	// — so cleanup never needs a grant the Target itself did not already require.
	// RenderArtifact never stores Secret- or
	// PlainHTTP-identifying information directly: both are read live from the referenced
	// Registry whenever credentials are needed, so a Registry's credentials or transport
	// settings can change without ever going stale on the artifact.
	// +optional
	RegistryRef *ObjectReference `json:"registryRef,omitempty"`
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
