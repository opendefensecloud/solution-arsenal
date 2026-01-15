// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type helmHandler struct {
}

func init() {
	handlerRegistry[HELM_HANDLER] = &helmHandler{}
}

func (h *helmHandler) ProcessEvent(ctx context.Context, ev discovery.ComponentVersionEvent) {
	// TODO: Implement actual processing
	fmt.Println("Processing Helm event:", ev)
}
