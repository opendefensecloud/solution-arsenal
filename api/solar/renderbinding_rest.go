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

var _ resource.Object = &RenderBinding{}
var _ rest.PrepareForCreater = &RenderBinding{}
var _ rest.PrepareForUpdater = &RenderBinding{}
var _ rest.TableConverter = &RenderBinding{}
var _ rest.Validater = &RenderBinding{}
var _ rest.ValidateUpdater = &RenderBinding{}

func (o *RenderBinding) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *RenderBinding) NamespaceScoped() bool {
	return true
}

func (o *RenderBinding) New() runtime.Object {
	return &RenderBinding{}
}

func (o *RenderBinding) NewList() runtime.Object {
	return &RenderBindingList{}
}

func (o *RenderBinding) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("renderbindings").GroupResource()
}

func (o *RenderBinding) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
}

func (o *RenderBinding) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*RenderBinding)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *RenderBinding) Validate(_ context.Context) field.ErrorList {
	return validateRenderBinding(o)
}

func (o *RenderBinding) ValidateUpdate(_ context.Context, _ runtime.Object) field.ErrorList {
	return validateRenderBinding(o)
}

func validateRenderBinding(o *RenderBinding) field.ErrorList {
	var errs field.ErrorList
	spec := field.NewPath("spec")
	if o.Spec.RenderArtifactRef.Name == "" {
		errs = append(errs, field.Required(spec.Child("renderArtifactRef").Child("name"), "renderArtifactRef.name must not be empty"))
	}
	if o.Spec.OwnerKind == "" {
		errs = append(errs, field.Required(spec.Child("ownerKind"), "ownerKind must not be empty"))
	}
	if o.Spec.OwnerName == "" {
		errs = append(errs, field.Required(spec.Child("ownerName"), "ownerName must not be empty"))
	}
	if o.Spec.OwnerNamespace == "" {
		errs = append(errs, field.Required(spec.Child("ownerNamespace"), "ownerNamespace must not be empty"))
	}

	return errs
}

func (o *RenderBinding) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Artifact", Type: "string"},
			{Name: "OwnerKind", Type: "string"},
			{Name: "OwnerName", Type: "string"},
			{Name: "Age", Type: "string"},
		},
		[]any{
			o.Name,
			o.Spec.RenderArtifactRef.Name,
			o.Spec.OwnerKind,
			o.Spec.OwnerName,
			duration.HumanDuration(metav1.Now().Sub(o.CreationTimestamp.Time)),
		},
	), nil
}
