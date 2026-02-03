// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"go.opendefense.cloud/kit/apiserver/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ resource.Object = &HydratedTarget{}
var _ resource.ObjectWithStatusSubResource = &HydratedTarget{}

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
