// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/download"
	"ocm.software/ocm/api/utils/tarutils"

	"go.opendefense.cloud/solar/api/solar/v1alpha1"
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

	fss := memoryfs.New()

	resources := comp.GetResources()
	for _, res := range resources {
		if res.Meta().Type == string(HelmResource) {
			// helm downloader registered by default.
			effPath, err := download.DownloadResource(comp.GetContext(), res, res.Meta().Name, download.WithFileSystem(fss))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to download helm resource %s", res.Meta().Name)
			}

			// report found files
			files, err := tarutils.ListArchiveContent(effPath, fss)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot list files for helm chart")
			}
			h.logger.Info("Helm chart files", "chart", res.Meta().Name, "files", files)
			for _, f := range files {
				h.logger.V(1).Info(fmt.Sprintf("Helm chart file: %s", f))
			}
		}
	}

	return result, nil
}
