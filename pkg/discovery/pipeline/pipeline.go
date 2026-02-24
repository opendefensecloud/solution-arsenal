// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	solarclient "go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/discovery/handler"
	"go.opendefense.cloud/solar/pkg/discovery/qualifier"
	"go.opendefense.cloud/solar/pkg/discovery/scanner"
	"go.opendefense.cloud/solar/pkg/discovery/webhook"
)

type Pipeline struct {
	regScanners   []*scanner.RegistryScanner
	webhookServer *webhook.WebhookServer
	qualifier     *qualifier.Qualifier
	filter        *handler.Filter
	handler       *handler.Handler
	errChan       chan<- discovery.ErrorEvent
	log           logr.Logger
}

type Option func(*Pipeline)

func NewPipeline(namespace string, registries *discovery.RegistryProvider, webhookLstnAddr string, errChan chan<- discovery.ErrorEvent, log logr.Logger, opts ...Option) (*Pipeline, error) {

	repoEvents := make(chan discovery.RepositoryEvent, 1000)
	filterInput := make(chan discovery.ComponentVersionEvent, 1000)
	handlerInput := make(chan discovery.ComponentVersionEvent, 1000)

	var httpRouter *webhook.WebhookRouter

	var regScanners []*scanner.RegistryScanner
	for _, registry := range registries.GetAll() {
		if registry.WebhookPath != "" {
			if httpRouter == nil {
				httpRouter = webhook.NewWebhookRouter(repoEvents)
				httpRouter.WithLogger(log)
			}
			if err := httpRouter.RegisterPath(registry); err != nil {
				return nil, fmt.Errorf("failed to register handler: %w", err)
			}
		}

		if registry.ScanInterval > 0 {
			scanner := scanner.NewRegistryScanner(registry, repoEvents, errChan,
				scanner.WithScanInterval(registry.ScanInterval),
				scanner.WithLogger(log),
			)
			regScanners = append(regScanners, scanner)
		}
	}

	var webhookServer *webhook.WebhookServer
	if httpRouter != nil {
		webhookServer = webhook.NewWebhookServer(webhookLstnAddr, httpRouter, errChan, log)
	}

	qualifier := qualifier.NewQualifier(registries, namespace, repoEvents, filterInput, errChan, discovery.WithLogger[discovery.RepositoryEvent, discovery.ComponentVersionEvent](log))

	clientset := solarclient.NewForConfigOrDie(config.GetConfigOrDie())
	filter := handler.NewFilter(clientset, namespace, filterInput, handlerInput, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent](log))

	// FIXME: Should send the output to the next handler to actually write component versions to cluster.
	handler := handler.NewHandler(registries, handlerInput, nil, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](log), discovery.WithRateLimiter[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](time.Second, 1))

	p := &Pipeline{
		regScanners:   regScanners,
		webhookServer: webhookServer,
		qualifier:     qualifier,
		filter:        filter,
		handler:       handler,
		errChan:       errChan,
		log:           log,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil

}

func (p *Pipeline) Start(ctx context.Context) (err error) {

	defer func() {
		if err != nil {
			p.Stop(ctx)
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
	if err = p.handler.Start(ctx); err != nil {
		return err
	}

	return nil
}

func (p *Pipeline) Stop(ctx context.Context) {

	if p.webhookServer != nil {
		p.webhookServer.Stop(ctx)
	}

	for _, scanner := range p.regScanners {
		scanner.Stop()
	}
	p.qualifier.Stop()
	p.filter.Stop()
	p.handler.Stop()
}

func WithScanner(s scanner.Scanner) Option {
	return func(p *Pipeline) {
		p.regScanners[0].Scanner = s
	}
}

func WithQualifierProcessor[InputEvent any, OutputEvent any](proc discovery.Processor[discovery.RepositoryEvent, discovery.ComponentVersionEvent]) Option {
	return func(p *Pipeline) {
		p.qualifier.Runner.Processor = proc
	}
}

func WithFilterProcessor[InputEvent any, OutputEvent any](proc discovery.Processor[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent]) Option {
	return func(p *Pipeline) {
		p.filter.Runner.Processor = proc
	}
}

func WithHandlerProcessor[InputEvent any, OutputEvent any](proc discovery.Processor[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent]) Option {
	return func(p *Pipeline) {
		p.handler.Runner.Processor = proc
	}
}
