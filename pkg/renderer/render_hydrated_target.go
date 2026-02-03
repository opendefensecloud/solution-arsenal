// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"embed"
)

//go:embed template/hydrated-target/*
var hydratedTargetFS embed.FS

type HydratedTargetInput struct {
	Releases map[string]ResourceAccess `json:"releases"` // NOTE: This should be Profiles eventually
	Userdata map[string]any            `json:"userdata"`
}

type HydratedTargetConfig struct {
	Chart ChartConfig         `json:"chart"`
	Input HydratedTargetInput `json:"input"`
}

func RenderHydratedTarget(c HydratedTargetConfig) (*RenderResult, error) {
	r := renderer{
		OutputName:  "solar-hydrated-target",
		TemplateFS:  hydratedTargetFS,
		TemplateDir: "template/hydrated-target",
		Data:        c,
	}

	return r.render()
}
