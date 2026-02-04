// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"go.opendefense.cloud/kit/apiserver/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ resource.Object = &Release{}
var _ resource.ObjectWithStatusSubResource = &Release{}

func (o *Release) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *Release) NamespaceScoped() bool {
	return true
}

func (o *Release) New() runtime.Object {
	return &Release{}
}

func (o *Release) NewList() runtime.Object {
	return &ReleaseList{}
}

func (o *Release) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("releases").GroupResource()
}

func (o *Release) CopyStatusTo(obj runtime.Object) {
	if obj, ok := obj.(*Release); ok {
		obj.Status = o.Status
	}
}
