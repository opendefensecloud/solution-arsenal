// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"go.opendefense.cloud/kit/apiserver/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ resource.Object = &Discovery{}
var _ resource.ObjectWithStatusSubResource = &Discovery{}

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
