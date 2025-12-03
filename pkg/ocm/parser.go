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

package ocm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/opendefensecloud/solution-arsenal/pkg/registry/oci"
)

// Parser parses OCM component descriptors from OCI registries.
type Parser struct {
	client oci.Client
}

// NewParser creates a new OCM parser.
func NewParser(client oci.Client) *Parser {
	return &Parser{client: client}
}

// OCM media types
const (
	// MediaTypeComponentDescriptorV2 is the media type for OCM component descriptors v2
	MediaTypeComponentDescriptorV2 = "application/vnd.ocm.software.component-descriptor.v2+json"
	// MediaTypeComponentDescriptorV2Yaml is the YAML media type
	MediaTypeComponentDescriptorV2Yaml = "application/vnd.ocm.software.component-descriptor.v2+yaml"
	// MediaTypeComponentDescriptorConfig is the config media type for component descriptors
	MediaTypeComponentDescriptorConfig = "application/vnd.ocm.software.component.config.v1+json"
	// MediaTypeOCMComponentLayer is the layer media type for component archives
	MediaTypeOCMComponentLayer = "application/vnd.ocm.software.component.layer.v1.tar+gzip"
	// MediaTypeOCMComponentLayerTar is the uncompressed layer media type
	MediaTypeOCMComponentLayerTar = "application/vnd.ocm.software.component.layer.v1.tar"
)

// ComponentDescriptorFileName is the standard filename for component descriptors
const ComponentDescriptorFileName = "component-descriptor.yaml"

// ParseResult contains the result of parsing an OCM component.
type ParseResult struct {
	// Descriptor is the parsed component descriptor
	Descriptor *ComponentDescriptor
	// Repository is the source repository
	Repository string
	// Tag is the tag/reference used to fetch the component
	Tag string
	// Digest is the manifest digest
	Digest string
}

// ParseComponent parses an OCM component from the registry.
func (p *Parser) ParseComponent(ctx context.Context, repository, reference string) (*ParseResult, error) {
	// Get the manifest
	manifest, err := p.client.GetManifest(ctx, repository, reference)
	if err != nil {
		return nil, fmt.Errorf("getting manifest: %w", err)
	}

	// Check if this looks like an OCM component
	if !isOCMManifest(manifest) {
		return nil, fmt.Errorf("not an OCM component manifest")
	}

	// Find the component descriptor layer
	var descriptorLayer *oci.Descriptor
	for i := range manifest.Layers {
		layer := &manifest.Layers[i]
		if isComponentDescriptorLayer(layer) {
			descriptorLayer = layer
			break
		}
	}

	if descriptorLayer == nil {
		// Try to get descriptor from config blob
		descriptor, err := p.parseFromConfig(ctx, repository, &manifest.Config)
		if err != nil {
			return nil, fmt.Errorf("no component descriptor found: %w", err)
		}
		return &ParseResult{
			Descriptor: descriptor,
			Repository: repository,
			Tag:        reference,
		}, nil
	}

	// Get the component descriptor blob
	descriptor, err := p.parseFromLayer(ctx, repository, descriptorLayer)
	if err != nil {
		return nil, fmt.Errorf("parsing component descriptor: %w", err)
	}

	return &ParseResult{
		Descriptor: descriptor,
		Repository: repository,
		Tag:        reference,
		Digest:     descriptorLayer.Digest,
	}, nil
}

// isOCMManifest checks if a manifest looks like an OCM component.
func isOCMManifest(manifest *oci.Manifest) bool {
	// Check config media type
	if strings.Contains(manifest.Config.MediaType, "ocm") ||
		strings.Contains(manifest.Config.MediaType, "component") {
		return true
	}

	// Check layer media types
	for _, layer := range manifest.Layers {
		if strings.Contains(layer.MediaType, "ocm") ||
			strings.Contains(layer.MediaType, "component-descriptor") {
			return true
		}
	}

	// Check annotations
	if manifest.Annotations != nil {
		if _, ok := manifest.Annotations["software.ocm.componentName"]; ok {
			return true
		}
		if _, ok := manifest.Annotations["software.ocm.componentVersion"]; ok {
			return true
		}
	}

	return false
}

// isComponentDescriptorLayer checks if a layer contains a component descriptor.
func isComponentDescriptorLayer(layer *oci.Descriptor) bool {
	// Check media type
	switch layer.MediaType {
	case MediaTypeComponentDescriptorV2,
		MediaTypeComponentDescriptorV2Yaml,
		MediaTypeOCMComponentLayer,
		MediaTypeOCMComponentLayerTar:
		return true
	}

	// Check annotations
	if layer.Annotations != nil {
		if title, ok := layer.Annotations["org.opencontainers.image.title"]; ok {
			if title == ComponentDescriptorFileName || title == "component-descriptor.json" {
				return true
			}
		}
	}

	return false
}

// parseFromLayer parses a component descriptor from a layer blob.
func (p *Parser) parseFromLayer(ctx context.Context, repository string, layer *oci.Descriptor) (*ComponentDescriptor, error) {
	reader, err := p.client.GetBlob(ctx, repository, layer.Digest)
	if err != nil {
		return nil, fmt.Errorf("getting blob: %w", err)
	}
	defer reader.Close()

	// Handle different media types
	switch layer.MediaType {
	case MediaTypeComponentDescriptorV2:
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("reading blob: %w", err)
		}
		return ParseComponentDescriptor(data)

	case MediaTypeComponentDescriptorV2Yaml:
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("reading blob: %w", err)
		}
		return ParseComponentDescriptorYAML(data)

	case MediaTypeOCMComponentLayer:
		// Gzip compressed tar archive
		return p.parseFromTarGz(reader)

	case MediaTypeOCMComponentLayerTar:
		// Uncompressed tar archive
		return p.parseFromTar(reader)

	default:
		// Try to parse as tar.gz, then tar, then raw JSON/YAML
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("reading blob: %w", err)
		}

		// Try JSON first
		if cd, err := ParseComponentDescriptor(data); err == nil {
			return cd, nil
		}

		// Try YAML
		if cd, err := ParseComponentDescriptorYAML(data); err == nil {
			return cd, nil
		}

		return nil, fmt.Errorf("unable to parse layer with media type %s", layer.MediaType)
	}
}

// parseFromTarGz parses a component descriptor from a gzipped tar archive.
func (p *Parser) parseFromTarGz(reader io.Reader) (*ComponentDescriptor, error) {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzReader.Close()

	return p.parseFromTar(gzReader)
}

// parseFromTar parses a component descriptor from a tar archive.
func (p *Parser) parseFromTar(reader io.Reader) (*ComponentDescriptor, error) {
	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar: %w", err)
		}

		// Look for component-descriptor.yaml or component-descriptor.json
		name := header.Name
		if name == ComponentDescriptorFileName || name == "./"+ComponentDescriptorFileName ||
			name == "component-descriptor.json" || name == "./component-descriptor.json" {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("reading descriptor file: %w", err)
			}

			if strings.HasSuffix(name, ".yaml") {
				return ParseComponentDescriptorYAML(data)
			}
			return ParseComponentDescriptor(data)
		}
	}

	return nil, fmt.Errorf("component descriptor not found in archive")
}

// parseFromConfig tries to parse component info from the config blob.
func (p *Parser) parseFromConfig(ctx context.Context, repository string, config *oci.Descriptor) (*ComponentDescriptor, error) {
	reader, err := p.client.GetBlob(ctx, repository, config.Digest)
	if err != nil {
		return nil, fmt.Errorf("getting config blob: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading config blob: %w", err)
	}

	// Try to parse as component descriptor config
	var configData struct {
		ComponentDescriptor *ComponentDescriptor `json:"componentDescriptor"`
	}

	if err := json.Unmarshal(data, &configData); err == nil && configData.ComponentDescriptor != nil {
		return configData.ComponentDescriptor, nil
	}

	// Try to parse as raw component descriptor
	return ParseComponentDescriptor(data)
}

// ListComponents lists all potential OCM components in a repository by scanning tags.
func (p *Parser) ListComponents(ctx context.Context, repository string) ([]string, error) {
	tags, err := p.client.ListTags(ctx, repository)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}

	// Filter tags that look like versions
	var versions []string
	for _, tag := range tags {
		// Include tags that look like semver
		if isVersionTag(tag) {
			versions = append(versions, tag)
		}
	}

	return versions, nil
}

// isVersionTag checks if a tag looks like a version.
func isVersionTag(tag string) bool {
	// Skip common non-version tags
	if tag == "latest" || tag == "main" || tag == "master" || tag == "dev" {
		return false
	}

	// Accept tags starting with 'v' followed by a digit
	if strings.HasPrefix(tag, "v") && len(tag) > 1 {
		c := tag[1]
		if c >= '0' && c <= '9' {
			return true
		}
	}

	// Accept tags starting with a digit
	if len(tag) > 0 {
		c := tag[0]
		if c >= '0' && c <= '9' {
			return true
		}
	}

	return false
}
