/*
Copyright 2024 Open Defense Cloud Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Common types used across resources

// ObjectReference contains enough information to let you locate the referenced object.
type ObjectReference struct {
	// Name of the referent.
	Name string `json:"name"`
	// Namespace of the referent. If not specified, the local namespace is assumed.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ComponentReference references an OCM component.
type ComponentReference struct {
	// Name is the OCM component name.
	Name string `json:"name"`
	// Version is the semantic version of the component.
	// +optional
	Version string `json:"version,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CatalogItem represents an OCM package available in the catalog.
// CatalogItems are namespaced and represent solutions available within a tenant.
type CatalogItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CatalogItemSpec   `json:"spec,omitempty"`
	Status CatalogItemStatus `json:"status,omitempty"`
}

// CatalogItemSpec defines the desired state of a CatalogItem.
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
	// Labels for categorization and filtering.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Dependencies lists other components this catalog item depends on.
	// +optional
	Dependencies []ComponentReference `json:"dependencies,omitempty"`
}

// CatalogItemPhase represents the phase of a CatalogItem.
type CatalogItemPhase string

const (
	// CatalogItemPhaseAvailable indicates the catalog item is available for deployment.
	CatalogItemPhaseAvailable CatalogItemPhase = "Available"
	// CatalogItemPhaseUnavailable indicates the catalog item is not available.
	CatalogItemPhaseUnavailable CatalogItemPhase = "Unavailable"
	// CatalogItemPhaseDeprecated indicates the catalog item is deprecated.
	CatalogItemPhaseDeprecated CatalogItemPhase = "Deprecated"
)

// CatalogItemStatus defines the observed state of a CatalogItem.
type CatalogItemStatus struct {
	// Phase indicates the current state of the catalog item.
	// +optional
	Phase CatalogItemPhase `json:"phase,omitempty"`
	// Conditions represent the latest available observations of the catalog item's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// LastScanned is the timestamp of the last discovery scan.
	// +optional
	LastScanned *metav1.Time `json:"lastScanned,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CatalogItemList contains a list of CatalogItem.
type CatalogItemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CatalogItem `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterCatalogItem is a cluster-scoped variant of CatalogItem.
// ClusterCatalogItems are available to all tenants in the cluster.
type ClusterCatalogItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CatalogItemSpec   `json:"spec,omitempty"`
	Status CatalogItemStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterCatalogItemList contains a list of ClusterCatalogItem.
type ClusterCatalogItemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterCatalogItem `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRegistration represents a Kubernetes cluster registered with SolAr.
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of a ClusterRegistration.
type ClusterRegistrationSpec struct {
	// DisplayName is a human-readable name for the cluster.
	DisplayName string `json:"displayName"`
	// Description provides additional context about the cluster.
	// +optional
	Description string `json:"description,omitempty"`
	// Labels for cluster categorization and targeting.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// AgentConfig holds configuration for the solar-agent deployed to this cluster.
	// +optional
	AgentConfig AgentConfiguration `json:"agentConfig,omitempty"`
}

// AgentConfiguration defines the configuration for a solar-agent.
type AgentConfiguration struct {
	// SyncEnabled enables catalog chaining via Sync resources.
	// +optional
	SyncEnabled bool `json:"syncEnabled,omitempty"`
	// ARCEndpoint is the ARC endpoint for catalog chaining.
	// +optional
	ARCEndpoint string `json:"arcEndpoint,omitempty"`
}

// ClusterPhase represents the phase of a ClusterRegistration.
type ClusterPhase string

const (
	// ClusterPhasePending indicates the cluster is pending agent deployment.
	ClusterPhasePending ClusterPhase = "Pending"
	// ClusterPhaseConnecting indicates the agent is connecting.
	ClusterPhaseConnecting ClusterPhase = "Connecting"
	// ClusterPhaseReady indicates the cluster is ready for deployments.
	ClusterPhaseReady ClusterPhase = "Ready"
	// ClusterPhaseNotReady indicates the cluster is not ready.
	ClusterPhaseNotReady ClusterPhase = "NotReady"
	// ClusterPhaseUnreachable indicates the cluster is unreachable.
	ClusterPhaseUnreachable ClusterPhase = "Unreachable"
)

// ReleaseReference is a reference to a Release.
type ReleaseReference struct {
	// Name of the release.
	Name string `json:"name"`
	// Version of the release.
	Version string `json:"version"`
}

// ClusterRegistrationStatus defines the observed state of a ClusterRegistration.
type ClusterRegistrationStatus struct {
	// Phase indicates the current state of the cluster registration.
	// +optional
	Phase ClusterPhase `json:"phase,omitempty"`
	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// AgentVersion is the version of the solar-agent running in the cluster.
	// +optional
	AgentVersion string `json:"agentVersion,omitempty"`
	// KubernetesVersion is the version of Kubernetes in the cluster.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	// LastHeartbeat is the timestamp of the last agent heartbeat.
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`
	// InstalledReleases lists releases currently installed on this cluster.
	// +optional
	InstalledReleases []ReleaseReference `json:"installedReleases,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRegistrationList contains a list of ClusterRegistration.
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistration `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Release represents a deployment of a CatalogItem to a cluster.
type Release struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleaseSpec   `json:"spec,omitempty"`
	Status ReleaseStatus `json:"status,omitempty"`
}

// ReleaseSpec defines the desired state of a Release.
type ReleaseSpec struct {
	// CatalogItemRef references the catalog item to deploy.
	CatalogItemRef ObjectReference `json:"catalogItemRef"`
	// TargetClusterRef references the target cluster for deployment.
	TargetClusterRef ObjectReference `json:"targetClusterRef"`
	// Values are the configuration values for the release.
	// +optional
	Values runtime.RawExtension `json:"values,omitempty"`
	// Suspend prevents reconciliation when set to true.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ReleasePhase represents the phase of a Release.
type ReleasePhase string

const (
	// ReleasePhasePending indicates the release is pending.
	ReleasePhasePending ReleasePhase = "Pending"
	// ReleasePhaseRendering indicates the release is being rendered.
	ReleasePhaseRendering ReleasePhase = "Rendering"
	// ReleasePhaseDeploying indicates the release is being deployed.
	ReleasePhaseDeploying ReleasePhase = "Deploying"
	// ReleasePhaseDeployed indicates the release is deployed.
	ReleasePhaseDeployed ReleasePhase = "Deployed"
	// ReleasePhaseFailed indicates the release failed.
	ReleasePhaseFailed ReleasePhase = "Failed"
	// ReleasePhaseSuspended indicates the release is suspended.
	ReleasePhaseSuspended ReleasePhase = "Suspended"
)

// ReleaseStatus defines the observed state of a Release.
type ReleaseStatus struct {
	// Phase indicates the current state of the release.
	// +optional
	Phase ReleasePhase `json:"phase,omitempty"`
	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// AppliedVersion is the version currently applied to the cluster.
	// +optional
	AppliedVersion string `json:"appliedVersion,omitempty"`
	// LastAppliedTime is the timestamp of the last successful apply.
	// +optional
	LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`
	// ObservedGeneration is the last observed generation of the release.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReleaseList contains a list of Release.
type ReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Release `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Sync represents a catalog chaining configuration.
type Sync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncSpec   `json:"spec,omitempty"`
	Status SyncStatus `json:"status,omitempty"`
}

// SyncSpec defines the desired state of a Sync.
type SyncSpec struct {
	// SourceRef references the source catalog item or pattern.
	SourceRef ObjectReference `json:"sourceRef"`
	// DestinationRegistry is the destination OCI registry URL.
	DestinationRegistry string `json:"destinationRegistry"`
	// Filter defines rules for filtering which items to sync.
	// +optional
	Filter SyncFilter `json:"filter,omitempty"`
}

// SyncFilter defines filtering rules for sync operations.
type SyncFilter struct {
	// IncludeLabels specifies labels that items must have to be included.
	// +optional
	IncludeLabels map[string]string `json:"includeLabels,omitempty"`
	// ExcludeLabels specifies labels that exclude items from sync.
	// +optional
	ExcludeLabels map[string]string `json:"excludeLabels,omitempty"`
}

// SyncPhase represents the phase of a Sync.
type SyncPhase string

const (
	// SyncPhasePending indicates the sync is pending.
	SyncPhasePending SyncPhase = "Pending"
	// SyncPhaseSyncing indicates the sync is in progress.
	SyncPhaseSyncing SyncPhase = "Syncing"
	// SyncPhaseSynced indicates the sync completed successfully.
	SyncPhaseSynced SyncPhase = "Synced"
	// SyncPhaseFailed indicates the sync failed.
	SyncPhaseFailed SyncPhase = "Failed"
)

// SyncStatus defines the observed state of a Sync.
type SyncStatus struct {
	// Phase indicates the current state of the sync.
	// +optional
	Phase SyncPhase `json:"phase,omitempty"`
	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// LastSyncTime is the timestamp of the last successful sync.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	// SyncedItems is the count of items synced in the last operation.
	// +optional
	SyncedItems int `json:"syncedItems,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncList contains a list of Sync.
type SyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sync `json:"items"`
}
