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

var _ resource.Object = &Profile{}
var _ resource.ObjectWithStatusSubResource = &Profile{}
var _ rest.PrepareForUpdater = &Profile{}
var _ rest.PrepareForCreater = &Profile{}
var _ rest.TableConverter = &Profile{}

func (o *Profile) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *Profile) NamespaceScoped() bool {
	return true
}

func (o *Profile) New() runtime.Object {
	return &Profile{}
}

func (o *Profile) NewList() runtime.Object {
	return &ProfileList{}
}

func (o *Profile) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("profiles").GroupResource()
}

func (o *Profile) CopyStatusTo(obj runtime.Object) {
	if obj, ok := obj.(*Profile); ok {
		obj.Status = o.Status
	}
}

func (o *Profile) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*Profile)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *Profile) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}

func (o *Profile) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Release Ref", Type: "string"},
			{Name: "Matched Targets", Type: "integer"},
			{Name: "Age", Type: "date"},
		},
		[]any{o.Name, o.Spec.ReleaseRef.Name, o.Status.MatchedTargets, o.CreationTimestamp.Time},
	), nil
}
