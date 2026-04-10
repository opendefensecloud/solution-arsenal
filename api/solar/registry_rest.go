// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"go.opendefense.cloud/kit/apiserver/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ resource.Object = &Registry{}

func (o *Registry) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *Registry) NamespaceScoped() bool {
	return true
}

func (o *Registry) New() runtime.Object {
	return &Registry{}
}

func (o *Registry) NewList() runtime.Object {
	return &RegistryList{}
}

func (o *Registry) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("registries").GroupResource()
}
