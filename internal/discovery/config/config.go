/*
Copyright 2024 Open Defense Cloud Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package config provides configuration handling for the solar-discovery service.
package config

import (
	"fmt"
	"os"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/opendefensecloud/solution-arsenal/internal/discovery/controller"
	"github.com/opendefensecloud/solution-arsenal/pkg/registry/oci"
)

// Config is the configuration for the solar-discovery service.
type Config struct {
	// ScanInterval is the interval between registry scans
	ScanInterval Duration `yaml:"scanInterval"`
	// Concurrency is the number of concurrent repository scans
	Concurrency int `yaml:"concurrency"`
	// Registries is the list of registries to scan
	Registries []RegistryConfig `yaml:"registries"`
	// SolarIndex is the configuration for connecting to solar-index
	SolarIndex SolarIndexConfig `yaml:"solarIndex"`
	// Metrics is the metrics configuration
	Metrics MetricsConfig `yaml:"metrics"`
	// Logging is the logging configuration
	Logging LoggingConfig `yaml:"logging"`
}

// RegistryConfig configures an OCI registry to scan.
type RegistryConfig struct {
	// Name is a friendly name for the registry
	Name string `yaml:"name"`
	// URL is the registry URL
	URL string `yaml:"url"`
	// Repositories is the list of repositories to scan
	Repositories []string `yaml:"repositories,omitempty"`
	// Auth is the authentication configuration
	Auth *AuthConfig `yaml:"auth,omitempty"`
	// Labels are default labels to apply to discovered items
	Labels map[string]string `yaml:"labels,omitempty"`
	// Namespace is the namespace for created CatalogItems
	Namespace string `yaml:"namespace,omitempty"`
	// Enabled indicates if this registry should be scanned
	Enabled *bool `yaml:"enabled,omitempty"`
}

// AuthConfig configures authentication for a registry.
type AuthConfig struct {
	// Type is the authentication type (basic, bearer, docker-config)
	Type string `yaml:"type"`
	// Username is the username for basic auth
	Username string `yaml:"username,omitempty"`
	// Password is the password for basic auth (can be a secret reference)
	Password string `yaml:"password,omitempty"`
	// Token is the bearer token
	Token string `yaml:"token,omitempty"`
	// SecretRef is a reference to a Kubernetes secret
	SecretRef *SecretRef `yaml:"secretRef,omitempty"`
}

// SecretRef references a Kubernetes secret.
type SecretRef struct {
	// Name is the secret name
	Name string `yaml:"name"`
	// Namespace is the secret namespace
	Namespace string `yaml:"namespace,omitempty"`
	// Key is the key within the secret (optional, uses entire secret if not specified)
	Key string `yaml:"key,omitempty"`
}

// SolarIndexConfig configures the connection to solar-index API.
type SolarIndexConfig struct {
	// URL is the solar-index API URL
	URL string `yaml:"url"`
	// Kubeconfig is the path to the kubeconfig file (optional)
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
	// InCluster indicates whether to use in-cluster config
	InCluster bool `yaml:"inCluster,omitempty"`
}

// MetricsConfig configures metrics collection.
type MetricsConfig struct {
	// Enabled indicates if metrics are enabled
	Enabled bool `yaml:"enabled"`
	// Address is the metrics server address
	Address string `yaml:"address"`
	// Path is the metrics endpoint path
	Path string `yaml:"path"`
}

// LoggingConfig configures logging.
type LoggingConfig struct {
	// Level is the log level (debug, info, warn, error)
	Level string `yaml:"level"`
	// Format is the log format (json, text)
	Format string `yaml:"format"`
}

// Duration is a wrapper around time.Duration for YAML marshaling.
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(duration)
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

// Duration returns the time.Duration value.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		ScanInterval: Duration(5 * time.Minute),
		Concurrency:  5,
		Registries:   []RegistryConfig{},
		SolarIndex: SolarIndexConfig{
			InCluster: true,
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Address: ":8080",
			Path:    "/metrics",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// LoadConfig loads configuration from a file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	return ParseConfig(data)
}

// ParseConfig parses configuration from YAML data.
func ParseConfig(data []byte) (*Config, error) {
	config := DefaultConfig()

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return config, nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.ScanInterval.Duration() < time.Second {
		return fmt.Errorf("scanInterval must be at least 1s")
	}

	if c.Concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}

	for i, reg := range c.Registries {
		if reg.Name == "" {
			return fmt.Errorf("registries[%d].name is required", i)
		}
		if reg.URL == "" {
			return fmt.Errorf("registries[%d].url is required", i)
		}
		if reg.Auth != nil {
			if err := reg.Auth.Validate(); err != nil {
				return fmt.Errorf("registries[%d].auth: %w", i, err)
			}
		}
	}

	return nil
}

// Validate validates the auth configuration.
func (a *AuthConfig) Validate() error {
	switch a.Type {
	case "basic":
		if a.Username == "" {
			return fmt.Errorf("username is required for basic auth")
		}
		if a.Password == "" && a.SecretRef == nil {
			return fmt.Errorf("password or secretRef is required for basic auth")
		}
	case "bearer":
		if a.Token == "" && a.SecretRef == nil {
			return fmt.Errorf("token or secretRef is required for bearer auth")
		}
	case "docker-config", "":
		// Docker config auth uses system docker config, no additional validation
	default:
		return fmt.Errorf("unknown auth type: %s", a.Type)
	}
	return nil
}

// ToControllerConfig converts the configuration to controller configuration.
func (c *Config) ToControllerConfig() ([]controller.RegistryConfig, []controller.Option) {
	registries := make([]controller.RegistryConfig, 0, len(c.Registries))

	for _, reg := range c.Registries {
		// Skip disabled registries
		if reg.Enabled != nil && !*reg.Enabled {
			continue
		}

		controllerReg := controller.RegistryConfig{
			Name:         reg.Name,
			URL:          reg.URL,
			Repositories: reg.Repositories,
			Labels:       reg.Labels,
			Namespace:    reg.Namespace,
		}

		// Set up authenticator
		if reg.Auth != nil {
			controllerReg.Auth = reg.Auth.ToAuthenticator()
		}

		registries = append(registries, controllerReg)
	}

	opts := []controller.Option{
		controller.WithScanInterval(c.ScanInterval.Duration()),
		controller.WithConcurrency(c.Concurrency),
	}

	return registries, opts
}

// ToAuthenticator creates an authenticator from the auth config.
func (a *AuthConfig) ToAuthenticator() oci.Authenticator {
	if a == nil {
		return nil
	}

	switch a.Type {
	case "basic":
		return oci.NewBasicAuthenticator(a.Username, a.Password)
	case "bearer":
		return oci.NewBearerAuthenticator(a.Token)
	case "docker-config", "":
		// Try to load docker config
		auth, err := oci.NewDockerConfigAuthenticator(a.Username)
		if err != nil {
			return oci.NewAnonymousAuthenticator()
		}
		return auth
	default:
		return oci.NewAnonymousAuthenticator()
	}
}

// IsEnabled returns true if the registry is enabled.
func (r *RegistryConfig) IsEnabled() bool {
	return r.Enabled == nil || *r.Enabled
}
