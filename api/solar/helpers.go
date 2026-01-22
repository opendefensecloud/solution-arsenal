// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"go.opendefense.cloud/kit/apiserver/resource"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

// incrementGenerationIfNotEqual increments the generation of an object if the given objects are not equal
func incrementGenerationIfNotEqual(o resource.Object, a, b any) {
	if !apiequality.Semantic.DeepEqual(a, b) {
		om := o.GetObjectMeta()
		gen := om.GetGeneration()
		om.SetGeneration(gen + 1)
	}
}
