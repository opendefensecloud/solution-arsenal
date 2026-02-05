// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"

	"github.com/go-logr/logr"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type helmHandler struct {
	logger logr.Logger
}

func init() {
	RegisterComponentHandler(HelmHandler, func(log logr.Logger) ComponentHandler {
		return &helmHandler{
			logger: log,
		}
	})
}

func (h *helmHandler) ProcessEvent(ctx context.Context, ev discovery.ComponentVersionEvent) {
	h.logger.Info("Processing Helm event", "event", ev)
}
