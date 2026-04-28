// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	EntrypointTypeKRO  EntrypointType = "kro"
	EntrypointTypeHelm EntrypointType = "helm"
)

// ResourceAccess defines how a Resource can be accessed along with optional metadata.
type ResourceAccess struct {
	// Repository of the Resource.
	Repository string `json:"repository"`
	// Insecure switches TLS/HTTPS off if true
	Insecure bool `json:"insecure"`
	// Tag of the Resource.
	Tag string `json:"tag"`
	// Helm contains metadata for Helm chart resources, populated during discovery.
	Helm *HelmResourceMetadata `json:"helm,omitempty"`
}

// HelmResourceMetadata contains metadata extracted from a Helm chart resource during discovery.
type HelmResourceMetadata struct {
	// Name of the Helm chart.
	Name string `json:"name"`
	// Description of the Helm chart.
	Description string `json:"description,omitempty"`
	// Version of the Helm chart.
	Version string `json:"version"`
	// AppVersion of the application deployed by the chart.
	AppVersion string `json:"appVersion,omitempty"`
	// ValuesTemplate contains the rendered helm values template, if present in the OCM package.
	ValuesTemplate *string `json:"valuesTemplate,omitempty"`
}

// EntrypointType is the Type of Entrypoint.
// +enum
type EntrypointType string

// Entrypoint defines the entrypoint for deploying a ComponentVersion.
type Entrypoint struct {
	// ResourceName is the Name of the Resource to use as the entrypoint.
	ResourceName string `json:"resourceName"`
	// Type of entrypoint.
	Type EntrypointType `json:"type"`
}

// ComponentVersionSpec defines the desired state of a ComponentVersion.
type ComponentVersionSpec struct {
	// ComponentRef is a reference to the parent Component.
	ComponentRef corev1.LocalObjectReference `json:"componentRef"`
	// Tag is a version of the component.
	Tag string `json:"tag"`
	// Resources are Resources that are within the ComponentVersion.
	Resources map[string]ResourceAccess `json:"resources"`
	// Entrypoint is the entrypoint for deploying a ComponentVersion.
	Entrypoint Entrypoint `json:"entrypoint"`
}

// ComponentVersionStatus defines the observed state of a ComponentVersion.
type ComponentVersionStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComponentVersion represents an OCM component available in the solution catalog.
type ComponentVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ComponentVersionSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ComponentVersionStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComponentVersionList contains a list of ComponentVersion resources.
type ComponentVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ComponentVersion `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (c *ComponentVersion) GetSingularName() string {
	return "componentversion"
}

func (c *ComponentVersion) ShortNames() []string {
	return []string{"cv"}
}
