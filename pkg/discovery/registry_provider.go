// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type RegistryProviderConfig struct {
	Registries []*Registry `yaml:"registries"`
}

// RegistryProvider manages a collection of OCI registries.
type RegistryProvider struct {
	mux      sync.RWMutex
	registry map[string]*Registry
}

// NewRegistryProvider creates and returns a new RegistryProvider instance.
func NewRegistryProvider() *RegistryProvider {
	return &RegistryProvider{
		registry: make(map[string]*Registry),
	}
}

// Unmarshall loads registries from a YAML file located at the given path.
func (p *RegistryProvider) Unmarshall(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open registry file: %w", err)
	}
	defer file.Close()

	var ymlConfig RegistryProviderConfig
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

// Marshall serializes the current registries to YAML format.
func (p *RegistryProvider) Marshall() ([]byte, error) {
	p.mux.RLock()
	defer p.mux.RUnlock()

	var ymlConfig RegistryProviderConfig
	for _, reg := range p.registry {
		ymlConfig.Registries = append(ymlConfig.Registries, reg)
	}

	data, err := yaml.Marshal(ymlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal registries to YAML: %w", err)
	}

	return data, nil
}

// Register adds one or more registries to the provider.
func (p *RegistryProvider) Register(registry ...*Registry) error {
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

// Get retrieves a registry by its name. It returns nil if the registry does not exist.
func (p *RegistryProvider) Get(name string) *Registry {
	p.mux.RLock()
	defer p.mux.RUnlock()

	if registry, ok := p.registry[name]; ok {
		return registry
	}

	return nil
}

// GetAll returns a slice of all registered registries.
func (p *RegistryProvider) GetAll() []*Registry {
	p.mux.RLock()
	defer p.mux.RUnlock()

	out := make([]*Registry, 0, len(p.registry))
	for _, reg := range p.registry {
		out = append(out, reg)
	}

	return out
}
