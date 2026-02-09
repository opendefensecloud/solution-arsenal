// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"embed"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

//go:embed template/release/*
var releaseFS embed.FS

func RenderRelease(c solarv1alpha1.ReleaseConfig) (*solarv1alpha1.RenderResult, error) {
	r := renderer{
		OutputName:  "solar-release",
		TemplateFS:  releaseFS,
		TemplateDir: "template/release",
		Data:        c,
	}

	return r.render()
}
