// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import "os"

const (
	TypeHydratedTarget RendererConfigType = "hydrated-target"
	TypeRelease        RendererConfigType = "release"
	TypeProfile        RendererConfigType = "profile"
)

type RendererConfigType string

type ChartConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	AppVersion  string `json:"appVersion"`
}

type RenderResult struct {
	Dir string
}

func (r *RenderResult) Close() error {
	return os.RemoveAll(r.Dir)
}

// TODO: move this to API and use it in RenderConfigSpec
type Config struct {
	Type                 RendererConfigType   `json:"type"`
	ReleaseConfig        ReleaseConfig        `json:"release"`
	HydratedTargetConfig HydratedTargetConfig `json:"hydrated-target"`
	PushOptions          PushOptions          `json:"push"`
}
