// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/creasty/defaults"
	"gopkg.in/yaml.v3"
)

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
	// Webhook is optional and provides information about requests coming in from the registry
	Webhook *Webhook `yaml:"webhook"`
	// ScanInterval specifies how often a full scan will be triggered
	ScanInterval time.Duration `yaml:"scanInterval" default:"24h"`
}

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

// RegistryCredentials represents the credentials required to access an OCI registry.
type RegistryCredentials struct {
	// Username is the username used to authenticate with the registry
	Username string `yaml:"username"`
	// Password is the password used to authenticate with the registry
	Password string `yaml:"password"`
}

type Webhook struct {
	Path string `yaml:"path"`
}

type YamlConfig struct {
	Registries []Registry `yaml:"registries"`
}

type RegistryProvider struct {
	mux      sync.RWMutex
	registry map[string]Registry
}

func NewRegistryProvider() *RegistryProvider {
	return &RegistryProvider{
		registry: make(map[string]Registry),
	}
}

func (p *RegistryProvider) FromYaml(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open registry file: %w", err)
	}
	defer file.Close()

	var ymlConfig YamlConfig
	if err := yaml.NewDecoder(file).Decode(&ymlConfig); err != nil {
		return fmt.Errorf("failed to parse registry file: %w", err)
	}

	for _, registry := range ymlConfig.Registries {
		if err := p.Register(registry); err != nil {
			return fmt.Errorf("failed to register registry '%s': %w", registry.Name, err)
		}
	}

	return nil
}

func (p *RegistryProvider) Register(registry ...Registry) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	for _, reg := range registry {
		if _, inUse := p.registry[reg.Name]; inUse {
			return fmt.Errorf("registry with name '%s' is already registered", reg.Name)
		}

		p.registry[reg.Name] = reg
	}

	return nil
}

func (p *RegistryProvider) Get(name string) *Registry {
	p.mux.RLock()
	defer p.mux.RUnlock()

	if registry, ok := p.registry[name]; ok {
		return &registry
	}

	return nil
}

func (p *RegistryProvider) GetAll() []Registry {
	p.mux.RLock()
	defer p.mux.RUnlock()

	out := make([]Registry, 0, len(p.registry))
	for _, reg := range p.registry {
		out = append(out, reg)
	}

	return out
}
