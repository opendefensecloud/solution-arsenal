// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	solarclient "go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/discovery/apiwriter"
	"go.opendefense.cloud/solar/pkg/discovery/handler"
	"go.opendefense.cloud/solar/pkg/discovery/qualifier"
	"go.opendefense.cloud/solar/pkg/discovery/scanner"
	"go.opendefense.cloud/solar/pkg/discovery/verifier"
	"go.opendefense.cloud/solar/pkg/discovery/webhook"
)

type Pipeline struct {
	regScanners   []*scanner.RegistryScanner
	webhookServer *webhook.WebhookServer
	qualifier     *qualifier.Qualifier
	filter        *handler.Filter
	verifier      *verifier.Verifier
	handler       *handler.Handler
	writer        *apiwriter.APIWriter
	errChan       chan<- discovery.ErrorEvent
	log           logr.Logger
}

// Option overrides pipeline components after construction (e.g. WithFilterProcessor).
type Option func(*Pipeline)

func NewPipeline(namespace string, registries *discovery.RegistryProvider, webhookLstnAddr string, errChan chan<- discovery.ErrorEvent, log logr.Logger, solarClient solarclient.SolarV1alpha1Interface, secretClient corev1client.CoreV1Interface, opts ...Option) (*Pipeline, error) {

	repoEvents := make(chan discovery.RepositoryEvent, 1000)
	filterInput := make(chan discovery.ComponentVersionEvent, 1000)
	verifierInput := make(chan discovery.ComponentVersionEvent, 1000)
	handlerInput := make(chan discovery.ComponentVersionEvent, 1000)
	writerInput := make(chan discovery.WriteAPIResourceEvent, 1000)

	var httpRouter *webhook.WebhookRouter

	var regScanners []*scanner.RegistryScanner
	for _, registry := range registries.GetAll() {
		if registry.Spec.WebhookPath != "" {
			if httpRouter == nil {
				httpRouter = webhook.NewWebhookRouter(repoEvents)
				httpRouter.WithLogger(log)
			}
			if err := httpRouter.RegisterPath(registry); err != nil {
				return nil, fmt.Errorf("failed to register handler: %w", err)
			}
		}

		if registry.Spec.ScanInterval != nil && registry.Spec.ScanInterval.Duration > 0 {
			creds := registries.GetCredentials(registry.Name)
			s := scanner.NewRegistryScanner(registry, creds, repoEvents, errChan,
				scanner.WithScanInterval(registry.Spec.ScanInterval.Duration),
				scanner.WithLogger(log),
			)
			regScanners = append(regScanners, s)
		}
	}

	var webhookServer *webhook.WebhookServer
	if httpRouter != nil {
		webhookServer = webhook.NewWebhookServer(webhookLstnAddr, httpRouter, errChan, log)
	}

	p := &Pipeline{
		regScanners:   regScanners,
		webhookServer: webhookServer,
		errChan:       errChan,
		log:           log,
	}

	p.qualifier = qualifier.NewQualifier(registries, namespace, repoEvents, filterInput, errChan, discovery.WithLogger[discovery.RepositoryEvent, discovery.ComponentVersionEvent](log))

	p.filter = handler.NewFilter(solarClient, namespace, filterInput, verifierInput, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent](log))

	p.verifier = verifier.NewVerifier(registries, secretClient, namespace, verifierInput, handlerInput, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent](log))

	p.handler = handler.NewHandler(registries, handlerInput, writerInput, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](log), discovery.WithRateLimiter[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](time.Second, 1))

	p.writer = apiwriter.NewAPIWriter(solarClient, namespace, registries, writerInput, errChan, discovery.WithLogger[discovery.WriteAPIResourceEvent, any](log))

	for _, opt := range opts {
		opt(p)
	}

	return p, nil

}

func (p *Pipeline) Start(ctx context.Context) (err error) {

	defer func() {
		if err != nil {
			stopErr := p.Stop(ctx)
			err = fmt.Errorf("failed to start pipeline: %w", errors.Join(err, stopErr))
		}
	}()

	if p.webhookServer != nil {
		if err = p.webhookServer.Start(ctx); err != nil {
			return err
		}
	}

	for _, scanner := range p.regScanners {
		if err = scanner.Start(ctx); err != nil {
			return err
		}
	}
	if err = p.qualifier.Start(ctx); err != nil {
		return err
	}
	if err = p.filter.Start(ctx); err != nil {
		return err
	}
	if err = p.verifier.Start(ctx); err != nil {
		return err
	}
	if err = p.handler.Start(ctx); err != nil {
		return err
	}
	if err = p.writer.Start(ctx); err != nil {
		return err
	}

	return nil
}

func (p *Pipeline) Stop(ctx context.Context) error {
	var err error
	if p.webhookServer != nil {
		err = p.webhookServer.Stop(ctx)
	}
	for _, scanner := range p.regScanners {
		scanner.Stop()
	}
	p.qualifier.Stop()
	p.filter.Stop()
	p.verifier.Stop()
	p.handler.Stop()
	p.writer.Stop()

	return err
}

func WithScanner(s scanner.Scanner) Option {
	return func(p *Pipeline) {
		if len(p.regScanners) > 0 {
			p.regScanners[0].Scanner = s
		}
	}
}

func WithQualifierProcessor(proc discovery.Processor[discovery.RepositoryEvent, discovery.ComponentVersionEvent]) Option {
	return func(p *Pipeline) {
		p.qualifier.Runner.Processor = proc
	}
}

func WithFilterProcessor(proc discovery.Processor[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent]) Option {
	return func(p *Pipeline) {
		p.filter.Runner.Processor = proc
	}
}

func WithHandlerProcessor(proc discovery.Processor[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent]) Option {
	return func(p *Pipeline) {
		p.handler.Runner.Processor = proc
	}
}

func WithVerifierProcessor(proc discovery.Processor[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent]) Option {
	return func(p *Pipeline) {
		p.verifier.Runner.Processor = proc
	}
}

func WithWriterProcessor(proc discovery.Processor[discovery.WriteAPIResourceEvent, any]) Option {
	return func(p *Pipeline) {
		p.writer.Runner.Processor = proc
	}
}
