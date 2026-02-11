// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"embed"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

//go:embed template/hydrated-target/*
var hydratedTargetFS embed.FS

func RenderHydratedTarget(c solarv1alpha1.HydratedTargetConfig) (*solarv1alpha1.RenderResult, error) {
	r := renderer{
		OutputName:  "solar-hydrated-target",
		TemplateFS:  hydratedTargetFS,
		TemplateDir: "template/hydrated-target",
		Data:        c,
	}

	return r.render()
}
