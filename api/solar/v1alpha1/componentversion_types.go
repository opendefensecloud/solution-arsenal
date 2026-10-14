// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	EntrypointTypeKRO  EntrypointType = "kro"
	EntrypointTypeHelm EntrypointType = "helm"
)

const (
	// AvailabilityRecheckAnnotation is set by discovery when the registry reports a
	// deletion, so the controller re-checks availability without waiting for the
	// next interval. Its value is a timestamp and carries no meaning beyond change.
	AvailabilityRecheckAnnotation = "solar.opendefense.cloud/availability-recheck"

	// ConditionTypeAvailable reports whether the backing descriptor manifest still resolves.
	ConditionTypeAvailable = "Available"

	ReasonArtifactAvailable      = "ArtifactAvailable"
	ReasonArtifactMissingPending = "ArtifactMissingPending"
	ReasonArtifactMissing        = "ArtifactMissing"
	ReasonRegistryUnreachable    = "RegistryUnreachable"
	ReasonNoDigest               = "NoDigest"
	ReasonRegistryNotConfigured  = "RegistryNotConfigured"
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
	// ComponentName is the OCM component name, e.g. "opendefense.cloud/arc".
	ComponentName string `json:"componentName"`
	// Scheme is the scheme to access the component.
	Scheme string `json:"scheme"`
	// Registry is the registry where the component is stored.
	Registry string `json:"registry"`
	// Repository is the repository where the component is stored.
	Repository string `json:"repository"`
	// Tag is a version of the component.
	Tag string `json:"tag"`
	// Digest is the full manifest digest ("sha256:<hex>") of the OCM component
	// descriptor this ComponentVersion was discovered from. The controller
	// re-checks it for availability, and registry delete events are matched on it.
	// +optional
	Digest string `json:"digest,omitempty"`
	// Resources are Resources that are within the ComponentVersion.
	Resources map[string]ResourceAccess `json:"resources"`
	// Entrypoint is the entrypoint for deploying a ComponentVersion.
	Entrypoint Entrypoint `json:"entrypoint"`
}

// ComponentVersionStatus defines the observed state of a ComponentVersion.
type ComponentVersionStatus struct {
	// Conditions represent the latest available observations of a ComponentVersion's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`
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

// OCMRef returns the OCM reference of this component version in the form
// "<scheme>://<registry>/<namespace>//<componentName>:<tag>". It returns an
// empty string if the component name, scheme or registry is not set, or if the
// repository does not end with the component name.
func (cv *ComponentVersion) OCMRef() string {
	if cv.Spec.ComponentName == "" || cv.Spec.Scheme == "" || cv.Spec.Registry == "" {
		return ""
	}

	repo, name := cv.Spec.Repository, cv.Spec.ComponentName
	if repo != name && !strings.HasSuffix(repo, "/"+name) {
		return ""
	}
	namespace := strings.Trim(strings.TrimSuffix(repo, name), "/")

	return fmt.Sprintf("%s://%s/%s//%s:%s", cv.Spec.Scheme, cv.Spec.Registry, namespace, cv.Spec.ComponentName, cv.Spec.Tag)
}

// DescriptorRepository returns the OCI repository holding this component
// version's descriptor, "<namespace>/component-descriptors/<componentName>",
// or "" when spec.repository does not end with the component name.
func (cv *ComponentVersion) DescriptorRepository() string {
	repo, name := cv.Spec.Repository, cv.Spec.ComponentName
	if name == "" || (repo != name && !strings.HasSuffix(repo, "/"+name)) {
		return ""
	}
	namespace := strings.Trim(strings.TrimSuffix(repo, name), "/")
	if namespace == "" {
		return "component-descriptors/" + name
	}

	return namespace + "/component-descriptors/" + name
}
