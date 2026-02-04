// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"embed"
	"encoding/json"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

//go:embed template/release/*
var releaseFS embed.FS

type ReleaseComponent struct {
	Name string `json:"name"`
}

type ReleaseInput struct {
	Component ReleaseComponent                        `json:"component"`
	Helm      solarv1alpha1.ResourceAccess            `json:"helm"`
	KRO       solarv1alpha1.ResourceAccess            `json:"kro"`
	Resources map[string]solarv1alpha1.ResourceAccess `json:"resources"`
}

type ReleaseConfig struct {
	Chart  ChartConfig     `json:"chart"`
	Input  ReleaseInput    `json:"input"`
	Values json.RawMessage `json:"values"`
}

func RenderRelease(c ReleaseConfig) (*RenderResult, error) {
	r := renderer{
		OutputName:  "solar-release",
		TemplateFS:  releaseFS,
		TemplateDir: "template/release",
		Data:        c,
	}

	return r.render()
}
