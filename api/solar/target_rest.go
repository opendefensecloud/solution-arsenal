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
)

var _ resource.Object = &Target{}
var _ resource.ObjectWithStatusSubResource = &Target{}
var _ rest.PrepareForUpdater = &Target{}
var _ rest.PrepareForCreater = &Target{}
var _ rest.TableConverter = &Target{}
var _ rest.NameValidator = &Target{}

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

func (o *Target) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Render Registry", Type: "string"},
			{Name: "Bootstrap Version", Type: "integer"},
			{Name: "Age", Type: "string"},
		},
		[]any{o.Name, o.Spec.RenderRegistryRef.Name, o.Status.BootstrapVersion, duration.HumanDuration(metav1.Now().Sub(o.CreationTimestamp.Time))},
	), nil
}
