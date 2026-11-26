// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"context"

	"go.opendefense.cloud/kit/apiserver/resource"
	"go.opendefense.cloud/kit/apiserver/rest"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ resource.Object = &Target{}
var _ resource.ObjectWithStatusSubResource = &Target{}
var _ rest.PrepareForUpdater = &Target{}
var _ rest.PrepareForCreater = &Target{}
var _ rest.TableConverter = &Target{}
var _ rest.NameValidator = &Target{}
var _ rest.Validater = &Target{}
var _ rest.ValidateUpdater = &Target{}

func (o *Target) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *Target) NamespaceScoped() bool {
	return true
}

func (o *Target) New() runtime.Object {
	return &Target{}
}

func (o *Target) NewList() runtime.Object {
	return &TargetList{}
}

func (o *Target) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("targets").GroupResource()
}

func (o *Target) CopyStatusTo(obj runtime.Object) {
	if obj, ok := obj.(*Target); ok {
		obj.Status = o.Status
	}
}

func (o *Target) PrepareForUpdate(ctx context.Context, old runtime.Object) {
	or := old.(*Target)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *Target) PrepareForCreate(ctx context.Context) {
	o.Generation = 1
	o.Status = TargetStatus{}
}

// ValidateName enforces an RFC 1123 label (≤63 chars, no dots) instead of the
// DNS-1123 subdomain Kubernetes allows, because metadata.name is embedded in
// the rendered "bootstrap-<name>" chart name and OCI repository path.
func (o *Target) ValidateName(name string, prefix bool) []string {
	return apimachineryvalidation.NameIsDNSLabel(name, prefix)
}

func (o *Target) Validate(_ context.Context) field.ErrorList {
	return validateTarget(o)
}

func (o *Target) ValidateUpdate(_ context.Context, _ runtime.Object) field.ErrorList {
	return validateTarget(o)
}

// validateTarget requires both halves of signing: a signed Target always names
// the public keys Flux verifies its charts with.
func validateTarget(o *Target) field.ErrorList {
	if o.Spec.Signing == nil {
		return nil
	}

	signing := field.NewPath("spec").Child("signing")
	errs := validateSecretName(signing.Child("solarPrivateKeySecretRef").Child("name"), o.Spec.Signing.SolarPrivateKeySecretRef.Name)

	return append(errs, validateSecretName(signing.Child("targetPublicKeySecretName"), o.Spec.Signing.TargetPublicKeySecretName)...)
}

// validateSecretName requires a valid Secret name.
func validateSecretName(path *field.Path, name string) field.ErrorList {
	if name == "" {
		return field.ErrorList{field.Required(path, "a Secret name is required when signing is configured")}
	}

	var errs field.ErrorList
	for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(name, false) {
		errs = append(errs, field.Invalid(path, name, msg))
	}

	return errs
}

func (o *Target) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Render Registry", Type: "string"},
			{Name: "Signing", Type: "boolean"},
			{Name: "Bootstrap Version", Type: "integer"},
			{Name: "Age", Type: "string"},
		},
		[]any{o.Name, o.Spec.RenderRegistryRef.Name, o.Spec.Signing != nil, o.Status.BootstrapVersion, duration.HumanDuration(metav1.Now().Sub(o.CreationTimestamp.Time))},
	), nil
}
