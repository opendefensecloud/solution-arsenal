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

package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
	"k8s.io/klog/v2"
)

// StorageFactory creates storage backends for Solar API resources.
type StorageFactory struct {
	options       *Options
	client        *clientv3.Client
	codec         runtime.Codec
	scheme        *runtime.Scheme
	groupResource schema.GroupResource
}

// NewStorageFactory creates a new storage factory.
func NewStorageFactory(opts *Options, codec runtime.Codec, scheme *runtime.Scheme) (*StorageFactory, error) {
	client, err := createEtcdClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	return &StorageFactory{
		options: opts,
		client:  client,
		codec:   codec,
		scheme:  scheme,
	}, nil
}

// createEtcdClient creates an etcd v3 client with the configured options.
func createEtcdClient(opts *Options) (*clientv3.Client, error) {
	cfg := clientv3.Config{
		Endpoints:   opts.Endpoints,
		DialTimeout: opts.DialTimeout,
	}

	if opts.TLSEnabled() {
		tlsConfig, err := createTLSConfig(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		cfg.TLS = tlsConfig
	}

	client, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), opts.DialTimeout)
	defer cancel()

	_, err = client.Status(ctx, opts.Endpoints[0])
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to verify etcd connection: %w", err)
	}

	klog.InfoS("Connected to etcd", "endpoints", opts.Endpoints)
	return client, nil
}

// createTLSConfig creates a TLS configuration for etcd.
func createTLSConfig(opts *Options) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	caData, err := os.ReadFile(opts.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caData) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// NewStorage creates a storage interface for the specified resource.
func (f *StorageFactory) NewStorage(
	gr schema.GroupResource,
	newFunc func() runtime.Object,
	newListFunc func() runtime.Object,
) (storage.Interface, factory.DestroyFunc, error) {
	resourcePrefix := path.Join(f.options.Prefix, gr.Group, gr.Resource)

	config := storagebackend.ConfigForResource{
		Config: storagebackend.Config{
			Type: storagebackend.StorageTypeETCD3,
			Transport: storagebackend.TransportConfig{
				ServerList:    f.options.Endpoints,
				CertFile:      f.options.CertFile,
				KeyFile:       f.options.KeyFile,
				TrustedCAFile: f.options.CAFile,
			},
			Prefix: f.options.Prefix,
			Codec:  f.codec,
		},
		GroupResource: gr,
	}

	store, destroyFunc, err := factory.Create(config, newFunc, newListFunc, resourcePrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create storage for %s: %w", gr.String(), err)
	}

	return store, destroyFunc, nil
}

// NewStorageWithCaching creates a storage interface with caching enabled.
func (f *StorageFactory) NewStorageWithCaching(
	gr schema.GroupResource,
	newFunc func() runtime.Object,
	newListFunc func() runtime.Object,
	cacheSize int,
) (storage.Interface, factory.DestroyFunc, error) {
	resourcePrefix := path.Join(f.options.Prefix, gr.Group, gr.Resource)

	config := storagebackend.ConfigForResource{
		Config: storagebackend.Config{
			Type: storagebackend.StorageTypeETCD3,
			Transport: storagebackend.TransportConfig{
				ServerList:    f.options.Endpoints,
				CertFile:      f.options.CertFile,
				KeyFile:       f.options.KeyFile,
				TrustedCAFile: f.options.CAFile,
			},
			Prefix: f.options.Prefix,
			Codec:  f.codec,
		},
		GroupResource: gr,
	}

	store, destroyFunc, err := factory.Create(config, newFunc, newListFunc, resourcePrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create storage for %s: %w", gr.String(), err)
	}

	// Log caching configuration
	if cacheSize > 0 {
		klog.InfoS("Caching enabled for resource", "resource", gr.String(), "cacheSize", cacheSize)
	}

	return store, destroyFunc, nil
}

// Close closes the etcd client connection.
func (f *StorageFactory) Close() error {
	if f.client != nil {
		return f.client.Close()
	}
	return nil
}

// Healthy checks if the etcd connection is healthy.
func (f *StorageFactory) Healthy(ctx context.Context) error {
	if f.client == nil {
		return fmt.Errorf("etcd client not initialized")
	}

	for _, endpoint := range f.options.Endpoints {
		_, err := f.client.Status(ctx, endpoint)
		if err == nil {
			return nil // At least one endpoint is healthy
		}
	}

	return fmt.Errorf("all etcd endpoints are unhealthy")
}

// Compact performs manual compaction on etcd.
func (f *StorageFactory) Compact(ctx context.Context) error {
	if f.client == nil {
		return fmt.Errorf("etcd client not initialized")
	}

	// Get current revision
	resp, err := f.client.Get(ctx, f.options.Prefix, clientv3.WithLimit(1))
	if err != nil {
		return fmt.Errorf("failed to get current revision: %w", err)
	}

	revision := resp.Header.Revision
	if revision <= 1 {
		return nil // Nothing to compact
	}

	// Compact up to current revision - 1
	_, err = f.client.Compact(ctx, revision-1)
	if err != nil {
		return fmt.Errorf("failed to compact etcd: %w", err)
	}

	klog.InfoS("Compacted etcd", "revision", revision-1)
	return nil
}

// StartCompactionLoop starts a background goroutine that periodically compacts etcd.
func (f *StorageFactory) StartCompactionLoop(ctx context.Context) {
	if f.options.CompactionInterval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(f.options.CompactionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := f.Compact(ctx); err != nil {
					klog.ErrorS(err, "Failed to compact etcd")
				}
			}
		}
	}()
}

// RESTOptionsGetter returns a generic.RESTOptionsGetter for the storage factory.
func (f *StorageFactory) RESTOptionsGetter() generic.RESTOptionsGetter {
	return &restOptionsGetter{factory: f}
}

// restOptionsGetter implements generic.RESTOptionsGetter.
type restOptionsGetter struct {
	factory *StorageFactory
}

// GetRESTOptions returns the REST options for a resource.
func (g *restOptionsGetter) GetRESTOptions(resource schema.GroupResource, _ runtime.Object) (generic.RESTOptions, error) {
	resourcePrefix := path.Join(g.factory.options.Prefix, resource.Group, resource.Resource)
	return generic.RESTOptions{
		StorageConfig: &storagebackend.ConfigForResource{
			Config: storagebackend.Config{
				Type: storagebackend.StorageTypeETCD3,
				Transport: storagebackend.TransportConfig{
					ServerList:    g.factory.options.Endpoints,
					CertFile:      g.factory.options.CertFile,
					KeyFile:       g.factory.options.KeyFile,
					TrustedCAFile: g.factory.options.CAFile,
				},
				Prefix: g.factory.options.Prefix,
				Codec:  g.factory.codec,
			},
			GroupResource: resource,
		},
		DeleteCollectionWorkers: 1,
		EnableGarbageCollection: true,
		ResourcePrefix:          resourcePrefix,
	}, nil
}
