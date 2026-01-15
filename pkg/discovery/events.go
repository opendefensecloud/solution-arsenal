// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"time"

	"github.com/go-logr/logr"
	"ocm.software/ocm/api/ocm/compdesc"
)

type Schema string

const (
	SCHEMA_HTTP  Schema = "http"
	SCHEMA_HTTPS Schema = "https"
)

type DiscoveryEvent interface {
	SetTimestamp()
}

type DiscoveryEventimpl struct {
	// Timestamp is the timestamp when the event was created
	Timestamp time.Time
}

func (d DiscoveryEventimpl) SetTimestamp() {
	d.Timestamp = time.Now().UTC()
}

// RepositoryEvent represents an event sent by the RegistryScanner or Webhook Server containing
// information about discovered artifacts in the OCI registry.
type RepositoryEvent struct {
	DiscoveryEventimpl
	// Schema is the schema type to access the registry
	Schema Schema
	// Registry is the hostname of the registry
	Registry string
	// Repository is the name of the repository in the registry
	Repository string
	// Version is an optional field that contains the version of the component discovered
	Version string
}

type ComponentVersionEvent struct {
	DiscoveryEventimpl
	Source RepositoryEvent
	// Namespace is the OCM namespace of the component
	Namespace string
	// Component is the name of the OCM component
	Component string
	// Descriptor is the component descriptor of the component
	Descriptor *compdesc.ComponentDescriptor
}

// ErrorEvent represents an event sent by the RegistryScanner or Webhook Server containing information about errors.
type ErrorEvent struct {
	DiscoveryEventimpl
	// Error is when an error occurred
	Error error
}

// Publish publishes the given event to the given channel.
// Drops events if the channel is full.
func Publish[T DiscoveryEvent](log *logr.Logger, channel chan<- T, event T) {
	event.SetTimestamp()
	select {
	case channel <- event:
	default:
		log.V(1).Info("error event channel full, dropping event", "event", event)
	}
}
