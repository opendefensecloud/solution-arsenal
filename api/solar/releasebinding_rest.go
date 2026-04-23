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

var _ resource.Object = &ReleaseBinding{}
var _ rest.PrepareForUpdater = &ReleaseBinding{}
var _ rest.PrepareForCreater = &ReleaseBinding{}
var _ rest.TableConverter = &ReleaseBinding{}

func (o *ReleaseBinding) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *ReleaseBinding) NamespaceScoped() bool {
	return true
}

func (o *ReleaseBinding) New() runtime.Object {
	return &ReleaseBinding{}
}

func (o *ReleaseBinding) NewList() runtime.Object {
	return &ReleaseBindingList{}
}

func (o *ReleaseBinding) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("releasebindings").GroupResource()
}

func (o *ReleaseBinding) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*ReleaseBinding)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *ReleaseBinding) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}

func (o *ReleaseBinding) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Target", Type: "string"},
			{Name: "Release", Type: "string"},
			{Name: "Age", Type: "date"},
		},
		[]any{o.Name, o.Spec.TargetRef.Name, o.Spec.ReleaseRef.Name, o.CreationTimestamp.Time},
	), nil
}
