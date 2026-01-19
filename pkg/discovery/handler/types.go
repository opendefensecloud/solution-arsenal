// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type HandlerType string

const (
	HelmHandler HandlerType = "helm"
	KroHandler  HandlerType = "kro"
)

type ComponentHandler interface {
	ProcessEvent(ctx context.Context, ev discovery.ComponentVersionEvent)
}
