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

var _ resource.Object = &ReferenceGrant{}
var _ rest.PrepareForUpdater = &ReferenceGrant{}
var _ rest.PrepareForCreater = &ReferenceGrant{}

func (o *ReferenceGrant) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *ReferenceGrant) NamespaceScoped() bool {
	return true
}

func (o *ReferenceGrant) New() runtime.Object {
	return &ReferenceGrant{}
}

func (o *ReferenceGrant) NewList() runtime.Object {
	return &ReferenceGrantList{}
}

func (o *ReferenceGrant) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("referencegrants").GroupResource()
}

func (o *ReferenceGrant) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*ReferenceGrant)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *ReferenceGrant) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}
