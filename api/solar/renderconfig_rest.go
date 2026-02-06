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

var _ resource.Object = &RenderConfig{}
var _ resource.ObjectWithStatusSubResource = &RenderConfig{}
var _ rest.PrepareForUpdater = &RenderConfig{}
var _ rest.PrepareForCreater = &RenderConfig{}

func (o *RenderConfig) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *RenderConfig) NamespaceScoped() bool {
	return true
}

func (o *RenderConfig) New() runtime.Object {
	return &RenderConfig{}
}

func (o *RenderConfig) NewList() runtime.Object {
	return &RenderConfigList{}
}

func (o *RenderConfig) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("renderconfigs").GroupResource()
}

func (o *RenderConfig) CopyStatusTo(obj runtime.Object) {
	if obj, ok := obj.(*RenderConfig); ok {
		obj.Status = o.Status
	}
}

func (o *RenderConfig) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*RenderConfig)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *RenderConfig) PrepareForCreate(ctx context.Context) {
	o.Generation = 0
}
