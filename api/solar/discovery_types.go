// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuthenticationType
// +enum
type AuthenticationType string

const (
	AuthenticationTypeBasic AuthenticationType = "Basic"
	AuthenticationTypeToken AuthenticationType = "Token"
)

type WebhookAuth struct {
	// Type represents the type of authentication to use. Currently, only "token" is supported.
	Type AuthenticationType `json:"type,omitempty"`
	// AuthSecretRef is the reference to the secret which contains the authentication information for the webhook.
	AuthSecretRef corev1.LocalObjectReference `json:"authSecretRef,omitempty"`
}

// Webhook represents the configuration for a webhook.
type Webhook struct {
	// Flavor is the webhook implementation to use.
	// +kubebuilder:validation:Pattern=`^(@(zot)$`
	Flavor string `json:"flavor,omitempty"`
	// Path is where the webhook should listen.
	Path string `json:"path,omitempty"`
	// Auth is the authentication information to use with the webhook.
	Auth WebhookAuth `json:"auth,omitempty"`
}

// Registry defines the configuration for a registry.
type Registry struct {
	// RegistryURL defines the URL which is used to connect to the registry.
	RegistryURL string `json:"registryURL"`

	// SecretRef specifies the secret containing the relevant credentials for the registry that should be used during discovery.
	// +optional
	DiscoverySecretRef corev1.LocalObjectReference `json:"discoverySecretRef"`

	// SecretRef specifies the secret containing the relevant credentials for the registry that should be used when a discovered component is part of a release. If not specified uses .spec.discoverySecretRef.
	// +optional
	ReleaseSecretRef corev1.LocalObjectReference `json:"releaseSecretRef"`
}

// Filter defines the filter criteria used to determine which components should be scanned.
type Filter struct {
	// RepositoryPatterns defines which repositories should be scanned for components. The default value is empty, which means that all repositories will be scanned.
	// Wildcards are supported, e.g. "foo-*" or "*-dev".
	RepositoryPatterns []string `json:"repositoryPatterns"`
}

// DiscoverySpec defines the desired state of a Discovery.
type DiscoverySpec struct {
	// Registry specifies the registry that should be scanned by the discovery process.
	Registry Registry `json:"registry"`

	// Webhook specifies the configuration for a webhook that is called by the registry on created, updated or deleted images/repositories.
	// +optional
	Webhook *Webhook `json:"webhook,omitempty"`

	// Filter specifies the filter that should be applied when scanning for components. If not specified, all components will be scanned.
	// +kubebuilder:validation:Optional
	Filter *Filter `json:"filter,omitempty"`

	// DiscoveryInterval is the amount of time between two full scans of the registry.
	// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"
	// May be set to zero to fetch and create it once. Defaults to 24h.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="24h"
	// +optional
	DiscoveryInterval *metav1.Duration `json:"discoveryInterval,omitempty"`

	// DisableStartupDiscovery defines whether the discovery should not be run on startup of the discovery process. If true it will only run on schedule, see .spec.cron.
	// +optional
	DisableStartupDiscovery bool `json:"disableStartupDiscovery,omitempty"`
}

// DiscoveryStatus defines the observed state of a Discovery.
type DiscoveryStatus struct {
	// PodGeneration is the generation of the discovery object at the time the worker was instantiated.
	PodGeneration int64 `json:"podGeneration"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Discovery represents a configuration for a registry to discover.
type Discovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   DiscoverySpec   `json:"spec,omitempty"   protobuf:"bytes,2,opt,name=spec"`
	Status DiscoveryStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DiscoveryList contains a list of Discovery resources.
type DiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Discovery `json:"items" protobuf:"bytes,2,rep,name=items"`
}
