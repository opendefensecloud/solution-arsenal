// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"embed"
	"encoding/json"
)

//go:embed template/hydrated-target/*
var hydratedTargetFS embed.FS

type HydratedTargetInput struct {
}

type HydratedTargetConfig struct {
	Chart  ChartConfig
	Input  HydratedTargetInput
	Values json.RawMessage
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
