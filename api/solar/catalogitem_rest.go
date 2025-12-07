// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"go.opendefense.cloud/kit/apiserver/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ resource.Object = &CatalogItem{}

func (o *CatalogItem) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *CatalogItem) NamespaceScoped() bool {
	return true
}

func (o *CatalogItem) New() runtime.Object {
	return &CatalogItem{}
}

func (o *CatalogItem) NewList() runtime.Object {
	return &CatalogItemList{}
}

func (o *CatalogItem) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("catalogitems").GroupResource()
}
