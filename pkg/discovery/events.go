// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import "time"

type Schema string

const (
	SCHEMA_HTTP  Schema = "http"
	SCHEMA_HTTPS Schema = "https"
)

// RepositoryEvent represents an event sent by the RegistryScanner or Webhook Server containing
// information about discovered artifacts in the OCI registry.
type RepositoryEvent struct {
	// Schema is the schema type to access the registry
	Schema Schema
	// Registry is the hostname of the registry
	Registry string
	// Repository is the name of the repository in the registry
	Repository string
	// Version is an optional field that contains the version of the component discovered
	Version string
	// Timestamp is the timestamp when the event was created
	Timestamp time.Time
}

type ComponentVersionEvent struct {
	Source RepositoryEvent
	// Namespace is the OCM namespace of the component
	Namespace string
	// Component is the name of the OCM component
	Component string
	// Version is the version of the component
	Version string
	// Timestamp is the timestamp when the event was created
	Timestamp time.Time
}

// ErrorEvent represents an event sent by the RegistryScanner or Webhook Server containing information about errors.
type ErrorEvent struct {
	// Timestamp is the timestamp when the event was created
	Timestamp time.Time
	// Error is when an error occurred
	Error error
}
