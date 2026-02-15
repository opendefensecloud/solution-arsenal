// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"fmt"
	"net/http"
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
	ctx           context.Context
	qualifier     *qualifier.Qualifier
	filter        *handler.Filter
	handler       *handler.Handler
	regScanners   []*scanner.RegistryScanner
	webhookServer *http.Server
	log           logr.Logger
	errChan       chan discovery.ErrorEvent
}

type Option func(*Pipeline)

func NewPipeline(ctx context.Context, log logr.Logger, namespace string, registries *discovery.RegistryProvider, webhookLstnAddr string, opts ...Option) (*Pipeline, error) {

	repoEventsChan := make(chan discovery.RepositoryEvent, 1000)
	cvChanFilterInput := make(chan discovery.ComponentVersionEvent, 1000)
	cvChanHandlerInput := make(chan discovery.ComponentVersionEvent, 1000)
	errChan := make(chan discovery.ErrorEvent)

	var httpRouter *webhook.WebhookRouter

	var regScanners []*scanner.RegistryScanner
	for _, registry := range registries.GetAll() {
		if registry.WebhookPath != "" {
			if httpRouter == nil {
				httpRouter = webhook.NewWebhookRouter(repoEventsChan)
				httpRouter.WithLogger(log)
			}
			if err := httpRouter.RegisterPath(registry); err != nil {
				return nil, fmt.Errorf("failed to register handler: %w", err)
			}
		}

		if registry.ScanInterval > 0 {
			scanner := scanner.NewRegistryScanner(registry, repoEventsChan, errChan,
				scanner.WithScanInterval(registry.ScanInterval),
				scanner.WithLogger(log),
			)
			regScanners = append(regScanners, scanner)
		}
	}

	var webhookServer *http.Server

	if httpRouter != nil {
		webhookServer = &http.Server{
			Addr:              webhookLstnAddr,
			Handler:           httpRouter,
			ReadHeaderTimeout: time.Second * 3,
		}
	}

	clientset := solarclient.NewForConfigOrDie(config.GetConfigOrDie())
	filter := handler.NewFilter(clientset, namespace, cvChanFilterInput, cvChanHandlerInput, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent](log))
	// FIXME: Should send the output to the next handler to actually write component versions to cluster.
	handler := handler.NewHandler(registries, cvChanHandlerInput, nil, errChan, discovery.WithLogger[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](log), discovery.WithRateLimiter[discovery.ComponentVersionEvent, discovery.WriteAPIResourceEvent](time.Second, 1))

	qualifier := qualifier.NewQualifier(registries, namespace, repoEventsChan, cvChanFilterInput, errChan, discovery.WithLogger[discovery.RepositoryEvent, discovery.ComponentVersionEvent](log))

	p := &Pipeline{
		ctx:           ctx,
		filter:        filter,
		handler:       handler,
		qualifier:     qualifier,
		regScanners:   regScanners,
		webhookServer: webhookServer,
		log:           log,
		errChan:       errChan,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil

}

func (p *Pipeline) Start() error {

	p.startWebhookServer()

	for _, scanner := range p.regScanners {
		if err := scanner.Start(p.ctx); err != nil {
			return err
		}

	}
	if err := p.qualifier.Start(p.ctx); err != nil {
		return err
	}
	if err := p.filter.Start(p.ctx); err != nil {
		return err
	}
	if err := p.handler.Start(p.ctx); err != nil {
		return err
	}

	return nil
}

func (p *Pipeline) Stop() {

	p.stopWebhookServer()

	for _, scanner := range p.regScanners {
		scanner.Stop()
	}
	p.qualifier.Stop()
	p.filter.Stop()
	p.handler.Stop()
}

// FIXME move starting and stopping the webook server out of here
func (p *Pipeline) startWebhookServer() {
	if p.webhookServer == nil {
		return
	}

	go func() {
		if err := p.webhookServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			discovery.Publish(&p.log, p.errChan, discovery.ErrorEvent{
				Error:     err,
				Timestamp: time.Now().UTC(),
			})
		}
	}()
}

// FIXME move starting and stopping the webook server out of here
func (p *Pipeline) stopWebhookServer() {
	if p.webhookServer == nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	if err := p.webhookServer.Shutdown(shutdownCtx); err != nil {
		p.log.Error(err, "Warning: server shutdown error")
	}
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
