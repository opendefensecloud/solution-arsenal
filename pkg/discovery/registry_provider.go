// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	versioned "go.opendefense.cloud/solar/client-go/clientset/versioned"
	solarclient "go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	registryinformers "go.opendefense.cloud/solar/client-go/informers/externalversions/solar/v1alpha1"
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
// referenced Secret to resolve credentials. Existing entries are replaced. A
// referenced Secret that no longer exists drops that registry's credentials
// rather than failing the reload.
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
		if apierrors.IsNotFound(err) {
			logr.FromContextOrDiscard(ctx).Info("referenced secret not found, dropping credentials", "secret", reg.Spec.SolarSecretRef.Name, "registry", reg.Name)
			continue
		}
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

// WatchAPI watches Registry objects and their referenced credential Secrets in
// the given namespace and reloads the provider's full cache (via LoadFromAPI)
// on every add/update/delete, so a spec change (e.g. hostname) or a rotated
// Secret takes effect without a process restart. It returns once the informer
// caches are synced; the informers keep
// running in the background until ctx is cancelled.
func (p *RegistryProvider) WatchAPI(ctx context.Context, client versioned.Interface, secretClient corev1client.CoreV1Interface, namespace string) error {
	log := logr.FromContextOrDiscard(ctx)

	reload := func(event, key string) {
		log.Info("registry event received, reloading registries", "event", event, "registry", key)
		if err := p.LoadFromAPI(ctx, client.SolarV1alpha1(), secretClient, namespace); err != nil {
			log.Error(err, "failed to reload registries from API after watch event", "event", event, "registry", key)
			return
		}
		log.Info("registries reloaded after watch event", "event", event, "registry", key, "count", len(p.GetAll()))
	}
	keyOf := func(obj any) string {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			return "<unknown>"
		}

		return key
	}

	handlers := func(kind string) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj any) { reload(kind+" add", keyOf(obj)) },
			UpdateFunc: func(_, obj any) { reload(kind+" update", keyOf(obj)) },
			DeleteFunc: func(obj any) { reload(kind+" delete", keyOf(obj)) },
		}
	}

	// nolint:contextcheck // generated informer factory takes no context
	registryInformer := registryinformers.NewFilteredRegistryInformer(client, namespace, 0, cache.Indexers{}, nil)
	if _, err := registryInformer.AddEventHandler(handlers("registry")); err != nil {
		return fmt.Errorf("failed to register registry event handler: %w", err)
	}

	secretLW := cache.NewListWatchFromClient(secretClient.RESTClient(), "secrets", namespace, fields.Everything())
	secretInformer := cache.NewSharedIndexInformer(secretLW, &corev1.Secret{}, 0, cache.Indexers{})
	if _, err := secretInformer.AddEventHandler(handlers("secret")); err != nil {
		return fmt.Errorf("failed to register secret event handler: %w", err)
	}

	go registryInformer.Run(ctx.Done())
	go secretInformer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), registryInformer.HasSynced, secretInformer.HasSynced) {
		return fmt.Errorf("failed to sync registry/secret informer caches")
	}

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
