// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CatalogItemVersionSpec struct {
	// Version is the semantic version of the component.
	Version string `json:"version"`
	// Digest is the OCI digest of the component version.
	Digest string `json:"digest"`
}

// CatalogItemSpec defines the desired state of a CatalogItem.
type CatalogItemSpec struct {
	// Repository is the OCI repository URL where the component is stored.
	Repository string `json:"repository"`

	// Versions lists the available versions of this component.
	Versions []CatalogItemVersionSpec `json:"versions"`

	// Provider is the provider or vendor of the catalog item.
	Provider string `json:"provider,omitempty"`

	// CreationTime is the creation time of component version
	CreationTime metav1.Time `json:"creationTime,omitempty"`
}

// CatalogItemStatus defines the observed state of a CatalogItem.
type CatalogItemStatus struct {
	// LastDiscoveredAt is when this item was last seen by the discovery service.
	// +optional
	LastDiscoveredAt *metav1.Time `json:"lastDiscoveredAt,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
