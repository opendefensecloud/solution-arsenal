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
	genericapiserver "k8s.io/apiserver/pkg/server"

	solarv1alpha1 "github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1"
)

// SolarServer contains the state for the solar-index API server.
type SolarServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

// New creates a new SolarServer from the given configuration.
func (c CompletedConfig) New() (*SolarServer, error) {
	genericServer, err := c.GenericConfig.New("solar-index", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	s := &SolarServer{
		GenericAPIServer: genericServer,
	}

	// Install the API group.
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		solarv1alpha1.GroupVersion.Group,
		Scheme,
		ParameterCodec,
		Codecs,
	)

	// Create storage for each resource type.
	v1alpha1Storage := map[string]rest.Storage{}

	// CatalogItem storage (namespaced)
	catalogItemStorage, catalogItemStatusStorage := NewCatalogItemStorage(Scheme)
	v1alpha1Storage["catalogitems"] = catalogItemStorage
	v1alpha1Storage["catalogitems/status"] = catalogItemStatusStorage

	// ClusterCatalogItem storage (cluster-scoped)
	clusterCatalogItemStorage, clusterCatalogItemStatusStorage := NewClusterCatalogItemStorage(Scheme)
	v1alpha1Storage["clustercatalogitems"] = clusterCatalogItemStorage
	v1alpha1Storage["clustercatalogitems/status"] = clusterCatalogItemStatusStorage

	// ClusterRegistration storage (namespaced)
	clusterRegistrationStorage, clusterRegistrationStatusStorage := NewClusterRegistrationStorage(Scheme)
	v1alpha1Storage["clusterregistrations"] = clusterRegistrationStorage
	v1alpha1Storage["clusterregistrations/status"] = clusterRegistrationStatusStorage

	// Release storage (namespaced)
	releaseStorage, releaseStatusStorage := NewReleaseStorage(Scheme)
	v1alpha1Storage["releases"] = releaseStorage
	v1alpha1Storage["releases/status"] = releaseStatusStorage

	// Sync storage (namespaced)
	syncStorage, syncStatusStorage := NewSyncStorage(Scheme)
	v1alpha1Storage["syncs"] = syncStorage
	v1alpha1Storage["syncs/status"] = syncStatusStorage

	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = v1alpha1Storage

	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

// ParameterCodec handles versioning of objects in query parameters.
var ParameterCodec = runtime.NewParameterCodec(Scheme)
