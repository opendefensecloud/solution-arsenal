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

var _ resource.Object = &Bootstrap{}
var _ resource.ObjectWithStatusSubResource = &Bootstrap{}
var _ rest.PrepareForUpdater = &Bootstrap{}
var _ rest.PrepareForCreater = &Bootstrap{}

func (o *Bootstrap) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *Bootstrap) NamespaceScoped() bool {
	return true
}

func (o *Bootstrap) New() runtime.Object {
	return &Bootstrap{}
}

func (o *Bootstrap) NewList() runtime.Object {
	return &BootstrapList{}
}

func (o *Bootstrap) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("bootstraps").GroupResource()
}

func (o *Bootstrap) CopyStatusTo(obj runtime.Object) {
	if obj, ok := obj.(*Bootstrap); ok {
		obj.Status = o.Status
	}
}

func (o *Bootstrap) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*Bootstrap)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *Bootstrap) PrepareForCreate(ctx context.Context) {
	o.Generation = 0
}
