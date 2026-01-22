// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"context"

	"go.opendefense.cloud/kit/apiserver/resource"
	"go.opendefense.cloud/kit/apiserver/rest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ resource.Object = &CatalogItem{}
var _ rest.PrepareForCreater = &Discovery{}
var _ rest.PrepareForUpdater = &Discovery{}

func (o *CatalogItem) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}

func (o *CatalogItem) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*CatalogItem)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

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
