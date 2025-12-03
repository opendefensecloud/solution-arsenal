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

// Package admission provides admission webhooks for Solar API resources.
package admission

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validator is the interface for validating Solar API resources.
type Validator interface {
	// ValidateCreate validates a resource on creation.
	ValidateCreate(ctx context.Context, obj runtime.Object) field.ErrorList

	// ValidateUpdate validates a resource on update.
	ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) field.ErrorList

	// ValidateDelete validates a resource on deletion.
	ValidateDelete(ctx context.Context, obj runtime.Object) field.ErrorList
}

// ValidatorRegistry holds validators for different resource types.
type ValidatorRegistry struct {
	validators map[string]Validator
}

// NewValidatorRegistry creates a new validator registry.
func NewValidatorRegistry() *ValidatorRegistry {
	return &ValidatorRegistry{
		validators: make(map[string]Validator),
	}
}

// Register registers a validator for a resource type.
func (r *ValidatorRegistry) Register(kind string, v Validator) {
	r.validators[kind] = v
}

// Get returns the validator for a resource type.
func (r *ValidatorRegistry) Get(kind string) (Validator, bool) {
	v, ok := r.validators[kind]
	return v, ok
}

// ValidationResult contains the result of a validation.
type ValidationResult struct {
	Allowed bool
	Reason  string
	Errors  field.ErrorList
}

// NewValidationResult creates a successful validation result.
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Allowed: true,
	}
}

// Deny creates a denied validation result.
func (r *ValidationResult) Deny(reason string) *ValidationResult {
	r.Allowed = false
	r.Reason = reason
	return r
}

// WithErrors adds field errors to the result.
func (r *ValidationResult) WithErrors(errs field.ErrorList) *ValidationResult {
	if len(errs) > 0 {
		r.Allowed = false
		r.Errors = errs
		r.Reason = errs.ToAggregate().Error()
	}
	return r
}

// CommonValidator provides common validation logic.
type CommonValidator struct{}

// ValidateName validates a resource name.
func (v *CommonValidator) ValidateName(name string, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	if name == "" {
		errs = append(errs, field.Required(fldPath, "name is required"))
	}

	if len(name) > 253 {
		errs = append(errs, field.TooLong(fldPath, name, 253))
	}

	return errs
}

// ValidateNamespace validates a namespace.
func (v *CommonValidator) ValidateNamespace(namespace string, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	if namespace == "" {
		errs = append(errs, field.Required(fldPath, "namespace is required"))
	}

	if len(namespace) > 63 {
		errs = append(errs, field.TooLong(fldPath, namespace, 63))
	}

	return errs
}

// ValidateLabels validates resource labels.
func (v *CommonValidator) ValidateLabels(labels map[string]string, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	for key, value := range labels {
		if len(key) > 63 {
			errs = append(errs, field.TooLong(fldPath.Key(key), key, 63))
		}
		if len(value) > 63 {
			errs = append(errs, field.TooLong(fldPath.Key(key), value, 63))
		}
	}

	return errs
}

// ValidateImmutableField checks if an immutable field has been modified.
func (v *CommonValidator) ValidateImmutableField(oldValue, newValue interface{}, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	if oldValue != newValue {
		errs = append(errs, field.Forbidden(fldPath, fmt.Sprintf("field is immutable, was %v", oldValue)))
	}

	return errs
}
