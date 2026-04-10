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

var _ resource.Object = &Target{}
var _ resource.ObjectWithStatusSubResource = &Target{}
var _ rest.PrepareForUpdater = &Target{}
var _ rest.PrepareForCreater = &Target{}

func (o *Target) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *Target) NamespaceScoped() bool {
	return true
}

func (o *Target) New() runtime.Object {
	return &Target{}
}

func (o *Target) NewList() runtime.Object {
	return &TargetList{}
}

func (o *Target) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("targets").GroupResource()
}

func (o *Target) CopyStatusTo(obj runtime.Object) {
	if obj, ok := obj.(*Target); ok {
		obj.Status = o.Status
	}
}

func (o *Target) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*Target)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *Target) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}
