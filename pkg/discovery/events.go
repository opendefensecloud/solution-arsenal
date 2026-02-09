// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"time"

	"github.com/go-logr/logr"
	"ocm.software/ocm/api/ocm/compdesc"

	"go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// EventType is an enumeration representing different types of events that can occur.
type EventType string

const (
	EventCreated EventType = "created"
	EventUpdated EventType = "updated"
	EventDeleted EventType = "deleted"
)

// RepositoryEvent represents an event sent by the RegistryScanner or Webhook Server containing
// information about discovered artifacts in the OCI registry.
type RepositoryEvent struct {
	// Registry is the name of the registry from which the event was discovered.
	Registry string
	// Repository is the name of the repository in the registry.
	Repository string
	// Version is an optional field that contains the version of the component discovered.
	Version string
	// Type is the type of event.
	Type EventType
	// Timestamp is the timestamp when the event was created.
	Timestamp time.Time
}

type ComponentVersionEvent struct {
	// Source is the event from which the component was discovered.
	Source RepositoryEvent
	// Namespace is the OCM namespace of the component.
	Namespace string
	// Component is the name of the OCM component.
	Component string
	// Descriptor is the component descriptor of the component.
	Descriptor *compdesc.ComponentDescriptor
	// Type is the type of event.
	Type EventType
	// Timestamp is the timestamp when the event was created.
	Timestamp time.Time
	// ComponentVersion is the component version of the component.
	ComponentVersion *v1alpha1.ComponentVersion
}

// ErrorEvent represents an event sent by the RegistryScanner or Webhook Server containing information about errors.
type ErrorEvent struct {
	// Error is when an error occurred.
	Error error
	// Timestamp is the timestamp when the event was created.
	Timestamp time.Time
}

// Publish publishes the given event to the given channel.
// Drops events if the channel is full.
func Publish[T any](log *logr.Logger, channel chan<- T, event T) {
	select {
	case channel <- event:
	default:
		log.V(1).Info("error event channel full, dropping event", "event", event)
	}
}
