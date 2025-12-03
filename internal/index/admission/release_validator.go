/*
Copyright 2024 Open Defense Cloud Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package admission

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	solarv1alpha1 "github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1"
)

// ReleaseValidator validates Release resources.
type ReleaseValidator struct {
	CommonValidator
	// catalogItemLister is used to verify catalog item references exist.
	// In production, this would be an actual lister.
	catalogItemLister CatalogItemLister
	// clusterLister is used to verify cluster references exist.
	clusterLister ClusterRegistrationLister
}

// CatalogItemLister is an interface for looking up CatalogItems.
type CatalogItemLister interface {
	Exists(namespace, name string) bool
}

// ClusterRegistrationLister is an interface for looking up ClusterRegistrations.
type ClusterRegistrationLister interface {
	Exists(namespace, name string) bool
}

// ReleaseValidatorOption is a functional option for ReleaseValidator.
type ReleaseValidatorOption func(*ReleaseValidator)

// WithCatalogItemLister sets the catalog item lister.
func WithCatalogItemLister(lister CatalogItemLister) ReleaseValidatorOption {
	return func(v *ReleaseValidator) {
		v.catalogItemLister = lister
	}
}

// WithClusterRegistrationLister sets the cluster registration lister.
func WithClusterRegistrationLister(lister ClusterRegistrationLister) ReleaseValidatorOption {
	return func(v *ReleaseValidator) {
		v.clusterLister = lister
	}
}

// NewReleaseValidator creates a new Release validator.
func NewReleaseValidator(opts ...ReleaseValidatorOption) *ReleaseValidator {
	v := &ReleaseValidator{}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// ValidateCreate validates a Release on creation.
func (v *ReleaseValidator) ValidateCreate(ctx context.Context, obj runtime.Object) field.ErrorList {
	release, ok := obj.(*solarv1alpha1.Release)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	return v.validateRelease(ctx, release, nil)
}

// ValidateUpdate validates a Release on update.
func (v *ReleaseValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) field.ErrorList {
	oldRelease, ok := oldObj.(*solarv1alpha1.Release)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	newRelease, ok := newObj.(*solarv1alpha1.Release)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	errs := v.validateRelease(ctx, newRelease, oldRelease)
	errs = append(errs, v.validateImmutableFields(oldRelease, newRelease)...)

	return errs
}

// ValidateDelete validates a Release on deletion.
func (v *ReleaseValidator) ValidateDelete(ctx context.Context, obj runtime.Object) field.ErrorList {
	// No special validation on delete
	return nil
}

func (v *ReleaseValidator) validateRelease(ctx context.Context, release *solarv1alpha1.Release, oldRelease *solarv1alpha1.Release) field.ErrorList {
	var errs field.ErrorList

	specPath := field.NewPath("spec")

	// Validate catalog item reference
	errs = append(errs, v.validateCatalogItemRef(ctx, release, specPath.Child("catalogItemRef"))...)

	// Validate target cluster reference
	errs = append(errs, v.validateTargetClusterRef(ctx, release, specPath.Child("targetClusterRef"))...)

	// Validate values (if present)
	if release.Spec.Values.Raw != nil {
		errs = append(errs, v.validateValues(release.Spec.Values.Raw, specPath.Child("values"))...)
	}

	return errs
}

func (v *ReleaseValidator) validateCatalogItemRef(ctx context.Context, release *solarv1alpha1.Release, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	ref := release.Spec.CatalogItemRef

	// Name is required
	if ref.Name == "" {
		errs = append(errs, field.Required(fldPath.Child("name"), "catalog item name is required"))
		return errs
	}

	// Validate name length
	if len(ref.Name) > 253 {
		errs = append(errs, field.TooLong(fldPath.Child("name"), ref.Name, 253))
	}

	// Check if catalog item exists (if lister is configured)
	if v.catalogItemLister != nil {
		namespace := ref.Namespace
		if namespace == "" {
			namespace = release.Namespace
		}
		if !v.catalogItemLister.Exists(namespace, ref.Name) {
			errs = append(errs, field.NotFound(fldPath, ref.Name))
		}
	}

	return errs
}

func (v *ReleaseValidator) validateTargetClusterRef(ctx context.Context, release *solarv1alpha1.Release, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	ref := release.Spec.TargetClusterRef

	// Name is required
	if ref.Name == "" {
		errs = append(errs, field.Required(fldPath.Child("name"), "target cluster name is required"))
		return errs
	}

	// Validate name length
	if len(ref.Name) > 253 {
		errs = append(errs, field.TooLong(fldPath.Child("name"), ref.Name, 253))
	}

	// Check if cluster registration exists (if lister is configured)
	if v.clusterLister != nil {
		namespace := ref.Namespace
		if namespace == "" {
			namespace = release.Namespace
		}
		if !v.clusterLister.Exists(namespace, ref.Name) {
			errs = append(errs, field.NotFound(fldPath, ref.Name))
		}
	}

	return errs
}

func (v *ReleaseValidator) validateValues(values []byte, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	// Values should be valid JSON
	if len(values) > 0 {
		// Check size limit (1MB)
		if len(values) > 1024*1024 {
			errs = append(errs, field.TooLong(fldPath, "<values>", 1024*1024))
		}
	}

	return errs
}

func (v *ReleaseValidator) validateImmutableFields(oldRelease, newRelease *solarv1alpha1.Release) field.ErrorList {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	// Target cluster reference is immutable
	if oldRelease.Spec.TargetClusterRef.Name != newRelease.Spec.TargetClusterRef.Name {
		errs = append(errs, field.Forbidden(specPath.Child("targetClusterRef", "name"),
			"targetClusterRef.name is immutable"))
	}
	if oldRelease.Spec.TargetClusterRef.Namespace != newRelease.Spec.TargetClusterRef.Namespace {
		errs = append(errs, field.Forbidden(specPath.Child("targetClusterRef", "namespace"),
			"targetClusterRef.namespace is immutable"))
	}

	return errs
}

// SyncValidator validates Sync resources.
type SyncValidator struct {
	CommonValidator
}

// NewSyncValidator creates a new Sync validator.
func NewSyncValidator() *SyncValidator {
	return &SyncValidator{}
}

// ValidateCreate validates a Sync on creation.
func (v *SyncValidator) ValidateCreate(ctx context.Context, obj runtime.Object) field.ErrorList {
	sync, ok := obj.(*solarv1alpha1.Sync)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	return v.validateSync(sync)
}

// ValidateUpdate validates a Sync on update.
func (v *SyncValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) field.ErrorList {
	newSync, ok := newObj.(*solarv1alpha1.Sync)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	return v.validateSync(newSync)
}

// ValidateDelete validates a Sync on deletion.
func (v *SyncValidator) ValidateDelete(ctx context.Context, obj runtime.Object) field.ErrorList {
	return nil
}

func (v *SyncValidator) validateSync(sync *solarv1alpha1.Sync) field.ErrorList {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	// Validate source reference
	if sync.Spec.SourceRef.Name == "" {
		errs = append(errs, field.Required(specPath.Child("sourceRef", "name"), "source name is required"))
	}

	// Validate destination registry
	if sync.Spec.DestinationRegistry == "" {
		errs = append(errs, field.Required(specPath.Child("destinationRegistry"), "destination registry is required"))
	} else if !isValidOCIReference(sync.Spec.DestinationRegistry) {
		errs = append(errs, field.Invalid(specPath.Child("destinationRegistry"), sync.Spec.DestinationRegistry,
			"must be a valid OCI registry reference"))
	}

	// Validate filter labels
	if sync.Spec.Filter.IncludeLabels != nil {
		errs = append(errs, v.ValidateLabels(sync.Spec.Filter.IncludeLabels, specPath.Child("filter", "includeLabels"))...)
	}
	if sync.Spec.Filter.ExcludeLabels != nil {
		errs = append(errs, v.ValidateLabels(sync.Spec.Filter.ExcludeLabels, specPath.Child("filter", "excludeLabels"))...)
	}

	return errs
}
