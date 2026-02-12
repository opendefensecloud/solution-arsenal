// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/download"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type helmHandler struct {
	logger logr.Logger
}

func init() {
	RegisterComponentHandler(HelmHandler, func(log logr.Logger) ComponentHandler {
		return &helmHandler{
			logger: log,
		}
	})
}

func (h *helmHandler) Process(ctx context.Context, ev *discovery.ComponentVersionEvent, comp ocm.ComponentVersionAccess) (*discovery.WriteAPIResourceEvent, error) {
	result := &discovery.WriteAPIResourceEvent{
		Source:    *ev,
		Timestamp: time.Now().UTC(),
	}

	// Check if the component has a Helm resource. If not, return an error.
	for _, res := range comp.GetResources() {
		if res.Meta().Type == string(HelmResource) {
			mfs := memoryfs.New()

			effPath, err := download.DownloadResource(comp.GetContext(), res, res.Meta().Name, download.WithFileSystem(mfs))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to download helm resource %s", res.Meta().Name)
			}

			f, err := mfs.Open(effPath)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			charter, err := loader.LoadArchive(f)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot load helm chart")
			}
			chartAccessor, err := chart.NewDefaultAccessor(charter)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot create chart accessor")
			}

			metadata := chartAccessor.MetadataAsMap()
			result.HelmDiscovery.Name = chartAccessor.Name()
			result.HelmDiscovery.Description = metadata["Description"].(string)
			result.HelmDiscovery.Version = metadata["Version"].(string)
			result.HelmDiscovery.AppVersion = metadata["AppVersion"].(string)
			result.HelmDiscovery.DefaultValues = chartAccessor.Values()
			result.HelmDiscovery.Schema = chartAccessor.Schema()
			result.HelmDiscovery.Digest = res.Meta().Digest.Value
			h.logger.V(1).Info("Chart discovered", "chart", result.HelmDiscovery.Name, "version", result.HelmDiscovery.Version, "appVersion", result.HelmDiscovery.AppVersion, "digest", result.HelmDiscovery.Digest)

			return result, nil
		}
	}

	return nil, errors.New("no helm resource found in component")
}
