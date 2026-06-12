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
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ resource.Object = &RenderArtifact{}
var _ resource.ObjectWithStatusSubResource = &RenderArtifact{}
var _ rest.PrepareForCreater = &RenderArtifact{}
var _ rest.PrepareForUpdater = &RenderArtifact{}
var _ rest.TableConverter = &RenderArtifact{}
var _ rest.Validater = &RenderArtifact{}
var _ rest.ValidateUpdater = &RenderArtifact{}

func (o *RenderArtifact) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *RenderArtifact) NamespaceScoped() bool {
	return true
}

func (o *RenderArtifact) New() runtime.Object {
	return &RenderArtifact{}
}

func (o *RenderArtifact) NewList() runtime.Object {
	return &RenderArtifactList{}
}

func (o *RenderArtifact) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("renderartifacts").GroupResource()
}

func (o *RenderArtifact) CopyStatusTo(obj runtime.Object) {
	if t, ok := obj.(*RenderArtifact); ok {
		t.Status = o.Status
	}
}

func (o *RenderArtifact) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}

func (o *RenderArtifact) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*RenderArtifact)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *RenderArtifact) Validate(_ context.Context) field.ErrorList {
	return validateRenderArtifact(o)
}

func (o *RenderArtifact) ValidateUpdate(_ context.Context, _ runtime.Object) field.ErrorList {
	return validateRenderArtifact(o)
}

func validateRenderArtifact(o *RenderArtifact) field.ErrorList {
	var errs field.ErrorList
	spec := field.NewPath("spec")
	if o.Spec.BaseURL == "" {
		errs = append(errs, field.Required(spec.Child("baseURL"), "baseURL must not be empty"))
	}
	if o.Spec.Repository == "" {
		errs = append(errs, field.Required(spec.Child("repository"), "repository must not be empty"))
	}
	if o.Spec.Tag == "" {
		errs = append(errs, field.Required(spec.Child("tag"), "tag must not be empty"))
	}
	if o.Spec.RenderTaskRef == "" {
		errs = append(errs, field.Required(spec.Child("renderTaskRef"), "renderTaskRef must not be empty"))
	}

	return errs
}

func (o *RenderArtifact) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Repository", Type: "string"},
			{Name: "Tag", Type: "string"},
			{Name: "ChartURL", Type: "string"},
			{Name: "Age", Type: "string"},
		},
		[]any{
			o.Name,
			o.Spec.Repository,
			o.Spec.Tag,
			o.Status.ChartURL,
			duration.HumanDuration(metav1.Now().Sub(o.CreationTimestamp.Time)),
		},
	), nil
}
