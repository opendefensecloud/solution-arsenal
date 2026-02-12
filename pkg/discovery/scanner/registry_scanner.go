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
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"

	"go.opendefense.cloud/solar/pkg/discovery"
)

// RegistryScanner continuously scans an OCI registry and sends discovery events
// to a channel. It uses ORAS to interact with the OCI registry.
type RegistryScanner struct {
	registry     *discovery.Registry
	eventsChan   chan<- discovery.RepositoryEvent
	errChan      chan<- discovery.ErrorEvent
	logger       logr.Logger
	stopChan     chan struct{}
	wg           sync.WaitGroup
	scanMutex    sync.Mutex
	scanInterval time.Duration
	stopped      bool
	stopMu       sync.Mutex
}

// Option describes the available options
// for creating the RegistryScanner.
type Option func(r *RegistryScanner)

// NewRegistryScanner creates a new RegistryScanner that will scan the provided
// OCI registry with the given credentials. Events will be sent to the provided channel.
// The logger is used for logging scanner activity.
func NewRegistryScanner(
	registry *discovery.Registry,
	eventsChan chan<- discovery.RepositoryEvent,
	errChan chan<- discovery.ErrorEvent,
	opts ...Option,
) *RegistryScanner {
	r := &RegistryScanner{
		registry:     registry,
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

// SetScanInterval sets the interval between registry scans.
func (rs *RegistryScanner) SetScanInterval(interval time.Duration) {
	rs.scanInterval = interval
}

// Start begins continuous scanning of the registry in a separate goroutine.
// The scanner will continue until Stop() is called.
func (rs *RegistryScanner) Start(ctx context.Context) error {
	rs.logger.Info("starting registry scanner",
		"registry", rs.registry.Name,
		"url", rs.registry.GetURL(),
		"interval", rs.scanInterval,
	)

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
			go rs.scanRegistry(ctx)
		}
	}
}

// scanRegistry performs a single scan of the registry and sends discovered events.
func (rs *RegistryScanner) scanRegistry(ctx context.Context) {
	if !rs.scanMutex.TryLock() {
		rs.logger.V(1).Info("skipping registry scan, already locked")
		return
	}
	defer rs.scanMutex.Unlock()

	rs.logger.V(1).Info("scanning registry", "registry", rs.registry.GetURL())

	// Create a registry client with credentials
	client, err := rs.createRegistryClient()
	if err != nil {
		rs.handleError(err, "failed to create registry client")
		return
	}

	// Lists all repositories in the registry
	err = client.Repositories(ctx, "", func(repos []string) error {
		for _, repo := range repos {
			if err := rs.processRepository(ctx, repo); err != nil {
				rs.handleError(err, "processRepository returned error", "repo", repo)
			}
		}

		return nil
	})

	if err != nil {
		rs.handleError(err, "failed to list repositories")
	}
}

func (rs *RegistryScanner) processRepository(_ context.Context, repoName string) error {
	if _, _, err := discovery.SplitRepository(repoName); err != nil {
		return err
	}

	// Send discovery event for repo found in the registry
	event := discovery.RepositoryEvent{
		Timestamp:  time.Now().UTC(),
		Registry:   rs.registry.Name,
		Repository: repoName,
		Type:       discovery.EventCreated,
	}

	discovery.Publish(&rs.logger, rs.eventsChan, event)

	return nil
}

// createRegistryClient creates a registry client authenticated with the configured credentials.
func (rs *RegistryScanner) createRegistryClient() (*remote.Registry, error) {
	// Create the base registry
	reg, err := remote.NewRegistry(rs.registry.Hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry: %w", err)
	}
	reg.PlainHTTP = rs.registry.PlainHTTP

	// Set up authentication if credentials are provided
	if rs.registry.Credentials != nil {
		authClient := &auth.Client{
			Client: http.DefaultClient,
			Credential: auth.StaticCredential(rs.registry.GetURL(), auth.Credential{
				Username: rs.registry.Credentials.Username,
				Password: rs.registry.Credentials.Password,
			}),
		}
		reg.Client = authClient
	}

	return reg, nil
}

func (r *RegistryScanner) handleError(err error, msg string, keysAndValues ...any) {
	discovery.Publish(&r.logger, r.errChan, discovery.ErrorEvent{
		Error:     err,
		Message:   msg,
		Timestamp: time.Now().UTC(),
	})
	r.logger.Error(err, msg, keysAndValues...)
}
