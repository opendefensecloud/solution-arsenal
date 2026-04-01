// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import "helm.sh/helm/v4/pkg/registry"

type PushOptions struct {
	Reference     string
	ClientOptions []registry.ClientOption
}
