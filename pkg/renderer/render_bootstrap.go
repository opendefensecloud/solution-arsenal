// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"embed"
	"fmt"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

//go:embed template/bootstrap/*
var bootstrapFS embed.FS

func RenderBootstrap(c solarv1alpha1.BootstrapConfig) (*solarv1alpha1.RenderResult, error) {
	if err := validateBootstrapConfig(c); err != nil {
		return nil, fmt.Errorf("invalid bootstrap config: %w", err)
	}

	r := renderer{
		OutputName:  "solar-bootstrap",
		TemplateFS:  bootstrapFS,
		TemplateDir: "template/bootstrap",
		Data:        c,
	}

	return r.render()
}
