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

var _ resource.Object = &Registry{}
var _ rest.PrepareForUpdater = &Registry{}
var _ rest.PrepareForCreater = &Registry{}
var _ rest.TableConverter = &Registry{}

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

func (o *Registry) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*Registry)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *Registry) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}

func (o *Registry) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Hostname", Type: "string"},
			{Name: "Plain HTTP", Type: "boolean"},
			{Name: "Age", Type: "date"},
		},
		[]any{o.Name, o.Spec.Hostname, o.Spec.PlainHTTP, o.CreationTimestamp.Time},
	), nil
}
