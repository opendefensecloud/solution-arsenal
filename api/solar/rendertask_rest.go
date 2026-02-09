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

var _ resource.Object = &RenderTask{}
var _ resource.ObjectWithStatusSubResource = &RenderTask{}
var _ rest.PrepareForUpdater = &RenderTask{}
var _ rest.PrepareForCreater = &RenderTask{}

func (o *RenderTask) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *RenderTask) NamespaceScoped() bool {
	return true
}

func (o *RenderTask) New() runtime.Object {
	return &RenderTask{}
}

func (o *RenderTask) NewList() runtime.Object {
	return &RenderTaskList{}
}

func (o *RenderTask) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("rendertasks").GroupResource()
}

func (o *RenderTask) CopyStatusTo(obj runtime.Object) {
	if obj, ok := obj.(*RenderTask); ok {
		obj.Status = o.Status
	}
}

func (o *RenderTask) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*RenderTask)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *RenderTask) PrepareForCreate(ctx context.Context) {
	o.Generation = 0
}
