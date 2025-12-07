// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_CatalogItem sets defaults for CatalogItem.
func SetDefaults_CatalogItem(obj *CatalogItem) {
	SetDefaults_CatalogItemSpec(&obj.Spec)
}

// SetDefaults_CatalogItemSpec sets defaults for CatalogItem spec.
func SetDefaults_CatalogItemSpec(obj *CatalogItemSpec) {
	// Default category to Application if not specified
	if obj.Category == "" {
		obj.Category = CatalogItemCategoryApplication
	}
}
