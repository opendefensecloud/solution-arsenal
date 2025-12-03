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

package apiserver

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	solarv1alpha1 "github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1"
	"github.com/opendefensecloud/solution-arsenal/internal/index/registry"
)

// NewCatalogItemStorage creates storage for CatalogItem resources.
func NewCatalogItemStorage(scheme *runtime.Scheme) (rest.Storage, rest.Storage) {
	store := registry.NewMemoryStore(
		func() runtime.Object { return &solarv1alpha1.CatalogItem{} },
		func() runtime.Object { return &solarv1alpha1.CatalogItemList{} },
		solarv1alpha1.GroupVersion.WithKind("CatalogItem"),
		true, // namespaced
	)

	statusStore := registry.NewStatusStore(
		store,
		func(from, to runtime.Object) {
			fromItem := from.(*solarv1alpha1.CatalogItem)
			toItem := to.(*solarv1alpha1.CatalogItem)
			toItem.Status = fromItem.Status
		},
		func() runtime.Object { return &solarv1alpha1.CatalogItem{} },
		true,
		"CatalogItem",
	)

	return store, statusStore
}

// NewClusterCatalogItemStorage creates storage for ClusterCatalogItem resources.
func NewClusterCatalogItemStorage(scheme *runtime.Scheme) (rest.Storage, rest.Storage) {
	store := registry.NewMemoryStore(
		func() runtime.Object { return &solarv1alpha1.ClusterCatalogItem{} },
		func() runtime.Object { return &solarv1alpha1.ClusterCatalogItemList{} },
		solarv1alpha1.GroupVersion.WithKind("ClusterCatalogItem"),
		false, // cluster-scoped
	)

	statusStore := registry.NewStatusStore(
		store,
		func(from, to runtime.Object) {
			fromItem := from.(*solarv1alpha1.ClusterCatalogItem)
			toItem := to.(*solarv1alpha1.ClusterCatalogItem)
			toItem.Status = fromItem.Status
		},
		func() runtime.Object { return &solarv1alpha1.ClusterCatalogItem{} },
		false,
		"ClusterCatalogItem",
	)

	return store, statusStore
}

// NewClusterRegistrationStorage creates storage for ClusterRegistration resources.
func NewClusterRegistrationStorage(scheme *runtime.Scheme) (rest.Storage, rest.Storage) {
	store := registry.NewMemoryStore(
		func() runtime.Object { return &solarv1alpha1.ClusterRegistration{} },
		func() runtime.Object { return &solarv1alpha1.ClusterRegistrationList{} },
		solarv1alpha1.GroupVersion.WithKind("ClusterRegistration"),
		true, // namespaced
	)

	statusStore := registry.NewStatusStore(
		store,
		func(from, to runtime.Object) {
			fromItem := from.(*solarv1alpha1.ClusterRegistration)
			toItem := to.(*solarv1alpha1.ClusterRegistration)
			toItem.Status = fromItem.Status
		},
		func() runtime.Object { return &solarv1alpha1.ClusterRegistration{} },
		true,
		"ClusterRegistration",
	)

	return store, statusStore
}

// NewReleaseStorage creates storage for Release resources.
func NewReleaseStorage(scheme *runtime.Scheme) (rest.Storage, rest.Storage) {
	store := registry.NewMemoryStore(
		func() runtime.Object { return &solarv1alpha1.Release{} },
		func() runtime.Object { return &solarv1alpha1.ReleaseList{} },
		solarv1alpha1.GroupVersion.WithKind("Release"),
		true, // namespaced
	)

	statusStore := registry.NewStatusStore(
		store,
		func(from, to runtime.Object) {
			fromItem := from.(*solarv1alpha1.Release)
			toItem := to.(*solarv1alpha1.Release)
			toItem.Status = fromItem.Status
		},
		func() runtime.Object { return &solarv1alpha1.Release{} },
		true,
		"Release",
	)

	return store, statusStore
}

// NewSyncStorage creates storage for Sync resources.
func NewSyncStorage(scheme *runtime.Scheme) (rest.Storage, rest.Storage) {
	store := registry.NewMemoryStore(
		func() runtime.Object { return &solarv1alpha1.Sync{} },
		func() runtime.Object { return &solarv1alpha1.SyncList{} },
		solarv1alpha1.GroupVersion.WithKind("Sync"),
		true, // namespaced
	)

	statusStore := registry.NewStatusStore(
		store,
		func(from, to runtime.Object) {
			fromItem := from.(*solarv1alpha1.Sync)
			toItem := to.(*solarv1alpha1.Sync)
			toItem.Status = fromItem.Status
		},
		func() runtime.Object { return &solarv1alpha1.Sync{} },
		true,
		"Sync",
	)

	return store, statusStore
}
