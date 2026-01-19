// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Cron represents a cron schedule.
type Cron struct {
	// Timezone is the timezone against which the cron schedule will be calculated, e.g. "Asia/Tokyo". Default is machine's local time.
	Timezone string `json:"timezone,omitempty"`
	// StartingDeadlineSeconds is the K8s-style deadline that will limit the time a schedule will be run after its
	// original scheduled time if it is missed.
	// +kubebuilder:validation:Minimum=0
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`
	// Schedules is a list of schedules to run in Cron format
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:Pattern=`^(@(yearly|annually|monthly|weekly|daily|midnight|hourly)|@every\s+([0-9]+(ns|us|µs|ms|s|m|h))+|([0-9*,/?-]+\s+){4}[0-9*,/?-]+)$`
	Schedules []string `json:"schedules"`
}

// WebhookAuth represents authentication for a webhook, e.g. basic auth.
type WebhookAuth struct {
	// Type is the type of authentication to use for this webhook. Currently, only "basic" is supported.
	// +kubebuilder:validation:items:Pattern=`^(@(basic)$`
	Type string `json:"type,omitempty"`
	// SecretRef references the secret containing the credentials.
	SecretRef corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

// Webhook represents the configuration for a webhook.
type Webhook struct {
	// Flavor is the webhook implementation to use.
	// +kubebuilder:validation:items:Pattern=`^(@(zot)$`
	Flavor string `json:"flavor,omitempty"`
	// Path is where the webhook should listen.
	Path string `json:"path,omitempty"`
	// Auth are the authentication information to use with the webhook.
	Auth []WebhookAuth `json:"auth,omitempty"`
}

// DiscoverySpec defines the desired state of a Discovery.
type DiscoverySpec struct {
	// RegistryURL defines the URL which is used to connect to the registry.
	RegistryURL string `json:"registryURL"`

	// SecretRef specifies the secret containing the relevant credentials for the registry that should be used during discovery.
	// +optional
	DiscoverySecretRef corev1.LocalObjectReference `json:"discoverySecretRef"`

	// SecretRef specifies the secret containing the relevant credentials for the registry that should be used when a discovered component is part of a release. If not specified uses .spec.discoverySecretRef.
	// +optional
	ReleaseSecretRef corev1.LocalObjectReference `json:"releaseSecretRef"`

	// Cron specifies options which determine when the discover process should run for the given registry.
	// +optional
	Cron *Cron `json:"cron,omitempty"`

	// DisableStartupDiscovery defines whether the discovery should not be run on startup of the discovery process. If true it will only run on schedule, see .spec.cron.
	// +optional
	DisableStartupDiscovery bool `json:"disableStartupDiscovery,omitempty"`

	// Webhook specifies the configuration for a webhook that is called by the registry on created, updated or deleted images/repositories.
	// +optional
	Webhook *Webhook `json:"webhook,omitempty"`
}

// DiscoveryStatus defines the observed state of a Discovery.
type DiscoveryStatus struct {
	// PodDiscoveryVersion is the version of the discovery object at the time the worker was instantiated.
	PodDiscoveryVersion string `json:"podDiscoveryVersion"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Discovery represents represents a configuration for a registry to discover.
type Discovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   DiscoverySpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status DiscoveryStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DiscoveryList contains a list of Discovery resources.
type DiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Discovery `json:"items" protobuf:"bytes,2,rep,name=items"`
}
