// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"go.opendefense.cloud/solar/pkg/discovery"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// RegistryCredentials contains credentials for authenticating with an OCI registry.
type RegistryCredentials struct {
	Username string
	Password string
}

// RegistryScanner continuously scans an OCI registry and sends discovery events
// to a channel. It uses ORAS to interact with the OCI registry.
type RegistryScanner struct {
	registryURL  string
	credentials  RegistryCredentials
	eventsChan   chan<- discovery.RegistryEvent
	logger       logr.Logger
	stopChan     chan struct{}
	wg           sync.WaitGroup
	scanInterval time.Duration
	stopped      bool
	stopMu       sync.Mutex
	plainHTTP    bool
}

// Option describes the available options
// for creating the RegistryScanner.
type Option func(r *RegistryScanner)

// NewRegistryScanner creates a new RegistryScanner that will scan the provided
// OCI registry with the given credentials. Events will be sent to the provided channel.
// The logger is used for logging scanner activity.
func NewRegistryScanner(registryURL string, eventsChan chan<- discovery.RegistryEvent, opts ...Option) *RegistryScanner {
	r := &RegistryScanner{
		registryURL:  registryURL,
		eventsChan:   eventsChan,
		stopChan:     make(chan struct{}),
		logger:       logr.Discard(),
		scanInterval: 30 * time.Second, // Default scan interval
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func WithScanInterval(d time.Duration) Option {
	return func(r *RegistryScanner) {
		r.scanInterval = d
	}
}

func WithLogger(l logr.Logger) Option {
	return func(r *RegistryScanner) {
		r.logger = l
	}
}

func WithPlainHTTP() Option {
	return func(r *RegistryScanner) {
		r.plainHTTP = true
	}
}

func WithCredentials(creds RegistryCredentials) Option {
	return func(r *RegistryScanner) {
		r.credentials = creds
	}
}

// SetScanInterval sets the interval between registry scans.
func (rs *RegistryScanner) SetScanInterval(interval time.Duration) {
	rs.scanInterval = interval
}

// Start begins continuous scanning of the registry in a separate goroutine.
// The scanner will continue until Stop() is called.
func (rs *RegistryScanner) Start(ctx context.Context) error {
	rs.logger.Info("starting registry scanner", "registry", rs.registryURL)

	rs.wg.Add(1)
	go rs.scanLoop(ctx)

	return nil
}

// Stop gracefully stops the registry scanner.
func (rs *RegistryScanner) Stop() {
	rs.stopMu.Lock()
	defer rs.stopMu.Unlock()

	if rs.stopped {
		return
	}

	rs.logger.Info("stopping registry scanner")
	rs.stopped = true
	close(rs.stopChan)
	rs.wg.Wait()
	rs.logger.Info("registry scanner stopped")
}

// scanLoop continuously scans the registry and sends events to the channel.
func (rs *RegistryScanner) scanLoop(ctx context.Context) {
	defer rs.wg.Done()

	ticker := time.NewTicker(rs.scanInterval)
	defer ticker.Stop()

	// Perform initial scan immediately
	rs.scanRegistry(ctx)

	for {
		select {
		case <-rs.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			rs.scanRegistry(ctx)
		}
	}
}

// scanRegistry performs a single scan of the registry and sends discovered events.
func (rs *RegistryScanner) scanRegistry(ctx context.Context) {
	rs.logger.V(1).Info("scanning registry", "registry", rs.registryURL)

	// Create a registry client with credentials
	client, err := rs.createRegistryClient(ctx)
	if err != nil {
		rs.sendEvent(discovery.RegistryEvent{
			Timestamp: time.Now(),
			Error:     fmt.Errorf("failed to create registry client: %w", err),
		})
		rs.logger.Error(err, "failed to create registry client")
		return
	}

	// List all repositories in the registry
	repositories, err := rs.listRepositories(ctx, client)
	if err != nil {
		rs.sendEvent(discovery.RegistryEvent{
			Timestamp: time.Now(),
			Error:     fmt.Errorf("failed to list repositories: %w", err),
		})
		rs.logger.Error(err, "failed to list repositories")
		return
	}

	// For each repository, discover tags
	for _, repoName := range repositories {
		if err := rs.discoverTagsInRepository(ctx, client, repoName); err != nil {
			rs.logger.Error(err, "failed to discover tags in repository", "repository", repoName)
		}
	}
}

// createRegistryClient creates a registry client authenticated with the configured credentials.
func (rs *RegistryScanner) createRegistryClient(ctx context.Context) (*remote.Registry, error) {
	// Create the base registry
	reg, err := remote.NewRegistry(rs.registryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry: %w", err)
	}
	reg.PlainHTTP = rs.plainHTTP

	// Set up authentication if credentials are provided
	if rs.credentials.Username != "" && rs.credentials.Password != "" {
		authClient := &auth.Client{
			Client: &http.Client{},
			Credential: auth.StaticCredential(rs.registryURL, auth.Credential{
				Username: rs.credentials.Username,
				Password: rs.credentials.Password,
			}),
		}
		reg.Client = authClient
	}

	return reg, nil
}

// listRepositories lists all repositories in the registry.
func (rs *RegistryScanner) listRepositories(ctx context.Context, reg *remote.Registry) ([]string, error) {
	var repositories []string

	err := reg.Repositories(ctx, "", func(repos []string) error {
		repositories = append(repositories, repos...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	return repositories, nil
}

// discoverTagsInRepository discovers all tags in a given repository.
func (rs *RegistryScanner) discoverTagsInRepository(ctx context.Context, reg *remote.Registry, repoName string) error {
	repo, err := reg.Repository(ctx, repoName)
	if err != nil {
		return fmt.Errorf("failed to get repository %s: %w", repoName, err)
	}

	err = repo.Tags(ctx, "", func(tags []string) error {
		for _, tag := range tags {
			d, _, err := oras.Fetch(ctx, repo, fmt.Sprintf("%s/%s:%s", rs.registryURL, repoName, tag), oras.DefaultFetchOptions)
			if err != nil {
				rs.logger.Error(fmt.Errorf("failed to fetch manifest %s: %w", tag, err), "failed to fetch manifest", "tag", tag)
				continue
			}

			// Extract namespace and component name from repository
			namespace, component, err := splitRepository(repoName)
			if err != nil {
				rs.logger.V(2).Info("splitting string returned: %v", err)
				continue
			}

			// set schema for http or https based on plainHTTP flag
			schema := "https"
			if rs.plainHTTP {
				schema = "http"
			}

			// Send discovery event for each tag found in the repository
			event := discovery.RegistryEvent{
				Registry:   rs.registryURL,
				Repository: repoName,
				Namespace:  namespace,
				Component:  component,
				Schema:     schema,
				Tag:        tag,
				Digest:     d.Digest.String(),
				Timestamp:  time.Now(),
			}
			rs.sendEvent(event)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to discover tags in repository %s: %w", repoName, err)
	}

	return nil
}

// sendEvent sends an event to the event channel without blocking.
// If the channel is full, the event is dropped with a warning.
func (rs *RegistryScanner) sendEvent(event discovery.RegistryEvent) {
	select {
	case rs.eventsChan <- event:
	default:
		rs.logger.V(1).Info("event channel full, dropping event", "event", event)
	}
}

func splitRepository(repo string) (string, string, error) {
	parts := strings.Split(repo, "/component-descriptors/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repository is not a component descriptor: splitting '%s' at './component-descriptors/' returns %d parts, expected exactly 2", repo, len(parts))
	}
	return parts[0], parts[1], nil
}
