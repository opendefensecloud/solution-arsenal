// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"go.opendefense.cloud/kit/apiserver/resource"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// newTable creates a metav1.Table with the given column definitions and a single row of cells.
// The original object is embedded in the row so the API server can extract metadata.
func newTable(obj runtime.Object, columns []metav1.TableColumnDefinition, cells []any) *metav1.Table {
	return &metav1.Table{
		ColumnDefinitions: columns,
		Rows: []metav1.TableRow{
			{
				Cells:  cells,
				Object: runtime.RawExtension{Object: obj},
			},
		},
	}
}

// incrementGenerationIfNotEqual increments the generation of an object if the given objects are not equal
func incrementGenerationIfNotEqual(o resource.Object, a, b any) {
	if !apiequality.Semantic.DeepEqual(a, b) {
		om := o.GetObjectMeta()
		gen := om.GetGeneration()
		om.SetGeneration(gen + 1)
	}
}
