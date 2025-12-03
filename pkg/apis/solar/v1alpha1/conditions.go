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

// Condition type constants for CatalogItem.
const (
	// CatalogItemConditionTypeReady indicates the catalog item is ready for use.
	CatalogItemConditionTypeReady = "Ready"
	// CatalogItemConditionTypeAvailable indicates the component is available in the registry.
	CatalogItemConditionTypeAvailable = "Available"
	// CatalogItemConditionTypeValidated indicates the component has been validated.
	CatalogItemConditionTypeValidated = "Validated"
)

// Condition type constants for ClusterRegistration.
const (
	// ClusterConditionTypeReady indicates the cluster is ready for deployments.
	ClusterConditionTypeReady = "Ready"
	// ClusterConditionTypeAgentConnected indicates the agent is connected.
	ClusterConditionTypeAgentConnected = "AgentConnected"
	// ClusterConditionTypePreflightPassed indicates preflight checks passed.
	ClusterConditionTypePreflightPassed = "PreflightPassed"
	// ClusterConditionTypeFluxInstalled indicates FluxCD is installed.
	ClusterConditionTypeFluxInstalled = "FluxInstalled"
)

// Condition type constants for Release.
const (
	// ReleaseConditionTypeReady indicates the release is fully deployed.
	ReleaseConditionTypeReady = "Ready"
	// ReleaseConditionTypeRendered indicates the release has been rendered.
	ReleaseConditionTypeRendered = "Rendered"
	// ReleaseConditionTypePublished indicates the release has been published to OCI.
	ReleaseConditionTypePublished = "Published"
	// ReleaseConditionTypeReconciled indicates FluxCD has reconciled the release.
	ReleaseConditionTypeReconciled = "Reconciled"
)

// Condition type constants for Sync.
const (
	// SyncConditionTypeReady indicates the sync is operational.
	SyncConditionTypeReady = "Ready"
	// SyncConditionTypeSynced indicates items have been synced.
	SyncConditionTypeSynced = "Synced"
)

// Condition reason constants.
const (
	// ReasonSucceeded indicates the operation succeeded.
	ReasonSucceeded = "Succeeded"
	// ReasonFailed indicates the operation failed.
	ReasonFailed = "Failed"
	// ReasonInProgress indicates the operation is in progress.
	ReasonInProgress = "InProgress"
	// ReasonPending indicates the operation is pending.
	ReasonPending = "Pending"
	// ReasonNotFound indicates a resource was not found.
	ReasonNotFound = "NotFound"
	// ReasonInvalidConfiguration indicates invalid configuration.
	ReasonInvalidConfiguration = "InvalidConfiguration"
	// ReasonTimeout indicates the operation timed out.
	ReasonTimeout = "Timeout"
	// ReasonUnauthorized indicates authorization failure.
	ReasonUnauthorized = "Unauthorized"
)
