// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"

	"ocm.software/ocm/api/ocm"

	"go.opendefense.cloud/solar/api/solar/v1alpha1"
)

type HandlerType string
type OCMResourceType string
type OCMResourceAccessType string

const (
	HelmHandler HandlerType = "helm"
	KroHandler  HandlerType = "kro"
)

const (
	HelmResource OCMResourceType = "helmChart"
	BlobResource OCMResourceType = "blob"
	OCIResource  OCMResourceType = "ociImage"
)

const (
	OCIAccessType OCMResourceAccessType = "ociArtifact"
)

type ComponentHandler interface {
	Process(ctx context.Context, comp ocm.ComponentVersionAccess) (*v1alpha1.ComponentVersion, error)
}
