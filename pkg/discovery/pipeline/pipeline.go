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
	"go.opendefense.cloud/solar/pkg/discovery/apiwriter"
	"go.opendefense.cloud/solar/pkg/discovery/handler"
	"go.opendefense.cloud/solar/pkg/discovery/qualifier"
	"go.opendefense.cloud/solar/pkg/discovery/scanner"
	"go.opendefense.cloud/solar/pkg/discovery/webhook"
)

type Pipeline struct {
	regScanners   []scanner.Scanner
	webhookServer *webhook.WebhookServer
	router        *webhook.WebhookRouter
	qualifier     *qualifier.Qualifier
	filter        *handler.Filter
	handler       *handler.Handler
	writer        *apiwriter.APIWriter
	errChan       chan<- discovery.ErrorEvent
	log           logr.Logger
}

type Option func(*Pipeline)

func NewPipeline(namespace string, registries *discovery.RegistryProvider, webhookLstnAddr string, errChan chan<- discovery.ErrorEvent, log logr.Logger, opts ...Option) (*Pipeline, error) {

	repoEvents := make(chan discovery.RepositoryEvent, 1000)
	filterInput := make(chan discovery.ComponentVersionEvent, 1000)
	handlerInput := make(chan discovery.ComponentVersionEvent, 1000)
	writerInput := make(chan discovery.WriteAPIResourceEvent, 1000)

	clientset := solarclient.NewForConfigOrDie(config.GetConfigOrDie())

	qualifier := qualifier.NewQualifier(registries, namespace, repoEvents, filterInput, errChan, discovery.WithLogger[discovery.RepositoryEvent, discovery.ComponentVersionEvent](log))
	filter := handler.NewFilter(clientset, namespace, filterInput, handlerInput, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent](log))
	handler := handler.NewHandler(registries, handlerInput, writerInput, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](log), discovery.WithRateLimiter[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](time.Second, 1))
	writer := apiwriter.NewAPIWriter(clientset, namespace, registries, writerInput, errChan, discovery.WithLogger[discovery.WriteAPIResourceEvent, any](log))

	router := webhook.NewWebhookRouter(repoEvents)
	router.WithLogger(log)

	p := &Pipeline{
		regScanners: make([]scanner.Scanner, 0),
		router:      router,
		qualifier:   qualifier,
		filter:      filter,
		handler:     handler,
		writer:      writer,
		errChan:     errChan,
		log:         log,
	}

	for _, opt := range opts {
		// WithHandler
		opt(p)
	}

	var initWebserver bool
	for _, registry := range registries.GetAll() {
		if registry.WebhookPath != "" {
			if err := router.RegisterPath(registry); err != nil {
				return nil, fmt.Errorf("failed to register handler: %w", err)
			}

			initWebserver = true
		}

		if registry.ScanInterval > 0 {
			p.addScanner(scanner.NewRegistryScanner(
				registry, repoEvents, errChan,
				scanner.WithScanInterval(registry.ScanInterval),
				scanner.WithLogger(log),
			))
		}
	}

	var webhookServer *webhook.WebhookServer
	if initWebserver {
		webhookServer = webhook.NewWebhookServer(webhookLstnAddr, router, errChan, log)
	}

	p.webhookServer = webhookServer

	return p, nil
}

func (p *Pipeline) addScanner(scanner scanner.Scanner) {
	p.regScanners = append(p.regScanners, scanner)
}

func (p *Pipeline) setScanner(s scanner.Scanner) {
	p.regScanners = []scanner.Scanner{s}
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

	for _, scan := range p.regScanners {
		if err = scan.Start(ctx); err != nil {
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
	if err = p.writer.Start(ctx); err != nil {
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
	p.writer.Stop()
}

// WithScanner overwrites existing scanners and replaces them
// with the given s [scanner.Scanner]
func WithScanner(s scanner.Scanner) Option {
	return func(p *Pipeline) {
		p.setScanner(s)
	}
}

func WithWebhookHandler(name string, initFn webhook.InitHandlerFunc) Option {
	return func(p *Pipeline) {
		p.router.RegisterHandler(name, initFn)
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

func WithWriterProcessor(proc discovery.Processor[discovery.WriteAPIResourceEvent, any]) Option {
	return func(p *Pipeline) {
		p.writer.Runner.Processor = proc
	}
}
