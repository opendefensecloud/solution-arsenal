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

var _ resource.Object = &HydratedTarget{}
var _ resource.ObjectWithStatusSubResource = &HydratedTarget{}
var _ rest.PrepareForUpdater = &HydratedTarget{}
var _ rest.PrepareForCreater = &HydratedTarget{}

func (o *HydratedTarget) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *HydratedTarget) NamespaceScoped() bool {
	return true
}

func (o *HydratedTarget) New() runtime.Object {
	return &HydratedTarget{}
}

func (o *HydratedTarget) NewList() runtime.Object {
	return &HydratedTargetList{}
}

func (o *HydratedTarget) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("hydratedtargets").GroupResource()
}

func (o *HydratedTarget) CopyStatusTo(obj runtime.Object) {
	if obj, ok := obj.(*HydratedTarget); ok {
		obj.Status = o.Status
	}
}

func (o *HydratedTarget) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*HydratedTarget)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *HydratedTarget) PrepareForCreate(ctx context.Context) {
	o.Generation = 0
}
