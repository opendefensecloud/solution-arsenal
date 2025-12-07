// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CatalogItemCategory defines the type of catalog item.
// +kubebuilder:validation:Enum=Application;Operator;Addon;Library
type CatalogItemCategory string

const (
	// CatalogItemCategoryApplication represents a deployable application.
	CatalogItemCategoryApplication CatalogItemCategory = "Application"
	// CatalogItemCategoryOperator represents a Kubernetes operator.
	CatalogItemCategoryOperator CatalogItemCategory = "Operator"
	// CatalogItemCategoryAddon represents a cluster addon or extension.
	CatalogItemCategoryAddon CatalogItemCategory = "Addon"
	// CatalogItemCategoryLibrary represents a shared library or dependency.
	CatalogItemCategoryLibrary CatalogItemCategory = "Library"
)

// CatalogItemPhase describes the lifecycle phase of a CatalogItem.
// +kubebuilder:validation:Enum=Discovered;Validating;Available;Failed;Deprecated
type CatalogItemPhase string

const (
	// CatalogItemPhaseDiscovered indicates the item was found by discovery but not yet validated.
	CatalogItemPhaseDiscovered CatalogItemPhase = "Discovered"
	// CatalogItemPhaseValidating indicates the item is being validated.
	CatalogItemPhaseValidating CatalogItemPhase = "Validating"
	// CatalogItemPhaseAvailable indicates the item is validated and available for deployment.
	CatalogItemPhaseAvailable CatalogItemPhase = "Available"
	// CatalogItemPhaseFailed indicates validation or processing has failed.
	CatalogItemPhaseFailed CatalogItemPhase = "Failed"
	// CatalogItemPhaseDeprecated indicates the item is deprecated and should not be used for new deployments.
	CatalogItemPhaseDeprecated CatalogItemPhase = "Deprecated"
)

// Maintainer contains information about the maintainer of a catalog item.
type Maintainer struct {
	// Name is the maintainer's name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Email is the maintainer's email address.
	// +optional
	Email string `json:"email,omitempty"`
}

// ComponentSource describes where the OCM component is stored.
type ComponentSource struct {
	// Registry is the OCI registry URL (e.g., "ghcr.io", "registry.example.com").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Registry string `json:"registry"`
	// Path is the path within the registry to the component.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`
}

// Attestation represents a security attestation for the component.
type Attestation struct {
	// Type is the attestation type (e.g., "vulnerability-scan", "stig-compliance", "signature").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Type string `json:"type"`
	// Issuer identifies who created the attestation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Issuer string `json:"issuer"`
	// Reference is a URL or identifier pointing to the attestation data.
	// +optional
	Reference string `json:"reference,omitempty"`
	// Passed indicates whether the attestation check passed.
	Passed bool `json:"passed"`
	// Timestamp is when the attestation was created.
	// +optional
	Timestamp *metav1.Time `json:"timestamp,omitempty"`
}

// ComponentDependency describes a dependency on another OCM component.
type ComponentDependency struct {
	// ComponentName is the OCM component name of the dependency.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ComponentName string `json:"componentName"`
	// VersionConstraint is a semver constraint for acceptable versions (e.g., ">=1.0.0", "^2.0.0").
	// +optional
	VersionConstraint string `json:"versionConstraint,omitempty"`
	// Optional indicates whether this dependency is optional.
	// +optional
	Optional bool `json:"optional,omitempty"`
}

// ResourceRequirements describes the resource requirements for deploying the catalog item.
type ResourceRequirements struct {
	// CPUCores is the estimated CPU cores required.
	// +optional
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?$`
	CPUCores string `json:"cpuCores,omitempty"`
	// MemoryMB is the estimated memory in megabytes required.
	// +optional
	// +kubebuilder:validation:Pattern=`^[0-9]+$`
	MemoryMB string `json:"memoryMB,omitempty"`
	// StorageGB is the estimated storage in gigabytes required.
	// +optional
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?$`
	StorageGB string `json:"storageGB,omitempty"`
}

// ValidationCheck represents the result of a single validation check.
type ValidationCheck struct {
	// Name is the name of the validation check.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Passed indicates whether the check passed.
	Passed bool `json:"passed"`
	// Message provides additional information about the check result.
	// +optional
	Message string `json:"message,omitempty"`
	// Timestamp is when the check was performed.
	// +optional
	Timestamp *metav1.Time `json:"timestamp,omitempty"`
}

// ValidationStatus contains the results of all validation checks.
type ValidationStatus struct {
	// Checks contains the results of individual validation checks.
	// +optional
	Checks []ValidationCheck `json:"checks,omitempty"`
	// LastValidatedAt is when validation was last performed.
	// +optional
	LastValidatedAt *metav1.Time `json:"lastValidatedAt,omitempty"`
}

// CatalogItemSpec defines the desired state of a CatalogItem.
type CatalogItemSpec struct {
	// ComponentName is the OCM component name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ComponentName string `json:"componentName"`
	// Version is the semantic version of the component.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?(\+[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$`
	Version string `json:"version"`
	// Repository is the OCI repository URL where the component is stored.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Repository string `json:"repository"`
	// Description provides a human-readable description of the catalog item.
	// +optional
	// +kubebuilder:validation:MaxLength=4096
	Description string `json:"description,omitempty"`

	// Category classifies the type of catalog item.
	// +optional
	Category CatalogItemCategory `json:"category,omitempty"`
	// Maintainers lists the maintainers of this catalog item.
	// +optional
	Maintainers []Maintainer `json:"maintainers,omitempty"`
	// Tags are labels for searching and filtering catalog items.
	// +optional
	// +kubebuilder:validation:MaxItems=20
	Tags []string `json:"tags,omitempty"`
	// Source describes where the OCM component is stored.
	// +optional
	Source *ComponentSource `json:"source,omitempty"`
	// RequiredAttestations lists the attestation types required before deployment.
	// +optional
	RequiredAttestations []string `json:"requiredAttestations,omitempty"`
	// Dependencies lists other OCM components this item depends on.
	// +optional
	Dependencies []ComponentDependency `json:"dependencies,omitempty"`
	// MinKubernetesVersion is the minimum Kubernetes version required.
	// +optional
	// +kubebuilder:validation:Pattern=`^v?[0-9]+\.[0-9]+(\.[0-9]+)?$`
	MinKubernetesVersion string `json:"minKubernetesVersion,omitempty"`
	// RequiredCapabilities lists Kubernetes features or CRDs required (e.g., "networking.k8s.io/v1/NetworkPolicy").
	// +optional
	RequiredCapabilities []string `json:"requiredCapabilities,omitempty"`
	// EstimatedResources provides resource estimates for capacity planning.
	// +optional
	EstimatedResources *ResourceRequirements `json:"estimatedResources,omitempty"`
	// Deprecated marks this catalog item as deprecated.
	// +optional
	Deprecated bool `json:"deprecated,omitempty"`
	// DeprecationMessage provides information about why this item is deprecated and what to use instead.
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	DeprecationMessage string `json:"deprecationMessage,omitempty"`
}

// CatalogItemStatus defines the observed state of a CatalogItem.
type CatalogItemStatus struct {
	// Phase is the current lifecycle phase of the catalog item.
	// +optional
	Phase CatalogItemPhase `json:"phase,omitempty"`
	// Validation contains the results of validation checks.
	// +optional
	Validation *ValidationStatus `json:"validation,omitempty"`
	// Attestations contains the attestations found for this component.
	// +optional
	Attestations []Attestation `json:"attestations,omitempty"`
	// LastDiscoveredAt is when this item was last seen by the discovery service.
	// +optional
	LastDiscoveredAt *metav1.Time `json:"lastDiscoveredAt,omitempty"`
	// Conditions represent the latest available observations of the catalog item's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Component",type=string,JSONPath=`.spec.componentName`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Category",type=string,JSONPath=`.spec.category`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CatalogItem represents an OCM component available in the solution catalog.
type CatalogItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   CatalogItemSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status CatalogItemStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CatalogItemList contains a list of CatalogItem resources.
type CatalogItemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []CatalogItem `json:"items" protobuf:"bytes,2,rep,name=items"`
}
