// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RenderTaskSpec holds the specification for a RenderTask
type RenderTaskSpec struct {
	// RendererConfig is the config used for the renderer job
	RendererConfig `json:",inline"`

	// Repository is the Repository where the chart will be pushed to (e.g. charts/mychart)
	// Keep in mind that the repository gets automatically prefixed with the
	// registry by the rendertask-controller.
	Repository string `json:"repository"`

	// Tag is the Tag of the helm chart to be pushed.
	// Make sure that the tag matches the version in Chart.yaml, otherwise helm
	// will error before pushing.
	Tag string `json:"tag"`

	// failedJobTTL is the TTL in seconds after which a failed render job and its secrets are cleaned up.
	// After this duration, the Kubernetes TTL controller will delete the Job and the controller will delete
	// the Secrets (ConfigSecret, AuthSecret). On success, Job and Secrets are deleted immediately.
	// If not set, defaults to 3600 (1 hour).
	// +optional
	FailedJobTTL *int32 `json:"failedJobTTL,omitempty"`

	// OwnerName is the name of the resource that created this RenderTask.
	// +kubebuilder:validation:MinLength=1
	OwnerName string `json:"ownerName"`

	// OwnerNamespace is the namespace of the resource that created this RenderTask.
	// +kubebuilder:validation:MinLength=1
	OwnerNamespace string `json:"ownerNamespace"`

	// OwnerKind is the kind of the resource that created this RenderTask (e.g. Release, Bootstrap).
	// +kubebuilder:validation:MinLength=1
	OwnerKind string `json:"ownerKind"`
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
// +genclient:nonNamespaced
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
