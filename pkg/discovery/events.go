// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"time"

	"github.com/go-logr/logr"
	"ocm.software/ocm/api/ocm/compdesc"
)

type DiscoveryEvent interface {
	SetTimestamp()
}

type DiscoveryEventImpl struct {
	// Timestamp is the timestamp when the event was created
	Timestamp time.Time
}

func (d DiscoveryEventImpl) SetTimestamp() {
	d.Timestamp = time.Now().UTC()
}

type Registry struct {
	// PlainHTTP is a boolean flag indicating whether the repository was discovered using plain HTTP
	PlainHTTP bool
	// Hostname is the hostname of the registry
	Hostname string
	// Username is the username used to authenticate with the registry
	Username string
	// Password is the password used to authenticate with the registry
	Password string
}

// RepositoryEvent represents an event sent by the RegistryScanner or Webhook Server containing
// information about discovered artifacts in the OCI registry.
type RepositoryEvent struct {
	DiscoveryEventImpl
	// Registry is the registry from which the event was discovered
	Registry Registry
	// Repository is the name of the repository in the registry
	Repository string
	// Version is an optional field that contains the version of the component discovered
	Version string
}

type ComponentVersionEvent struct {
	DiscoveryEventImpl
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
	DiscoveryEventImpl
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
