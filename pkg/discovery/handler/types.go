// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type HandlerType string

const (
	HELM_HANDLER HandlerType = "helm"
	KRO_HANDLER  HandlerType = "kro"
)

type ComponentHandler interface {
	ProcessEvent(ctx context.Context, ev discovery.ComponentVersionEvent)
}
