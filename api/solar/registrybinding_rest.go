// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"go.opendefense.cloud/kit/apiserver/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ resource.Object = &RegistryBinding{}

func (o *RegistryBinding) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *RegistryBinding) NamespaceScoped() bool {
	return true
}

func (o *RegistryBinding) New() runtime.Object {
	return &RegistryBinding{}
}

func (o *RegistryBinding) NewList() runtime.Object {
	return &RegistryBindingList{}
}

func (o *RegistryBinding) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("registrybindings").GroupResource()
}
