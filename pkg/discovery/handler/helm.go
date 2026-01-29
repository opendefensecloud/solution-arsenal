// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"

	"github.com/go-logr/logr"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type helmHandler struct {
}

func init() {
	handlerRegistry[HelmHandler] = &helmHandler{}
}

func (h *helmHandler) ProcessEvent(ctx context.Context, ev discovery.ComponentVersionEvent) {
	// TODO: Implement actual processing
	logr.FromContextOrDiscard(ctx).Info("Processing Helm event", "event", ev)
}
