// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RenderTaskSpec holds the specification for a RenderTask
type RenderTaskSpec struct {
	// RendererConfig is the config used for the renderer job
	RendererConfig `json:",inline"`
	// ReferenceURL is the OCI registry URL where the chart will be pushed (e.g., oci://registry.example.com/charts/mychart:v0.1.0)
	// Make sure that the tag matches the version in Chart.yaml, otherwise helm will error before pushing.
	ReferenceURL string `json:"referenceURL,omitempty"`
	// SecretRef specifies the secret containing the relevant credentials for the OCI registry where rendered charts get pushed to.
	// Secret type is used to decide which authentication method to use. Supported secret types are:
	// - kubernetes.io/dockerconfigjson
	// - kubernetes.io/basic-auth
	// +optional
	SecretRef corev1.LocalObjectReference `json:"secretRef"`
}

// RenderTaskStatus holds the status of the rendering process
type RenderTaskStatus struct {
	// Conditions represent the latest available observations of a RenderTask's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`

	// JobRef is a reference to the Job that is executing the rendering.
	// +optional
	JobRef *corev1.ObjectReference `json:"jobRef,omitempty"`

	// ConfigSecretRef is a reference to the Secret containing the renderer configuration.
	// +optional
	ConfigSecretRef *corev1.ObjectReference `json:"configSecretRef,omitempty"`

	// ChartURL represents the URL of where the rendered chart was pushed to.
	// +optional
	ChartURL string `json:"chartURL"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RenderTask manages a rendering job
type RenderTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   RenderTaskSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status RenderTaskStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReleaseList contains a list of RenderTask resources.
type RenderTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []RenderTask `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (r *RenderTask) GetSingularName() string {
	return "rendertask"
}

func (r *RenderTask) ShortNames() []string {
	return []string{"rt"}
}
