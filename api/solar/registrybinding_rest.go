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

var _ resource.Object = &RegistryBinding{}
var _ rest.PrepareForUpdater = &RegistryBinding{}
var _ rest.PrepareForCreater = &RegistryBinding{}

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

func (o *RegistryBinding) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*RegistryBinding)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *RegistryBinding) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}
