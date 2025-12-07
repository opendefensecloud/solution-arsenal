// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CatalogItemSpec struct {
	// ComponentName is the OCM component name.
	ComponentName string `json:"componentName"`
	// Version is the semantic version of the component.
	Version string `json:"version"`
	// Repository is the OCI repository URL where the component is stored.
	Repository string `json:"repository"`
	// Description provides a human-readable description of the catalog item.
	// +optional
	Description string `json:"description,omitempty"`
}

type CatalogItemStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CatalogItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   CatalogItemSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status CatalogItemStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CatalogItemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []CatalogItem `json:"items" protobuf:"bytes,2,rep,name=items"`
}
