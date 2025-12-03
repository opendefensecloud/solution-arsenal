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
	"net/url"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	solarv1alpha1 "github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1"
)

// semverRegex matches semantic versions (e.g., 1.0.0, v1.2.3-beta.1+build.123)
var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

// ocmComponentNameRegex matches OCM component names (e.g., github.com/org/repo)
var ocmComponentNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?(/[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?)*$`)

// CatalogItemValidator validates CatalogItem resources.
type CatalogItemValidator struct {
	CommonValidator
}

// NewCatalogItemValidator creates a new CatalogItem validator.
func NewCatalogItemValidator() *CatalogItemValidator {
	return &CatalogItemValidator{}
}

// ValidateCreate validates a CatalogItem on creation.
func (v *CatalogItemValidator) ValidateCreate(ctx context.Context, obj runtime.Object) field.ErrorList {
	item, ok := obj.(*solarv1alpha1.CatalogItem)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	return v.validateCatalogItem(item)
}

// ValidateUpdate validates a CatalogItem on update.
func (v *CatalogItemValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) field.ErrorList {
	oldItem, ok := oldObj.(*solarv1alpha1.CatalogItem)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	newItem, ok := newObj.(*solarv1alpha1.CatalogItem)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	errs := v.validateCatalogItem(newItem)

	// Validate immutable fields
	specPath := field.NewPath("spec")
	errs = append(errs, v.validateImmutableFields(oldItem, newItem, specPath)...)

	return errs
}

// ValidateDelete validates a CatalogItem on deletion.
func (v *CatalogItemValidator) ValidateDelete(ctx context.Context, obj runtime.Object) field.ErrorList {
	// No special validation on delete
	return nil
}

func (v *CatalogItemValidator) validateCatalogItem(item *solarv1alpha1.CatalogItem) field.ErrorList {
	var errs field.ErrorList

	specPath := field.NewPath("spec")

	// Validate component name
	errs = append(errs, v.validateComponentName(item.Spec.ComponentName, specPath.Child("componentName"))...)

	// Validate version
	errs = append(errs, v.validateVersion(item.Spec.Version, specPath.Child("version"))...)

	// Validate repository
	errs = append(errs, v.validateRepository(item.Spec.Repository, specPath.Child("repository"))...)

	// Validate labels
	if item.Spec.Labels != nil {
		errs = append(errs, v.ValidateLabels(item.Spec.Labels, specPath.Child("labels"))...)
	}

	// Validate dependencies
	errs = append(errs, v.validateDependencies(item.Spec.Dependencies, specPath.Child("dependencies"))...)

	return errs
}

func (v *CatalogItemValidator) validateComponentName(name string, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	if name == "" {
		errs = append(errs, field.Required(fldPath, "componentName is required"))
		return errs
	}

	if len(name) > 255 {
		errs = append(errs, field.TooLong(fldPath, name, 255))
	}

	if !ocmComponentNameRegex.MatchString(name) {
		errs = append(errs, field.Invalid(fldPath, name,
			"must be a valid OCM component name (e.g., github.com/org/component)"))
	}

	return errs
}

func (v *CatalogItemValidator) validateVersion(version string, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	if version == "" {
		errs = append(errs, field.Required(fldPath, "version is required"))
		return errs
	}

	if !semverRegex.MatchString(version) {
		errs = append(errs, field.Invalid(fldPath, version,
			"must be a valid semantic version (e.g., 1.0.0, v1.2.3-beta.1)"))
	}

	return errs
}

func (v *CatalogItemValidator) validateRepository(repo string, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	if repo == "" {
		errs = append(errs, field.Required(fldPath, "repository is required"))
		return errs
	}

	// Check if it's a valid URL or OCI reference
	if strings.HasPrefix(repo, "oci://") {
		// OCI reference format
		ociRef := strings.TrimPrefix(repo, "oci://")
		if !isValidOCIReference(ociRef) {
			errs = append(errs, field.Invalid(fldPath, repo,
				"must be a valid OCI reference (e.g., oci://ghcr.io/org/repo)"))
		}
	} else if strings.Contains(repo, "://") {
		// URL format
		if _, err := url.Parse(repo); err != nil {
			errs = append(errs, field.Invalid(fldPath, repo,
				"must be a valid URL"))
		}
	} else {
		// Assume it's a registry/repo format (e.g., ghcr.io/org/repo)
		if !isValidOCIReference(repo) {
			errs = append(errs, field.Invalid(fldPath, repo,
				"must be a valid OCI registry reference"))
		}
	}

	return errs
}

func (v *CatalogItemValidator) validateDependencies(deps []solarv1alpha1.ComponentReference, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	seen := make(map[string]bool)
	for i, dep := range deps {
		idxPath := fldPath.Index(i)

		// Validate dependency name
		if dep.Name == "" {
			errs = append(errs, field.Required(idxPath.Child("name"), "dependency name is required"))
		} else {
			// Check for duplicates
			if seen[dep.Name] {
				errs = append(errs, field.Duplicate(idxPath.Child("name"), dep.Name))
			}
			seen[dep.Name] = true

			// Validate name format
			if !ocmComponentNameRegex.MatchString(dep.Name) {
				errs = append(errs, field.Invalid(idxPath.Child("name"), dep.Name,
					"must be a valid OCM component name"))
			}
		}

		// Validate version if specified
		if dep.Version != "" && !semverRegex.MatchString(dep.Version) {
			errs = append(errs, field.Invalid(idxPath.Child("version"), dep.Version,
				"must be a valid semantic version"))
		}
	}

	return errs
}

func (v *CatalogItemValidator) validateImmutableFields(oldItem, newItem *solarv1alpha1.CatalogItem, specPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	// Component name is immutable
	if oldItem.Spec.ComponentName != newItem.Spec.ComponentName {
		errs = append(errs, field.Forbidden(specPath.Child("componentName"),
			"componentName is immutable"))
	}

	// Version is immutable (create a new CatalogItem for new versions)
	if oldItem.Spec.Version != newItem.Spec.Version {
		errs = append(errs, field.Forbidden(specPath.Child("version"),
			"version is immutable; create a new CatalogItem for a different version"))
	}

	return errs
}

// isValidOCIReference checks if a string is a valid OCI registry reference.
func isValidOCIReference(ref string) bool {
	// Basic validation for OCI references
	// Format: registry/namespace/repository[:tag][@digest]
	if ref == "" {
		return false
	}

	// Must contain at least one slash (registry/repo)
	if !strings.Contains(ref, "/") {
		return false
	}

	// Split off tag/digest
	refPart := ref
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		refPart = ref[:idx]
	}
	if idx := strings.LastIndex(refPart, ":"); idx != -1 {
		// Check if it's a port (e.g., localhost:5000) or tag
		afterColon := refPart[idx+1:]
		if !strings.Contains(afterColon, "/") {
			refPart = refPart[:idx]
		}
	}

	// Check each component
	parts := strings.Split(refPart, "/")
	for _, part := range parts {
		if part == "" {
			return false
		}
		// Basic character validation
		for _, c := range part {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' || c == ':') {
				return false
			}
		}
	}

	return true
}

// ClusterCatalogItemValidator validates ClusterCatalogItem resources.
// It uses the same validation logic as CatalogItemValidator.
type ClusterCatalogItemValidator struct {
	CatalogItemValidator
}

// NewClusterCatalogItemValidator creates a new ClusterCatalogItem validator.
func NewClusterCatalogItemValidator() *ClusterCatalogItemValidator {
	return &ClusterCatalogItemValidator{
		CatalogItemValidator: *NewCatalogItemValidator(),
	}
}

// ValidateCreate validates a ClusterCatalogItem on creation.
func (v *ClusterCatalogItemValidator) ValidateCreate(ctx context.Context, obj runtime.Object) field.ErrorList {
	item, ok := obj.(*solarv1alpha1.ClusterCatalogItem)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	// Convert to CatalogItem for validation (same spec structure)
	catalogItem := &solarv1alpha1.CatalogItem{
		Spec: item.Spec,
	}

	return v.validateCatalogItem(catalogItem)
}

// ValidateUpdate validates a ClusterCatalogItem on update.
func (v *ClusterCatalogItemValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) field.ErrorList {
	oldItem, ok := oldObj.(*solarv1alpha1.ClusterCatalogItem)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	newItem, ok := newObj.(*solarv1alpha1.ClusterCatalogItem)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	// Convert to CatalogItem for validation
	oldCatalogItem := &solarv1alpha1.CatalogItem{Spec: oldItem.Spec}
	newCatalogItem := &solarv1alpha1.CatalogItem{Spec: newItem.Spec}

	errs := v.validateCatalogItem(newCatalogItem)
	errs = append(errs, v.validateImmutableFields(oldCatalogItem, newCatalogItem, field.NewPath("spec"))...)

	return errs
}

// ValidateDelete validates a ClusterCatalogItem on deletion.
func (v *ClusterCatalogItemValidator) ValidateDelete(ctx context.Context, obj runtime.Object) field.ErrorList {
	return nil
}
