// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"embed"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

//go:embed template/bootstrap/*
var bootstrapFS embed.FS

func RenderBootstrap(c solarv1alpha1.BootstrapConfig) (*solarv1alpha1.RenderResult, error) {
	r := renderer{
		OutputName:  "solar-bootstrap",
		TemplateFS:  bootstrapFS,
		TemplateDir: "template/bootstrap",
		Data:        c,
	}

	return r.render()
}
