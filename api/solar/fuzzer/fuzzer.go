// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package fuzzer

import (
	"go.opendefense.cloud/solar/api/solar"
	"sigs.k8s.io/randfill"

	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
)

// Funcs returns the fuzzer functions for the solar api group.
var Funcs = func(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
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
