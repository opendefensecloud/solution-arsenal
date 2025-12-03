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

// Package ocm provides types and utilities for working with Open Component Model (OCM) packages.
package ocm

import (
	"encoding/json"
	"fmt"
	"time"

	"go.yaml.in/yaml/v3"
)

// ComponentDescriptor describes an OCM component.
type ComponentDescriptor struct {
	// Meta contains schema information
	Meta Metadata `json:"meta" yaml:"meta"`
	// Component contains the component specification
	Component ComponentSpec `json:"component" yaml:"component"`
}

// Metadata contains schema version information.
type Metadata struct {
	// SchemaVersion is the OCM schema version (e.g., "v2")
	SchemaVersion string `json:"schemaVersion" yaml:"schemaVersion"`
}

// ComponentSpec describes a component.
type ComponentSpec struct {
	// Name is the component name (e.g., "github.com/org/component")
	Name string `json:"name" yaml:"name"`
	// Version is the component version (semver)
	Version string `json:"version" yaml:"version"`
	// Provider describes who provides the component
	Provider Provider `json:"provider" yaml:"provider"`
	// RepositoryContexts lists the repositories where the component is available
	RepositoryContexts []RepositoryContext `json:"repositoryContexts,omitempty" yaml:"repositoryContexts,omitempty"`
	// Sources lists the source code references
	Sources []Source `json:"sources,omitempty" yaml:"sources,omitempty"`
	// Resources lists the resources (artifacts) in the component
	Resources []Resource `json:"resources" yaml:"resources"`
	// References lists references to other components
	References []Reference `json:"componentReferences,omitempty" yaml:"componentReferences,omitempty"`
	// Labels are arbitrary key-value pairs
	Labels Labels `json:"labels,omitempty" yaml:"labels,omitempty"`
	// CreationTime is when the component was created
	CreationTime *time.Time `json:"creationTime,omitempty" yaml:"creationTime,omitempty"`
}

// Provider describes the component provider.
type Provider struct {
	// Name is the provider name
	Name string `json:"name" yaml:"name"`
	// Labels are arbitrary labels for the provider
	Labels Labels `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// RepositoryContext describes a repository where the component is stored.
type RepositoryContext struct {
	// Type is the repository type (e.g., "OCIRegistry")
	Type string `json:"type" yaml:"type"`
	// BaseURL is the base URL of the repository
	BaseURL string `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	// ComponentNameMapping defines how component names map to repository paths
	ComponentNameMapping string `json:"componentNameMapping,omitempty" yaml:"componentNameMapping,omitempty"`
}

// Source describes a source code reference.
type Source struct {
	// Name is the source name
	Name string `json:"name" yaml:"name"`
	// Version is the source version
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// Type is the source type (e.g., "git")
	Type string `json:"type" yaml:"type"`
	// Access describes how to access the source
	Access AccessSpec `json:"access" yaml:"access"`
	// Labels are arbitrary labels
	Labels Labels `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// Resource describes a resource (artifact) in the component.
type Resource struct {
	// Name is the resource name
	Name string `json:"name" yaml:"name"`
	// Version is the resource version
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// Type is the resource type (e.g., "ociImage", "helmChart", "blueprint")
	Type string `json:"type" yaml:"type"`
	// Relation describes the relationship (local, external)
	Relation string `json:"relation,omitempty" yaml:"relation,omitempty"`
	// Access describes how to access the resource
	Access AccessSpec `json:"access" yaml:"access"`
	// Digest is the content digest
	Digest *DigestSpec `json:"digest,omitempty" yaml:"digest,omitempty"`
	// Labels are arbitrary labels
	Labels Labels `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// Reference describes a reference to another component.
type Reference struct {
	// Name is the reference name
	Name string `json:"name" yaml:"name"`
	// ComponentName is the referenced component name
	ComponentName string `json:"componentName" yaml:"componentName"`
	// Version is the referenced version
	Version string `json:"version" yaml:"version"`
	// Digest is the component digest
	Digest *DigestSpec `json:"digest,omitempty" yaml:"digest,omitempty"`
	// Labels are arbitrary labels
	Labels Labels `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// AccessSpec describes how to access a resource.
type AccessSpec struct {
	// Type is the access type (e.g., "ociArtifact", "ociBlob", "localBlob")
	Type string `json:"type" yaml:"type"`
	// ImageReference is the OCI image reference (for ociArtifact)
	ImageReference string `json:"imageReference,omitempty" yaml:"imageReference,omitempty"`
	// LocalReference is the local blob reference (for localBlob)
	LocalReference string `json:"localReference,omitempty" yaml:"localReference,omitempty"`
	// MediaType is the media type of the artifact
	MediaType string `json:"mediaType,omitempty" yaml:"mediaType,omitempty"`
	// ReferenceName is a reference name (for localBlob)
	ReferenceName string `json:"referenceName,omitempty" yaml:"referenceName,omitempty"`
	// GlobalAccess provides external access information
	GlobalAccess *GlobalAccessSpec `json:"globalAccess,omitempty" yaml:"globalAccess,omitempty"`
}

// GlobalAccessSpec describes global (external) access to a resource.
type GlobalAccessSpec struct {
	// Type is the global access type
	Type string `json:"type" yaml:"type"`
	// ImageReference is the OCI image reference
	ImageReference string `json:"imageReference,omitempty" yaml:"imageReference,omitempty"`
	// Digest is the content digest
	Digest string `json:"digest,omitempty" yaml:"digest,omitempty"`
}

// DigestSpec describes a content digest.
type DigestSpec struct {
	// HashAlgorithm is the hash algorithm (e.g., "SHA-256")
	HashAlgorithm string `json:"hashAlgorithm" yaml:"hashAlgorithm"`
	// NormalisationAlgorithm is the normalisation algorithm
	NormalisationAlgorithm string `json:"normalisationAlgorithm,omitempty" yaml:"normalisationAlgorithm,omitempty"`
	// Value is the digest value
	Value string `json:"value" yaml:"value"`
}

// Labels is a collection of labels.
type Labels []Label

// Label is a key-value label.
type Label struct {
	// Name is the label name
	Name string `json:"name" yaml:"name"`
	// Value is the label value (can be any JSON value)
	Value json.RawMessage `json:"value" yaml:"value"`
	// Version is the label schema version
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// Signing indicates if this label should be included in signing
	Signing bool `json:"signing,omitempty" yaml:"signing,omitempty"`
}

// Get returns the value for a label by name.
func (l Labels) Get(name string) (json.RawMessage, bool) {
	for _, label := range l {
		if label.Name == name {
			return label.Value, true
		}
	}
	return nil, false
}

// GetString returns the string value for a label by name.
func (l Labels) GetString(name string) (string, bool) {
	val, ok := l.Get(name)
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(val, &s); err != nil {
		return "", false
	}
	return s, true
}

// ToMap converts labels to a string map (only string values).
func (l Labels) ToMap() map[string]string {
	result := make(map[string]string)
	for _, label := range l {
		var s string
		if err := json.Unmarshal(label.Value, &s); err == nil {
			result[label.Name] = s
		}
	}
	return result
}

// ParseComponentDescriptor parses a component descriptor from JSON.
func ParseComponentDescriptor(data []byte) (*ComponentDescriptor, error) {
	var cd ComponentDescriptor
	if err := json.Unmarshal(data, &cd); err != nil {
		return nil, fmt.Errorf("parsing component descriptor: %w", err)
	}
	return &cd, nil
}

// ParseComponentDescriptorYAML parses a component descriptor from YAML.
func ParseComponentDescriptorYAML(data []byte) (*ComponentDescriptor, error) {
	var cd ComponentDescriptor
	if err := yaml.Unmarshal(data, &cd); err != nil {
		return nil, fmt.Errorf("parsing component descriptor YAML: %w", err)
	}
	return &cd, nil
}

// Validate validates the component descriptor.
func (cd *ComponentDescriptor) Validate() error {
	if cd.Meta.SchemaVersion == "" {
		return fmt.Errorf("meta.schemaVersion is required")
	}
	if cd.Component.Name == "" {
		return fmt.Errorf("component.name is required")
	}
	if cd.Component.Version == "" {
		return fmt.Errorf("component.version is required")
	}
	if cd.Component.Provider.Name == "" {
		return fmt.Errorf("component.provider.name is required")
	}
	return nil
}

// GetResource returns a resource by name.
func (cd *ComponentDescriptor) GetResource(name string) (*Resource, bool) {
	for i := range cd.Component.Resources {
		if cd.Component.Resources[i].Name == name {
			return &cd.Component.Resources[i], true
		}
	}
	return nil, false
}

// GetResourcesByType returns all resources of a given type.
func (cd *ComponentDescriptor) GetResourcesByType(resourceType string) []Resource {
	var result []Resource
	for _, r := range cd.Component.Resources {
		if r.Type == resourceType {
			result = append(result, r)
		}
	}
	return result
}

// GetReference returns a reference by name.
func (cd *ComponentDescriptor) GetReference(name string) (*Reference, bool) {
	for i := range cd.Component.References {
		if cd.Component.References[i].Name == name {
			return &cd.Component.References[i], true
		}
	}
	return nil, false
}

// Common resource types
const (
	ResourceTypeOCIImage   = "ociImage"
	ResourceTypeHelmChart  = "helmChart"
	ResourceTypeBlueprint  = "blueprint"
	ResourceTypePlainText  = "plainText"
	ResourceTypeDirectory  = "directoryTree"
)

// Common access types
const (
	AccessTypeOCIArtifact = "ociArtifact"
	AccessTypeOCIBlob     = "ociBlob"
	AccessTypeLocalBlob   = "localBlob"
	AccessTypeGitHub      = "github"
	AccessTypeS3          = "s3"
)

// Common repository types
const (
	RepositoryTypeOCIRegistry = "OCIRegistry"
	RepositoryTypeCommonTransportFormat = "CommonTransportFormat"
)
