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

package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// Client is an interface for interacting with OCI registries.
type Client interface {
	// ListTags lists all tags for a given repository.
	ListTags(ctx context.Context, repository string) ([]string, error)
	// GetManifest retrieves the manifest for a given repository and reference.
	GetManifest(ctx context.Context, repository, reference string) (*Manifest, error)
	// GetBlob retrieves a blob from the registry.
	GetBlob(ctx context.Context, repository, digest string) (io.ReadCloser, error)
	// Ping checks if the registry is accessible.
	Ping(ctx context.Context) error
}

// Manifest represents an OCI manifest.
type Manifest struct {
	SchemaVersion int              `json:"schemaVersion"`
	MediaType     string           `json:"mediaType"`
	Config        Descriptor       `json:"config"`
	Layers        []Descriptor     `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

// Descriptor describes a content addressable blob.
type Descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// TagList represents the response from the tags/list endpoint.
type TagList struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// RegistryClient implements Client for OCI registries.
type RegistryClient struct {
	registry   string
	httpClient *http.Client
	auth       Authenticator
	userAgent  string
}

// Authenticator provides authentication for registry requests.
type Authenticator interface {
	// Authenticate adds authentication to the request.
	Authenticate(req *http.Request) error
}

// ClientOption configures a RegistryClient.
type ClientOption func(*RegistryClient)

// WithHTTPClient sets the HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *RegistryClient) {
		c.httpClient = client
	}
}

// WithAuthenticator sets the authenticator.
func WithAuthenticator(auth Authenticator) ClientOption {
	return func(c *RegistryClient) {
		c.auth = auth
	}
}

// WithUserAgent sets the user agent string.
func WithUserAgent(ua string) ClientOption {
	return func(c *RegistryClient) {
		c.userAgent = ua
	}
}

// NewClient creates a new OCI registry client.
func NewClient(registry string, opts ...ClientOption) *RegistryClient {
	c := &RegistryClient{
		registry: normalizeRegistry(registry),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: "solar-discovery/1.0",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// normalizeRegistry ensures the registry URL has a scheme.
func normalizeRegistry(registry string) string {
	if !strings.HasPrefix(registry, "http://") && !strings.HasPrefix(registry, "https://") {
		return "https://" + registry
	}
	return registry
}

// Ping checks if the registry is accessible.
func (c *RegistryClient) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/v2/", c.registry)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		// Try to authenticate and retry
		if c.auth != nil {
			if err := c.auth.Authenticate(req); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			resp, err = c.httpClient.Do(req)
			if err != nil {
				return fmt.Errorf("executing authenticated request: %w", err)
			}
			defer resp.Body.Close()
		}
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// ListTags lists all tags for a given repository.
func (c *RegistryClient) ListTags(ctx context.Context, repository string) ([]string, error) {
	var allTags []string
	url := fmt.Sprintf("%s/v2/%s/tags/list", c.registry, repository)

	for url != "" {
		tags, nextURL, err := c.listTagsPage(ctx, url)
		if err != nil {
			return nil, err
		}
		allTags = append(allTags, tags...)
		url = nextURL
	}

	return allTags, nil
}

func (c *RegistryClient) listTagsPage(ctx context.Context, url string) ([]string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}

	c.setHeaders(req)

	if c.auth != nil {
		if err := c.auth.Authenticate(req); err != nil {
			return nil, "", fmt.Errorf("authentication: %w", err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var tagList TagList
	if err := json.NewDecoder(resp.Body).Decode(&tagList); err != nil {
		return nil, "", fmt.Errorf("decoding response: %w", err)
	}

	// Check for pagination Link header
	nextURL := ""
	if link := resp.Header.Get("Link"); link != "" {
		nextURL = parseLinkHeader(link, c.registry)
	}

	return tagList.Tags, nextURL, nil
}

// GetManifest retrieves the manifest for a given repository and reference.
func (c *RegistryClient) GetManifest(ctx context.Context, repository, reference string) (*Manifest, error) {
	url := fmt.Sprintf("%s/v2/%s/manifests/%s", c.registry, repository, reference)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setHeaders(req)
	// Accept OCI manifest media types
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
	}, ", "))

	if c.auth != nil {
		if err := c.auth.Authenticate(req); err != nil {
			return nil, fmt.Errorf("authentication: %w", err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decoding manifest: %w", err)
	}

	manifest.MediaType = resp.Header.Get("Content-Type")

	return &manifest, nil
}

// GetBlob retrieves a blob from the registry.
func (c *RegistryClient) GetBlob(ctx context.Context, repository, digest string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/v2/%s/blobs/%s", c.registry, repository, digest)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setHeaders(req)

	if c.auth != nil {
		if err := c.auth.Authenticate(req); err != nil {
			return nil, fmt.Errorf("authentication: %w", err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}

func (c *RegistryClient) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.userAgent)
}

// parseLinkHeader parses the Link header for pagination.
func parseLinkHeader(link, baseURL string) string {
	// Link header format: </v2/repo/tags/list?n=10&last=tag>; rel="next"
	parts := strings.Split(link, ";")
	if len(parts) < 2 {
		return ""
	}

	// Check if it's a "next" link
	for _, part := range parts[1:] {
		if strings.Contains(part, `rel="next"`) {
			urlPart := strings.Trim(parts[0], " <>")
			if strings.HasPrefix(urlPart, "/") {
				return baseURL + urlPart
			}
			return urlPart
		}
	}

	return ""
}

// Error types
var (
	ErrNotFound     = fmt.Errorf("not found")
	ErrUnauthorized = fmt.Errorf("unauthorized")
)

// IsNotFound returns true if the error is a not found error.
func IsNotFound(err error) bool {
	return err == ErrNotFound
}

// Logger interface for the client
func init() {
	// Ensure klog is initialized
	_ = klog.V(4)
}
