// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	stderrors "errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"go.opendefense.cloud/ocm-kit/helmvalues"
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

func (h *helmHandler) Process(ocmCtx ocm.Context, ev *discovery.ComponentVersionEvent, comp ocm.ComponentVersionAccess) (*discovery.WriteAPIResourceEvent, error) {
	result := &discovery.WriteAPIResourceEvent{
		Source:        *ev,
		ComponentSpec: comp.GetDescriptor().ComponentSpec,
		Timestamp:     time.Now().UTC(),
	}

	// Check if the component has a Helm resource. If not, return an error.
	for _, res := range comp.GetResources() {
		if res.Meta().Type != string(HelmResource) {
			continue
		}

		if err := h.processHelmResource(ocmCtx, comp, res, result); err != nil {
			return nil, err
		}

		return result, nil
	}

	return nil, errors.New("no helm resource found in component")
}

func (h *helmHandler) processHelmResource(ocmCtx ocm.Context, comp ocm.ComponentVersionAccess, resourceAccess ocm.ResourceAccess, result *discovery.WriteAPIResourceEvent) error {
	mfs := memoryfs.New()

	effPath, err := download.DownloadResource(ocmCtx, resourceAccess, resourceAccess.Meta().Name, download.WithFileSystem(mfs))
	if err != nil {
		return errors.Wrapf(err, "failed to download helm resource %s", resourceAccess.Meta().Name)
	}

	f, err := mfs.Open(effPath)
	if err != nil {
		return err
	}
	defer f.Close()

	charter, err := loader.LoadArchive(f)
	if err != nil {
		return errors.Wrapf(err, "cannot load helm chart")
	}

	chartAccessor, err := chart.NewDefaultAccessor(charter)
	if err != nil {
		return errors.Wrapf(err, "cannot create chart accessor")
	}

	metadata := chartAccessor.MetadataAsMap()
	result.HelmDiscovery.ResourceName = resourceAccess.Meta().Name
	result.HelmDiscovery.Name = chartAccessor.Name()
	result.HelmDiscovery.Description = metadata["Description"].(string)
	result.HelmDiscovery.Version = metadata["Version"].(string)
	result.HelmDiscovery.AppVersion = metadata["AppVersion"].(string)
	result.HelmDiscovery.DefaultValues = chartAccessor.Values()
	result.HelmDiscovery.Schema = chartAccessor.Schema()
	result.HelmDiscovery.Digest = resourceAccess.Meta().Digest.Value
	h.logger.V(1).Info("Chart discovered", "chart", result.HelmDiscovery.Name, "version", result.HelmDiscovery.Version, "appVersion", result.HelmDiscovery.AppVersion, "digest", result.HelmDiscovery.Digest)

	// Look for a helm values template; this is optional — not all OCM packages have one.
	hvt, err := helmvalues.GetHelmValuesTemplate(comp, resourceAccess.Meta().Name)
	if err != nil {
		if stderrors.Is(err, helmvalues.ErrNotFound) {
			h.logger.V(1).Info("No helm values template found for chart", "chart", chartAccessor.Name())

			return nil
		}

		return fmt.Errorf("cannot get helm values template: %w", err)
	}

	input, err := helmvalues.GetRenderingInput(comp)
	if err != nil {
		return fmt.Errorf("cannot get helm values rendering input: %w", err)
	}

	renderedString, err := helmvalues.Render(hvt, input)
	if err != nil {
		return fmt.Errorf("cannot render helm values: %w", err)
	}

	result.HelmDiscovery.ValuesTemplate = &renderedString

	return nil
}
