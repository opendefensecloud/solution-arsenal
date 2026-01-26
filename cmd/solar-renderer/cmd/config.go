// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import "go.opendefense.cloud/solar/pkg/renderer"

type RendererConfig struct {
	ReleaseConfig renderer.ReleaseConfig `json:"releaseConfig"`

	PushOptions renderer.PushOptions `json:"pushOptions"`
}
