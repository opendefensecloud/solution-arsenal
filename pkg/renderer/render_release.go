// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"text/template"
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
	dir, err := os.MkdirTemp("", "solar-release")
	if err != nil {
		return nil, err
	}

	for _, fname := range releaseFiles {
		tpl, err := template.New(filepath.Base(fname)).Delims("<<", ">>").Funcs(funcMap()).ParseFS(releaseFS, fname)
		if err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}

		outputPath := filepath.Join(dir, filepath.Base(fname))
		// Handle nested paths for templates directory
		if filepath.Dir(filepath.Base(fname)) == "templates" || filepath.Base(fname) == "release.yaml" {
			// Create templates directory if needed
			templatesDir := filepath.Join(dir, "templates")
			_ = os.MkdirAll(templatesDir, 0755)
			outputPath = filepath.Join(templatesDir, "release.yaml")
		}

		// Ensure parent directory exists
		_ = os.MkdirAll(filepath.Dir(outputPath), 0755)

		f, err := os.Create(outputPath)
		if err != nil {
			_ = f.Close()
			_ = os.RemoveAll(dir)
			return nil, err
		}

		err = tpl.Execute(f, &c)
		if err != nil {
			_ = f.Close()
			_ = os.RemoveAll(dir)
			return nil, err
		}

		_ = f.Close()
	}

	return &RenderResult{
		Dir: dir,
	}, nil
}
