// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
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
	"ocm.software/ocm/api/ocm/internal"
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

		mfs := memoryfs.New()

		effPath, err := download.DownloadResource(ocmCtx, res, res.Meta().Name, download.WithFileSystem(mfs))
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
		result.HelmDiscovery.ResourceName = res.Meta().Name
		result.HelmDiscovery.Name = chartAccessor.Name()
		result.HelmDiscovery.Description = metadata["Description"].(string)
		result.HelmDiscovery.Version = metadata["Version"].(string)
		result.HelmDiscovery.AppVersion = metadata["AppVersion"].(string)
		result.HelmDiscovery.DefaultValues = chartAccessor.Values()
		result.HelmDiscovery.Schema = chartAccessor.Schema()
		result.HelmDiscovery.Digest = res.Meta().Digest.Value
		h.logger.V(1).Info("Chart discovered", "chart", result.HelmDiscovery.Name, "version", result.HelmDiscovery.Version, "appVersion", result.HelmDiscovery.AppVersion, "digest", result.HelmDiscovery.Digest)

		hvt, err := helmvalues.GetHelmValuesTemplate(comp, chartAccessor.Name())
		if err != nil {
			return nil, fmt.Errorf("cannot get helm values template: %w", err)
		}

		// Get rendering input with component data
		input, err := helmvalues.GetRenderingInput(comp)
		if err != nil {
			return nil, fmt.Errorf("cannot get helm values rendering input: %w", err)
		}

		// Render the template
		renderedString, err := helmvalues.Render(hvt, input)
		if err != nil {
			h.logger.Error(err, "cannot render helm values")
		}

		return result, nil
	}

	return nil, errors.New("no helm resource found in component")
}

func (h *helmHandler) processHelmRepo(ocmCtx ocm.Context, res internal.ResourceAccess, result *discovery.WriteAPIResourceEvent) error {
	mfs := memoryfs.New()

	effPath, err := download.DownloadResource(ocmCtx, res, res.Meta().Name, download.WithFileSystem(mfs))
	if err != nil {
		return errors.Wrapf(err, "failed to download helm resource %s", res.Meta().Name)
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
	result.HelmDiscovery.ResourceName = res.Meta().Name
	result.HelmDiscovery.Name = chartAccessor.Name()
	result.HelmDiscovery.Description = metadata["Description"].(string)
	result.HelmDiscovery.Version = metadata["Version"].(string)
	result.HelmDiscovery.AppVersion = metadata["AppVersion"].(string)
	result.HelmDiscovery.DefaultValues = chartAccessor.Values()
	result.HelmDiscovery.Schema = chartAccessor.Schema()
	result.HelmDiscovery.Digest = res.Meta().Digest.Value
	h.logger.V(1).Info("Chart discovered", "chart", result.HelmDiscovery.Name, "version", result.HelmDiscovery.Version, "appVersion", result.HelmDiscovery.AppVersion, "digest", result.HelmDiscovery.Digest)

	hvt, err := helmvalues.GetHelmValuesTemplate(comp, chartAccessor.Name())
	if err != nil {
		return fmt.Errorf("cannot get helm values template: %w", err)
	}

	repo, err := ocmCtx.RepositoryForSpec(ocireg.NewRepositorySpec(cvr.BaseURL()))
	if err != nil {
		return fmt.Errorf("failed to get repository for spec: %w", err)
	}
	defer repo.Close()

	// Get component version
	// (chartAccessor.Name, metadata["Version"].(string))???
	compVer, err := repo.LookupComponentVersion(cvr.ComponentName, cvr.Version)
	if err != nil {
		return fmt.Errorf("failed to lookup component version: %w", err)
	}
	defer compVer.Close()

	// Get rendering input with component data
	input, err := helmvalues.GetRenderingInput(compVer)
	if err != nil {
		return fmt.Errorf("cannot get helm values rendering input: %w", err)
	}

	// Render the template
	_, err = helmvalues.Render(hvt, input)
	if err != nil {
		h.logger.Error(err, "cannot render helm values")
	}

	return nil
}
