// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"go.opendefense.cloud/solar/api/solar/v1alpha1"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/download"
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

func (h *helmHandler) Process(ctx context.Context, comp ocm.ComponentVersionAccess) (*v1alpha1.ComponentVersion, error) {
	result := &v1alpha1.ComponentVersion{}

	mfs := memoryfs.New()

	resources := comp.GetResources()
	for _, res := range resources {
		if res.Meta().Type == string(HelmResource) {
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
			h.logger.Info("Chart found!", "name", chartAccessor.Name())
		}
	}

	return result, nil
}
