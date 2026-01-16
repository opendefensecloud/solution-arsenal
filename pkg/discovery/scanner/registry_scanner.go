// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"go.opendefense.cloud/solar/pkg/discovery"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// RegistryScanner continuously scans an OCI registry and sends discovery events
// to a channel. It uses ORAS to interact with the OCI registry.
type RegistryScanner struct {
	registryURL  string
	credentials  discovery.RegistryCredentials
	eventsChan   chan<- discovery.RepositoryEvent
	errChan      chan<- discovery.ErrorEvent
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
func NewRegistryScanner(registryURL string, eventsChan chan<- discovery.RepositoryEvent, errChan chan<- discovery.ErrorEvent, opts ...Option) *RegistryScanner {
	r := &RegistryScanner{
		registryURL:  registryURL,
		eventsChan:   eventsChan,
		errChan:      errChan,
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

func WithCredentials(creds discovery.RegistryCredentials) Option {
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
	client, err := rs.createRegistryClient()
	if err != nil {
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Error: fmt.Errorf("failed to create registry client: %w", err),
		})
		rs.logger.Error(err, "failed to create registry client")
		return
	}

	// List all repositories in the registry
	err = client.Repositories(ctx, "", func(repos []string) error {
		for _, repoName := range repos {
			_, _, err := discovery.SplitRepository(repoName)
			if err != nil {
				rs.logger.V(2).Info("splitting string returned: %v", err)
				continue
			}

			// Send discovery event for repo found in the registry
			event := discovery.RepositoryEvent{
				Registry: discovery.Registry{
					Hostname:    rs.registryURL,
					PlainHTTP:   rs.plainHTTP,
					Credentials: rs.credentials,
				},
				Repository: repoName,
				Type:       discovery.EVENT_CREATED,
			}
			discovery.Publish(&rs.logger, rs.eventsChan, event)
		}
		return nil
	})
	if err != nil {
		discovery.Publish(&rs.logger, rs.errChan, discovery.ErrorEvent{
			Error: fmt.Errorf("failed to list repositories: %w", err),
		})
		rs.logger.Error(err, "failed to list repositories")
		return
	}
}

// createRegistryClient creates a registry client authenticated with the configured credentials.
func (rs *RegistryScanner) createRegistryClient() (*remote.Registry, error) {
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
