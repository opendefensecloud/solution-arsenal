// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import "os"

type ResourceAccess struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	SecretRef  string `json:"secretRef"`
}

type ChartConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	AppVersion  string `json:"appVersion"`
}

type RenderResult struct {
	Dir string
}

func (r *RenderResult) Close() error {
	return os.RemoveAll(r.Dir)
}

type RendererConfig struct {
	Type          string        `json:"type"`
	ReleaseConfig ReleaseConfig `json:"release"`
	PushOptions   PushOptions   `json:"push"`
} // TODO: finish refactor from main.go
