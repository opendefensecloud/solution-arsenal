// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package webhook

type Config struct {
	Registry Registry `yaml:"registry"`
}

type Registry struct {
	Name         string   `yaml:"name"`
	URL          string   `yaml:"url"`
	Flavor       string   `yaml:"flavor"`
	Webhook      *Webhook `yaml:"webhook"`
	ScanInterval string   `yaml:"scanInterval" default:"1h"`
}

type Webhook struct {
	Path string `yaml:"path"`
}
