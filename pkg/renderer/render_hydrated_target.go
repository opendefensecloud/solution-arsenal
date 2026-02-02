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
	Profiles map[string]Profile `json:"profiles"`
	Userdata map[string]any     `json:"userdata"`
}

type Profile struct {
	Repository string `json:"repository"`
	Semver     string `json:"semver"`
	SecretRef  string `json:"secretRef"`
}

type HydratedTargetConfig struct {
	Chart  ChartConfig         `json:"chart"`
	Input  HydratedTargetInput `json:"input"`
	Values json.RawMessage     `json:"values"`
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
