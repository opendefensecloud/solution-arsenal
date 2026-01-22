// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"context"
	"reflect"

	"go.opendefense.cloud/kit/apiserver/resource"
	"go.opendefense.cloud/kit/apiserver/rest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ resource.Object = &Discovery{}
var _ resource.ObjectWithStatusSubResource = &Discovery{}
var _ rest.PrepareForCreater = &Discovery{}
var _ rest.PrepareForUpdater = &Discovery{}

func (o *Discovery) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}

func (o *Discovery) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	oldRes := old.(*Discovery)
	// Compare spec equals
	if !reflect.DeepEqual(o.Spec, oldRes.Spec) {
		o.Generation++
	}
}

func (o *Discovery) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *Discovery) NamespaceScoped() bool {
	return true
}

func (o *Discovery) New() runtime.Object {
	return &Discovery{}
}

func (o *Discovery) NewList() runtime.Object {
	return &DiscoveryList{}
}

func (o *Discovery) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("discoveries").GroupResource()
}

func (o *Discovery) CopyStatusTo(obj runtime.Object) {
	if obj, ok := obj.(*Discovery); ok {
		obj.Status = o.Status
	}
}
