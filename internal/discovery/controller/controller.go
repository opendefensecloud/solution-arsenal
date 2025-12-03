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

// Package controller implements the discovery controller that scans OCI registries
// for OCM components and creates CatalogItems in the solar-index API.
package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/opendefensecloud/solution-arsenal/pkg/ocm"
	"github.com/opendefensecloud/solution-arsenal/pkg/registry/oci"
)

// Controller is the discovery controller that scans registries for OCM components.
type Controller struct {
	mu sync.RWMutex

	// registries is the list of registries to scan
	registries []RegistryConfig

	// catalogStore is used to create/update catalog items
	catalogStore CatalogStore

	// scanInterval is the interval between scans
	scanInterval time.Duration

	// concurrency is the number of concurrent scans
	concurrency int

	// stopCh is used to stop the controller
	stopCh chan struct{}

	// running indicates if the controller is running
	running bool

	// metrics tracks controller metrics
	metrics *Metrics
}

// RegistryConfig configures a registry to scan.
type RegistryConfig struct {
	// Name is a friendly name for the registry
	Name string
	// URL is the registry URL
	URL string
	// Repositories is the list of repositories to scan (optional, scans all if empty)
	Repositories []string
	// Auth is the authenticator for the registry (optional)
	Auth oci.Authenticator
	// Labels are default labels to apply to discovered items
	Labels map[string]string
	// Namespace is the namespace for created CatalogItems (empty for cluster-scoped)
	Namespace string
}

// CatalogStore is the interface for storing discovered catalog items.
type CatalogStore interface {
	// CreateOrUpdate creates or updates a catalog item.
	CreateOrUpdate(ctx context.Context, item *DiscoveredItem) error
	// Get gets a catalog item by component name and version.
	Get(ctx context.Context, componentName, version string) (*DiscoveredItem, error)
	// List lists all catalog items.
	List(ctx context.Context) ([]*DiscoveredItem, error)
}

// DiscoveredItem represents a discovered OCM component.
type DiscoveredItem struct {
	// ComponentName is the OCM component name
	ComponentName string
	// Version is the component version
	Version string
	// Repository is the OCI repository
	Repository string
	// Registry is the registry URL
	Registry string
	// Provider is the component provider
	Provider string
	// Description is the component description
	Description string
	// Labels are the component labels
	Labels map[string]string
	// Resources is the list of resources in the component
	Resources []ResourceInfo
	// Dependencies is the list of component dependencies
	Dependencies []DependencyInfo
	// DiscoveredAt is when the component was discovered
	DiscoveredAt time.Time
	// Digest is the manifest digest
	Digest string
	// Namespace is the namespace for the CatalogItem (empty for cluster-scoped)
	Namespace string
}

// ResourceInfo contains information about a component resource.
type ResourceInfo struct {
	Name    string
	Type    string
	Version string
}

// DependencyInfo contains information about a component dependency.
type DependencyInfo struct {
	Name    string
	Version string
}

// Option configures the controller.
type Option func(*Controller)

// WithScanInterval sets the scan interval.
func WithScanInterval(interval time.Duration) Option {
	return func(c *Controller) {
		c.scanInterval = interval
	}
}

// WithConcurrency sets the number of concurrent scans.
func WithConcurrency(n int) Option {
	return func(c *Controller) {
		c.concurrency = n
	}
}

// WithMetrics sets the metrics collector.
func WithMetrics(m *Metrics) Option {
	return func(c *Controller) {
		c.metrics = m
	}
}

// NewController creates a new discovery controller.
func NewController(registries []RegistryConfig, store CatalogStore, opts ...Option) *Controller {
	c := &Controller{
		registries:   registries,
		catalogStore: store,
		scanInterval: 5 * time.Minute,
		concurrency:  5,
		stopCh:       make(chan struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.metrics == nil {
		c.metrics = NewMetrics()
	}

	return c
}

// Start starts the controller.
func (c *Controller) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("controller already running")
	}
	c.running = true
	c.stopCh = make(chan struct{})
	c.mu.Unlock()

	klog.InfoS("Starting discovery controller", "registries", len(c.registries), "interval", c.scanInterval)

	// Run initial scan
	c.runScan(ctx)

	// Start periodic scan
	ticker := time.NewTicker(c.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.InfoS("Discovery controller stopped by context")
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
			return ctx.Err()

		case <-c.stopCh:
			klog.InfoS("Discovery controller stopped")
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
			return nil

		case <-ticker.C:
			c.runScan(ctx)
		}
	}
}

// Stop stops the controller.
func (c *Controller) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		close(c.stopCh)
	}
}

// IsRunning returns true if the controller is running.
func (c *Controller) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// ScanNow triggers an immediate scan.
func (c *Controller) ScanNow(ctx context.Context) error {
	c.runScan(ctx)
	return nil
}

// runScan performs a scan of all registries.
func (c *Controller) runScan(ctx context.Context) {
	startTime := time.Now()
	klog.V(2).InfoS("Starting registry scan", "registries", len(c.registries))

	// Create a work channel
	type scanWork struct {
		registry   RegistryConfig
		repository string
	}
	workCh := make(chan scanWork, 100)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < c.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for work := range workCh {
				c.scanRepository(ctx, work.registry, work.repository)
			}
		}(i)
	}

	// Queue work
	for _, reg := range c.registries {
		if len(reg.Repositories) > 0 {
			// Scan specific repositories
			for _, repo := range reg.Repositories {
				select {
				case workCh <- scanWork{registry: reg, repository: repo}:
				case <-ctx.Done():
					close(workCh)
					wg.Wait()
					return
				}
			}
		} else {
			// Single repository scan (registry URL is the repository)
			select {
			case workCh <- scanWork{registry: reg, repository: ""}:
			case <-ctx.Done():
				close(workCh)
				wg.Wait()
				return
			}
		}
	}

	close(workCh)
	wg.Wait()

	duration := time.Since(startTime)
	klog.V(2).InfoS("Registry scan completed", "duration", duration)
	c.metrics.RecordScan(duration)
}

// scanRepository scans a single repository for OCM components.
func (c *Controller) scanRepository(ctx context.Context, reg RegistryConfig, repository string) {
	logger := klog.FromContext(ctx)

	// Create OCI client
	clientOpts := []oci.ClientOption{}
	if reg.Auth != nil {
		clientOpts = append(clientOpts, oci.WithAuthenticator(reg.Auth))
	}
	client := oci.NewClient(reg.URL, clientOpts...)

	// Determine repository path
	repoPath := repository
	if repoPath == "" {
		// Extract repository from URL if not specified
		repoPath = extractRepository(reg.URL)
	}

	if repoPath == "" {
		logger.V(4).Info("No repository path, skipping", "registry", reg.Name)
		return
	}

	// Create OCM parser
	parser := ocm.NewParser(client)

	// List tags/versions
	versions, err := parser.ListComponents(ctx, repoPath)
	if err != nil {
		logger.Error(err, "Failed to list components", "registry", reg.Name, "repository", repoPath)
		c.metrics.RecordError("list_components")
		return
	}

	logger.V(4).Info("Found versions", "registry", reg.Name, "repository", repoPath, "count", len(versions))

	// Parse each version
	for _, version := range versions {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, err := parser.ParseComponent(ctx, repoPath, version)
		if err != nil {
			logger.V(4).Info("Failed to parse component", "repository", repoPath, "version", version, "error", err)
			c.metrics.RecordError("parse_component")
			continue
		}

		// Validate the descriptor
		if err := result.Descriptor.Validate(); err != nil {
			logger.V(4).Info("Invalid component descriptor", "repository", repoPath, "version", version, "error", err)
			c.metrics.RecordError("invalid_descriptor")
			continue
		}

		// Create discovered item
		item := c.createDiscoveredItem(reg, result)

		// Store the item
		if err := c.catalogStore.CreateOrUpdate(ctx, item); err != nil {
			logger.Error(err, "Failed to store catalog item",
				"component", item.ComponentName, "version", item.Version)
			c.metrics.RecordError("store_item")
			continue
		}

		logger.V(4).Info("Discovered component",
			"component", item.ComponentName,
			"version", item.Version,
			"resources", len(item.Resources))
		c.metrics.RecordDiscovery()
	}
}

// createDiscoveredItem creates a DiscoveredItem from a parse result.
func (c *Controller) createDiscoveredItem(reg RegistryConfig, result *ocm.ParseResult) *DiscoveredItem {
	cd := result.Descriptor

	// Extract resources
	resources := make([]ResourceInfo, 0, len(cd.Component.Resources))
	for _, r := range cd.Component.Resources {
		resources = append(resources, ResourceInfo{
			Name:    r.Name,
			Type:    r.Type,
			Version: r.Version,
		})
	}

	// Extract dependencies
	deps := make([]DependencyInfo, 0, len(cd.Component.References))
	for _, ref := range cd.Component.References {
		deps = append(deps, DependencyInfo{
			Name:    ref.ComponentName,
			Version: ref.Version,
		})
	}

	// Merge labels
	labels := make(map[string]string)
	for k, v := range reg.Labels {
		labels[k] = v
	}
	for k, v := range cd.Component.Labels.ToMap() {
		labels[k] = v
	}

	// Get description from labels
	description, _ := cd.Component.Labels.GetString("description")

	return &DiscoveredItem{
		ComponentName: cd.Component.Name,
		Version:       cd.Component.Version,
		Repository:    result.Repository,
		Registry:      reg.URL,
		Provider:      cd.Component.Provider.Name,
		Description:   description,
		Labels:        labels,
		Resources:     resources,
		Dependencies:  deps,
		DiscoveredAt:  time.Now(),
		Digest:        result.Digest,
		Namespace:     reg.Namespace,
	}
}

// extractRepository extracts the repository path from a URL.
func extractRepository(url string) string {
	// This is a simplified implementation
	// In production, you'd want more sophisticated URL parsing
	return ""
}

// Metrics tracks discovery controller metrics.
type Metrics struct {
	mu             sync.Mutex
	totalScans     int64
	totalErrors    int64
	totalDiscovered int64
	lastScanDuration time.Duration
	lastScanTime   time.Time
}

// NewMetrics creates new metrics.
func NewMetrics() *Metrics {
	return &Metrics{}
}

// RecordScan records a scan completion.
func (m *Metrics) RecordScan(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalScans++
	m.lastScanDuration = duration
	m.lastScanTime = time.Now()
}

// RecordError records an error.
func (m *Metrics) RecordError(errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalErrors++
}

// RecordDiscovery records a discovered component.
func (m *Metrics) RecordDiscovery() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalDiscovered++
}

// GetStats returns current statistics.
func (m *Metrics) GetStats() (scans, errors, discovered int64, lastDuration time.Duration, lastTime time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalScans, m.totalErrors, m.totalDiscovered, m.lastScanDuration, m.lastScanTime
}
