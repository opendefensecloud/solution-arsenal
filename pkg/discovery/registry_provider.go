// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solarclient "go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
)

const (
	// SecretKeyUsername is the key in a SolarSecretRef Secret that holds the registry username.
	SecretKeyUsername = "username"
	// SecretKeyPassword is the key in a SolarSecretRef Secret that holds the registry password.
	SecretKeyPassword = "password"
)

// RegistryProvider manages a collection of OCI registries loaded from the solar.Registry API.
type RegistryProvider struct {
	mux        sync.RWMutex
	registries map[string]*solarv1alpha1.Registry
	creds      map[string]*RegistryCredentials
}

// NewRegistryProvider creates and returns a new, empty RegistryProvider instance.
func NewRegistryProvider() *RegistryProvider {
	return &RegistryProvider{
		registries: make(map[string]*solarv1alpha1.Registry),
		creds:      make(map[string]*RegistryCredentials),
	}
}

// LoadFromAPI lists all solar.Registry objects in the given namespace from the
// Kubernetes API server and, for those with a SolarSecretRef, reads the
// referenced Secret to resolve credentials. Existing entries are replaced.
func (p *RegistryProvider) LoadFromAPI(ctx context.Context, solarClient solarclient.SolarV1alpha1Interface, secretClient corev1client.CoreV1Interface, namespace string) error {
	list, err := solarClient.Registries(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list registries in namespace %q: %w", namespace, err)
	}

	registries := make(map[string]*solarv1alpha1.Registry, len(list.Items))
	creds := make(map[string]*RegistryCredentials)

	for i := range list.Items {
		reg := &list.Items[i]
		registries[reg.Name] = reg

		if reg.Spec.SolarSecretRef == nil {
			continue
		}

		secret, err := secretClient.Secrets(namespace).Get(ctx, reg.Spec.SolarSecretRef.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to read secret %q for registry %q: %w", reg.Spec.SolarSecretRef.Name, reg.Name, err)
		}

		username, ok := secret.Data[SecretKeyUsername]
		if !ok {
			return fmt.Errorf("secret %q for registry %q is missing key %q", reg.Spec.SolarSecretRef.Name, reg.Name, SecretKeyUsername)
		}

		password, ok := secret.Data[SecretKeyPassword]
		if !ok {
			return fmt.Errorf("secret %q for registry %q is missing key %q", reg.Spec.SolarSecretRef.Name, reg.Name, SecretKeyPassword)
		}

		creds[reg.Name] = &RegistryCredentials{
			Username: string(username),
			Password: string(password),
		}
	}

	p.mux.Lock()
	defer p.mux.Unlock()

	p.registries = registries
	p.creds = creds

	return nil
}

// Register adds or replaces a registry entry directly. Primarily used in tests.
func (p *RegistryProvider) Register(reg *solarv1alpha1.Registry, creds *RegistryCredentials) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	if _, inUse := p.registries[reg.Name]; inUse {
		return fmt.Errorf("registry with name %q is already registered", reg.Name)
	}

	p.registries[reg.Name] = reg
	if creds != nil {
		p.creds[reg.Name] = creds
	}

	return nil
}

// Get retrieves a registry by its Kubernetes name. Returns nil if not found.
func (p *RegistryProvider) Get(name string) *solarv1alpha1.Registry {
	p.mux.RLock()
	defer p.mux.RUnlock()

	return p.registries[name]
}

// GetCredentials returns the resolved credentials for the named registry, or
// nil if the registry has no SolarSecretRef or was not found.
func (p *RegistryProvider) GetCredentials(name string) *RegistryCredentials {
	p.mux.RLock()
	defer p.mux.RUnlock()

	return p.creds[name]
}

// GetAll returns a snapshot of all registered registries.
func (p *RegistryProvider) GetAll() []*solarv1alpha1.Registry {
	p.mux.RLock()
	defer p.mux.RUnlock()

	out := make([]*solarv1alpha1.Registry, 0, len(p.registries))
	for _, reg := range p.registries {
		out = append(out, reg)
	}

	return out
}
