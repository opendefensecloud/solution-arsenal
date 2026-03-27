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
	"go.opendefense.cloud/solar/pkg/discovery/webhook/generic"
	"go.opendefense.cloud/solar/pkg/discovery/webhook/zot"
)

type Pipeline struct {
	httpRouter    *webhook.WebhookRouter
	regScanners   []*scanner.RegistryScanner
	webhookServer *webhook.WebhookServer
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

	var httpRouter *webhook.WebhookRouter

	var regScanners []*scanner.RegistryScanner
	for _, registry := range registries.GetAll() {
		if registry.WebhookPath != "" {
			if httpRouter == nil {
				httpRouter = webhook.NewWebhookRouter(repoEvents)
				httpRouter.RegisterHandler("generic", generic.NewHandler)
				httpRouter.RegisterHandler("zot", zot.NewHandler)
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

	filter := handler.NewFilter(clientset, namespace, filterInput, handlerInput, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent](log))

	handler := handler.NewHandler(registries, handlerInput, writerInput, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](log), discovery.WithRateLimiter[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](time.Second, 1))

	writer := apiwriter.NewAPIWriter(clientset, namespace, registries, writerInput, errChan, discovery.WithLogger[discovery.WriteAPIResourceEvent, any](log))

	p := &Pipeline{
		httpRouter:    httpRouter,
		regScanners:   regScanners,
		webhookServer: webhookServer,
		qualifier:     qualifier,
		filter:        filter,
		handler:       handler,
		writer:        writer,
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

func withZotWebhookHandler(fn webhook.InitHandlerFunc, reg *discovery.Registry) Option {
	return func(p *Pipeline) {
		p.httpRouter.UnregisterHandler("zot")
		p.httpRouter.UnregisterPath(reg)
		p.httpRouter.RegisterHandler("zot", fn)
		err := p.httpRouter.RegisterPath(reg)
		if err != nil {
			panic(fmt.Sprintf("failed to register zot path: %v", err))
		}
		p.webhookServer = webhook.NewWebhookServer(p.webhookServer.Addr, p.httpRouter, p.errChan, p.log)
	}
}

func withScanner(s scanner.Scanner) Option {
	return func(p *Pipeline) {
		p.regScanners[0].Scanner = s
	}
}

func withQualifierProcessor(proc discovery.Processor[discovery.RepositoryEvent, discovery.ComponentVersionEvent]) Option {
	return func(p *Pipeline) {
		p.qualifier.Runner.Processor = proc
	}
}

func withFilterProcessor(proc discovery.Processor[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent]) Option {
	return func(p *Pipeline) {
		p.filter.Runner.Processor = proc
	}
}

func withHandlerProcessor(proc discovery.Processor[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent]) Option {
	return func(p *Pipeline) {
		p.handler.Runner.Processor = proc
	}
}

func withWriterProcessor(proc discovery.Processor[discovery.WriteAPIResourceEvent, any]) Option {
	return func(p *Pipeline) {
		p.writer.Runner.Processor = proc
	}
}
