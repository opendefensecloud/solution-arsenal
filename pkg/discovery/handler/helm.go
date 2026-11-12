// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/ocmv2"
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

func (h *helmHandler) Process(
	ctx context.Context,
	repo *ocmv2.Repository,
	ev *discovery.ComponentVersionEvent,
	desc *descruntime.Descriptor,
) (*discovery.WriteAPIResourceEvent, error) {
	result := &discovery.WriteAPIResourceEvent{
		Source:    *ev,
		Component: desc.Component,
		Timestamp: time.Now().UTC(),
	}

	// Check if the component has a Helm resource. If not, return an error.
	for i := range desc.Component.Resources {
		res := desc.Component.Resources[i]
		if res.Type != string(HelmResource) {
			continue
		}

		if err := h.processHelmResource(ctx, repo, desc, res, result); err != nil {
			return nil, err
		}

		return result, nil
	}

	return nil, errors.New("no helm resource found in component")
}

func (h *helmHandler) processHelmResource(
	ctx context.Context,
	repo *ocmv2.Repository,
	desc *descruntime.Descriptor,
	res descruntime.Resource,
	result *discovery.WriteAPIResourceEvent,
) error {
	archive, err := repo.ChartArchive(ctx, desc, res)
	if err != nil {
		return fmt.Errorf("failed to download helm resource %s: %w", res.Name, err)
	}

	charter, err := loader.LoadArchive(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("cannot load helm chart %s: %w", res.Name, err)
	}

	chartAccessor, err := chart.NewDefaultAccessor(charter)
	if err != nil {
		return fmt.Errorf("cannot create chart accessor for %s: %w", res.Name, err)
	}

	metadata := chartAccessor.MetadataAsMap()
	result.HelmDiscovery.ResourceName = res.Name
	result.HelmDiscovery.Name = chartAccessor.Name()
	result.HelmDiscovery.Description, _ = metadata["Description"].(string)
	result.HelmDiscovery.Version, _ = metadata["Version"].(string)
	result.HelmDiscovery.AppVersion, _ = metadata["AppVersion"].(string)
	result.HelmDiscovery.DefaultValues = chartAccessor.Values()
	result.HelmDiscovery.Schema = chartAccessor.Schema()
	if res.Digest != nil {
		result.HelmDiscovery.Digest = res.Digest.Value
	}
	h.logger.V(1).Info("Chart discovered", "chart", result.HelmDiscovery.Name, "version", result.HelmDiscovery.Version, "appVersion", result.HelmDiscovery.AppVersion, "digest", result.HelmDiscovery.Digest)

	return nil
}
