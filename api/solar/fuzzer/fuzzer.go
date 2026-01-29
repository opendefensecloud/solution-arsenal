// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package fuzzer

import (
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/randfill"

	"go.opendefense.cloud/solar/api/solar"
)

// Funcs returns the fuzzer functions for the solar api group.
var Funcs = func(codecs runtimeserializer.CodecFactory) []any {
	return []any{
		func(s *solar.CatalogItem, c randfill.Continue) {
			c.FillNoCustom(s) // fuzz self without calling this function again
		},
		func(s *solar.CatalogItemSpec, c randfill.Continue) {
			c.FillNoCustom(s)
		},
		func(s *solar.CatalogItemStatus, c randfill.Continue) {
			c.FillNoCustom(s)
		},
	}
}
