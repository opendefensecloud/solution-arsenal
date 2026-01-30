// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type renderer struct {
	OutputName  string
	TemplateFS  fs.FS
	TemplateDir string
	Data        any
}

func (r *renderer) render() (*RenderResult, error) {
	tmp, err := os.MkdirTemp("", r.OutputName)
	if err != nil {
		return nil, err
	}

	files := []string{}
	err = fs.WalkDir(r.TemplateFS, r.TemplateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Type().IsRegular() {
			files = append(files, strings.TrimPrefix(path, r.TemplateDir+"/"))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	for _, fname := range files {
		err = r.renderFile(fname, tmp)
		if err != nil {
			_ = os.RemoveAll(tmp)
			return nil, err
		}
	}

	return &RenderResult{
		Dir: tmp,
	}, nil
}

func (r *renderer) renderFile(name string, dest string) error {
	tpl, err := template.New(filepath.Base(name)).Delims("<<", ">>").Funcs(funcMap()).ParseFS(r.TemplateFS, filepath.Join(r.TemplateDir, name))
	if err != nil {
		return err
	}

	outputPath := filepath.Join(dest, name)

	// Handle nested paths
	if filepath.Dir(name) != "." {
		d := filepath.Join(dest, filepath.Dir(name))
		_ = os.MkdirAll(d, 0755)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return tpl.Execute(f, &r.Data)
}
