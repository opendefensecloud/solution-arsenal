// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"fmt"
	"time"

	"github.com/creasty/defaults"
)

// RegistryCredentials represents the credentials required to access an OCI registry.
type RegistryCredentials struct {
	// Username is the username used to authenticate with the registry
	Username string `yaml:"username"`
	// Password is the password used to authenticate with the registry
	Password string `yaml:"password"`
}

// Registry is a struct representing an OCI registry.
type Registry struct {
	// Name identifies the registry locally and must be unique
	Name string `yaml:"name"`
	// Flavor is the type of registry, e.g. zot
	Flavor string `yaml:"flavor"`
	// PlainHTTP is a boolean flag indicating whether the repository was discovered using plain HTTP
	PlainHTTP bool `yaml:"plainHTTP"`
	// Hostname is the hostname of the registry
	Hostname string `yaml:"hostname"`
	// Credentials is an optional struct containing credentials for accessing the registry
	Credentials *RegistryCredentials `yaml:"credentials"`
	// WebhoWebhookPath is an optional field that specifies the path of a webhook endpoint in the registry.
	WebhookPath string `yaml:"webhookPath"`
	// ScanInterval specifies how often a full scan will be triggered
	ScanInterval time.Duration `yaml:"scanInterval" default:"24h"`
}

// GetURL constructs and returns the full URL of the registry based on its hostname and PlainHTTP flag.
func (r Registry) GetURL() string {
	scheme := "https"
	if r.PlainHTTP {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, r.Hostname)
}

// UnmarshalYAML implements Unmarshaler interface and adds support for default
// values via tags, which is not supported
func (r *Registry) UnmarshalYAML(unmarshal func(any) error) error {
	if err := defaults.Set(r); err != nil {
		return err
	}

	type plain Registry
	if err := unmarshal((*plain)(r)); err != nil {
		return err
	}

	return nil
}
