// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"context"

	"go.opendefense.cloud/kit/apiserver/resource"
	"go.opendefense.cloud/kit/apiserver/rest"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	_ resource.Object                      = &Release{}
	_ resource.ObjectWithStatusSubResource = &Release{}
	_ rest.PrepareForUpdater               = &Release{}
	_ rest.PrepareForCreater               = &Release{}
	_ rest.TableConverter                  = &Release{}
	_ rest.Validater                       = &Release{}
	_ rest.ValidateUpdater                 = &Release{}
	_ rest.NameValidator                   = &Release{}
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
	o.Status = ReleaseStatus{}
}

func (o *Release) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	reason := func(conditionType string) string {
		if c := apimeta.FindStatusCondition(o.Status.Conditions, conditionType); c != nil {
			return c.Reason
		}

		return "Unknown"
	}

	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "ComponentVersion Ref", Type: "string"},
			{Name: "Status", Type: "string"},
			{Name: "Renderable", Type: "string"},
			{Name: "Age", Type: "string"},
		},
		[]any{o.Name, o.Spec.ComponentVersionRef.Name, reason("ComponentVersionResolved"), reason("Renderable"), duration.HumanDuration(metav1.Now().Sub(o.CreationTimestamp.Time))},
	), nil
}

func (o *Release) Validate(ctx context.Context) field.ErrorList {
	return validateRelease(o)
}

// ValidateName enforces an RFC 1123 label (≤63 chars, no dots) instead of the
// DNS-1123 subdomain Kubernetes allows, because metadata.name is embedded in
// the rendered "release-<name>" chart name and OCI repository path.
func (o *Release) ValidateName(name string, prefix bool) []string {
	return apimachineryvalidation.NameIsDNSLabel(name, prefix)
}

func (o *Release) ValidateUpdate(ctx context.Context, old runtime.Object) field.ErrorList {
	errors := validateRelease(o)
	or := old.(*Release)
	if o.Spec.UniqueName != or.Spec.UniqueName {
		errors = append(errors, field.Forbidden(field.NewPath("spec").Child("uniqueName"), "uniqueName is immutable"))
	}

	return errors
}

func validateRelease(o *Release) field.ErrorList {
	var errors field.ErrorList
	spec := field.NewPath("spec")
	if o.Spec.ComponentVersionRef.Name == "" {
		errors = append(errors, field.Required(
			spec.Child("componentVersionRef").Child("name"),
			"componentVersionRef.name must not be empty",
		))
	} else {
		errors = append(errors, invalidName(spec.Child("componentVersionRef").Child("name"),
			o.Spec.ComponentVersionRef.Name, apimachineryvalidation.NameIsDNSSubdomain)...)
	}

	// The renderer prints these into the target's manifests.
	if ns := o.Spec.TargetNamespace; ns != nil && *ns != "" {
		errors = append(errors, invalidName(spec.Child("targetNamespace"), *ns, apimachineryvalidation.ValidateNamespaceName)...)
	}
	if o.Spec.UniqueName != "" {
		errors = append(errors, invalidName(spec.Child("uniqueName"), o.Spec.UniqueName, apimachineryvalidation.NameIsDNSLabel)...)
	}

	return errors
}

// invalidName reports each message a name validator returns for value.
func invalidName(path *field.Path, value string, validate apimachineryvalidation.ValidateNameFunc) field.ErrorList {
	var errs field.ErrorList
	for _, msg := range validate(value, false) {
		errs = append(errs, field.Invalid(path, value, msg))
	}

	return errs
}
