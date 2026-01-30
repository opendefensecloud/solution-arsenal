// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"embed"
	"encoding/json"
)

//go:embed template/release/*
var releaseFS embed.FS

var (
	releaseFiles []string = []string{
		"template/release/Chart.yaml",
		"template/release/values.yaml",
		"template/release/.helmignore",
		"template/release/templates/release.yaml",
	}
)

type ReleaseComponent struct {
	Name string `json:"name"`
}

type ReleaseInput struct {
	Component ReleaseComponent          `json:"component"`
	Helm      ResourceAccess            `json:"helm"`
	KRO       ResourceAccess            `json:"kro"`
	Resources map[string]ResourceAccess `json:"resources"`
}

type ReleaseConfig struct {
	Chart  ChartConfig     `json:"chart"`
	Input  ReleaseInput    `json:"input"`
	Values json.RawMessage `json:"values"`
}

func RenderRelease(c ReleaseConfig) (*RenderResult, error) {
	return newRenderer().
		withTemplateData(c).
		withTemplateFS(releaseFS).
		withTemplateFiles(releaseFiles).
		withOutputName("solar-release").
		render()
}
