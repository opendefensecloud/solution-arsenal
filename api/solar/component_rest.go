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

var _ resource.Object = &Component{}
var _ rest.PrepareForUpdater = &Component{}
var _ rest.PrepareForCreater = &Component{}

func (o *Component) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *Component) NamespaceScoped() bool {
	return true
}

func (o *Component) New() runtime.Object {
	return &Component{}
}

func (o *Component) NewList() runtime.Object {
	return &ComponentList{}
}

func (o *Component) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("components").GroupResource()
}

func (o *Component) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*Component)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *Component) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}
