// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/go-logr/logr"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type WebhookServer struct {
	server  *http.Server
	Addr    string
	errChan chan<- discovery.ErrorEvent
	log     logr.Logger
}

func NewWebhookServer(webhookLstnAddr string, router http.Handler, errChan chan<- discovery.ErrorEvent, log logr.Logger) *WebhookServer {

	server := &http.Server{
		Addr:              webhookLstnAddr,
		Handler:           router,
		ReadHeaderTimeout: time.Second * 3,
	}

	return &WebhookServer{
		server:  server,
		Addr:    server.Addr,
		errChan: errChan,
		log:     log,
	}
}

func (s *WebhookServer) Start(ctx context.Context) error {
	lc := net.ListenConfig{}
	l, err := lc.Listen(ctx, "tcp", s.server.Addr)
	if err != nil {
		return err
	}
	// if the port was zero, the address needs to be updated to reflect the actual port that has been used
	s.Addr = l.Addr().String()

	s.log.Info("Starting webhook server", "addr", s.Addr)
	go func() {
		if err := s.server.Serve(l); err != nil && err != http.ErrServerClosed {
			discovery.Publish(&s.log, s.errChan, discovery.ErrorEvent{
				Error:     err,
				Timestamp: time.Now().UTC(),
			})
		}
	}()

	return nil
}

func (s *WebhookServer) Stop(ctx context.Context) {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		s.log.Error(err, "Warning: server shutdown error")
	}
}
