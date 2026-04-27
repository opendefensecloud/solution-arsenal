// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"context"

	"go.opendefense.cloud/kit/apiserver/resource"
	"go.opendefense.cloud/kit/apiserver/rest"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ resource.Object = &RenderTask{}
var _ resource.ObjectWithStatusSubResource = &RenderTask{}
var _ rest.PrepareForUpdater = &RenderTask{}
var _ rest.PrepareForCreater = &RenderTask{}
var _ rest.ValidateUpdater = &RenderTask{}
var _ rest.TableConverter = &RenderTask{}

func (o *RenderTask) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *RenderTask) NamespaceScoped() bool {
	return true
}

func (o *RenderTask) New() runtime.Object {
	return &RenderTask{}
}

func (o *RenderTask) NewList() runtime.Object {
	return &RenderTaskList{}
}

func (o *RenderTask) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("rendertasks").GroupResource()
}

func (o *RenderTask) CopyStatusTo(obj runtime.Object) {
	if obj, ok := obj.(*RenderTask); ok {
		obj.Status = o.Status
	}
}

func (o *RenderTask) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*RenderTask)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *RenderTask) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}

func (o *RenderTask) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	status := "Unknown"
	scheduledFalseReason := ""
	for _, c := range o.Status.Conditions {
		if c.Status == metav1.ConditionTrue {
			switch c.Type {
			case "JobSucceeded", "JobFailed":
				status = c.Reason
			case "JobScheduled":
				if status == "Unknown" {
					status = c.Reason
				}
			}
		} else if c.Type == "JobScheduled" && c.Status == metav1.ConditionFalse {
			scheduledFalseReason = c.Reason
		}
	}
	if status == "Unknown" && scheduledFalseReason != "" {
		status = scheduledFalseReason
	}

	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Owner Kind", Type: "string"},
			{Name: "Owner Name", Type: "string"},
			{Name: "Status", Type: "string"},
			{Name: "Age", Type: "string"},
		},
		[]any{o.Name, o.Spec.OwnerKind, o.Spec.OwnerName, status, duration.HumanDuration(metav1.Now().Sub(o.CreationTimestamp.Time))},
	), nil
}

func (o *RenderTask) ValidateUpdate(ctx context.Context, old runtime.Object) field.ErrorList {
	errors := field.ErrorList{}
	or := old.(*RenderTask)

	// RendererConfig is immutable
	if !apiequality.Semantic.DeepEqual(o.Spec.RendererConfig, or.Spec.RendererConfig) {
		errors = append(errors, field.Forbidden(field.NewPath("spec.rendererConfig"), "rendererConfig is immutable"))
	}

	return errors
}
