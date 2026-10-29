// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"

	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/ocmv2"
)

type HandlerType string
type OCMResourceType string

const (
	HelmHandler HandlerType = "helm"
	KroHandler  HandlerType = "kro"
)

const (
	HelmResource OCMResourceType = "helmChart"
	BlobResource OCMResourceType = "blob"
	OCIResource  OCMResourceType = "ociImage"
)

type ComponentHandler interface {
	// Process inspects a resolved component descriptor and produces the event
	// that the API writer turns into catalog resources. repo stays open for the
	// call so handlers can pull resource blobs they need to introspect.
	Process(
		ctx context.Context,
		repo *ocmv2.Repository,
		ev *discovery.ComponentVersionEvent,
		desc *descruntime.Descriptor,
	) (*discovery.WriteAPIResourceEvent, error)
}
