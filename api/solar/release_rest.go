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

var (
	_ resource.Object                      = &Release{}
	_ resource.ObjectWithStatusSubResource = &Release{}
	_ rest.PrepareForUpdater               = &Release{}
	_ rest.PrepareForCreater               = &Release{}
	_ rest.TableConverter                  = &Release{}
)

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

func (o *Release) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*Release)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *Release) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}

func (o *Release) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	status := "Unknown"
	for _, c := range o.Status.Conditions {
		if c.Type == "ComponentVersionResolved" {
			status = c.Reason
			break
		}
	}

	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "ComponentVersion Ref", Type: "string"},
			{Name: "Status", Type: "string"},
			{Name: "Age", Type: "date"},
		},
		[]any{o.Name, o.Spec.ComponentVersionRef.Name, status, o.CreationTimestamp.Time},
	), nil
}
