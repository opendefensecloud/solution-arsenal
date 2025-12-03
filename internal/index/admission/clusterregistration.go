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
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	solarv1alpha1 "github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1"
)

// ClusterRegistrationValidator validates ClusterRegistration resources.
type ClusterRegistrationValidator struct {
	CommonValidator
}

// NewClusterRegistrationValidator creates a new ClusterRegistration validator.
func NewClusterRegistrationValidator() *ClusterRegistrationValidator {
	return &ClusterRegistrationValidator{}
}

// ValidateCreate validates a ClusterRegistration on creation.
func (v *ClusterRegistrationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) field.ErrorList {
	cr, ok := obj.(*solarv1alpha1.ClusterRegistration)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	return v.validateClusterRegistration(cr)
}

// ValidateUpdate validates a ClusterRegistration on update.
func (v *ClusterRegistrationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) field.ErrorList {
	oldCR, ok := oldObj.(*solarv1alpha1.ClusterRegistration)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	newCR, ok := newObj.(*solarv1alpha1.ClusterRegistration)
	if !ok {
		return field.ErrorList{field.InternalError(nil, nil)}
	}

	errs := v.validateClusterRegistration(newCR)
	errs = append(errs, v.validateImmutableFields(oldCR, newCR)...)

	return errs
}

// ValidateDelete validates a ClusterRegistration on deletion.
func (v *ClusterRegistrationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) field.ErrorList {
	// Could add checks for active releases here
	return nil
}

func (v *ClusterRegistrationValidator) validateClusterRegistration(cr *solarv1alpha1.ClusterRegistration) field.ErrorList {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	// DisplayName is required
	if cr.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	} else if len(cr.Spec.DisplayName) > 256 {
		errs = append(errs, field.TooLong(specPath.Child("displayName"), cr.Spec.DisplayName, 256))
	}

	// Description length check
	if len(cr.Spec.Description) > 2048 {
		errs = append(errs, field.TooLong(specPath.Child("description"), cr.Spec.Description, 2048))
	}

	// Validate labels
	if cr.Spec.Labels != nil {
		errs = append(errs, v.ValidateLabels(cr.Spec.Labels, specPath.Child("labels"))...)
	}

	// Validate agent config
	errs = append(errs, v.validateAgentConfig(&cr.Spec.AgentConfig, specPath.Child("agentConfig"))...)

	return errs
}

func (v *ClusterRegistrationValidator) validateAgentConfig(config *solarv1alpha1.AgentConfiguration, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList

	// If ARCEndpoint is specified, validate it
	if config.ARCEndpoint != "" {
		// Basic URL validation
		if len(config.ARCEndpoint) > 2048 {
			errs = append(errs, field.TooLong(fldPath.Child("arcEndpoint"), config.ARCEndpoint, 2048))
		}
	}

	return errs
}

func (v *ClusterRegistrationValidator) validateImmutableFields(oldCR, newCR *solarv1alpha1.ClusterRegistration) field.ErrorList {
	var errs field.ErrorList

	// No immutable fields for ClusterRegistration currently
	// The name is already immutable by Kubernetes API conventions

	return errs
}

// ClusterRegistrationMutator mutates ClusterRegistration resources.
type ClusterRegistrationMutator struct {
	// tokenGenerator generates agent tokens
	tokenGenerator TokenGenerator
}

// TokenGenerator generates secure tokens for agent authentication.
type TokenGenerator interface {
	GenerateToken() (string, error)
}

// DefaultTokenGenerator generates random tokens.
type DefaultTokenGenerator struct {
	tokenLength int
}

// NewDefaultTokenGenerator creates a new default token generator.
func NewDefaultTokenGenerator(length int) *DefaultTokenGenerator {
	if length <= 0 {
		length = 32
	}
	return &DefaultTokenGenerator{
		tokenLength: length,
	}
}

// GenerateToken generates a random token.
func (g *DefaultTokenGenerator) GenerateToken() (string, error) {
	bytes := make([]byte, g.tokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// ClusterRegistrationMutatorOption is a functional option for ClusterRegistrationMutator.
type ClusterRegistrationMutatorOption func(*ClusterRegistrationMutator)

// WithTokenGenerator sets the token generator.
func WithTokenGenerator(gen TokenGenerator) ClusterRegistrationMutatorOption {
	return func(m *ClusterRegistrationMutator) {
		m.tokenGenerator = gen
	}
}

// NewClusterRegistrationMutator creates a new ClusterRegistration mutator.
func NewClusterRegistrationMutator(opts ...ClusterRegistrationMutatorOption) *ClusterRegistrationMutator {
	m := &ClusterRegistrationMutator{
		tokenGenerator: NewDefaultTokenGenerator(32),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// MutateCreate mutates a ClusterRegistration on creation.
func (m *ClusterRegistrationMutator) MutateCreate(ctx context.Context, obj runtime.Object) error {
	cr, ok := obj.(*solarv1alpha1.ClusterRegistration)
	if !ok {
		return fmt.Errorf("expected ClusterRegistration, got %T", obj)
	}

	// Set default agent configuration if not specified
	m.setAgentDefaults(cr)

	// Add finalizer for cleanup
	m.addFinalizer(cr)

	return nil
}

// MutateUpdate mutates a ClusterRegistration on update.
func (m *ClusterRegistrationMutator) MutateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	newCR, ok := newObj.(*solarv1alpha1.ClusterRegistration)
	if !ok {
		return fmt.Errorf("expected ClusterRegistration, got %T", newObj)
	}

	// Ensure finalizer is present
	m.addFinalizer(newCR)

	return nil
}

func (m *ClusterRegistrationMutator) setAgentDefaults(cr *solarv1alpha1.ClusterRegistration) {
	// AgentConfig defaults are already zero values, which are appropriate
	// SyncEnabled defaults to false (zero value for bool)
}

func (m *ClusterRegistrationMutator) addFinalizer(cr *solarv1alpha1.ClusterRegistration) {
	finalizerName := "solar.odc.io/cluster-registration-cleanup"

	// Check if finalizer already exists
	for _, f := range cr.Finalizers {
		if f == finalizerName {
			return
		}
	}

	// Add finalizer
	cr.Finalizers = append(cr.Finalizers, finalizerName)
}

// GenerateAgentCredentials generates credentials for the agent.
// This is typically called separately from mutation, e.g., via a subresource.
func (m *ClusterRegistrationMutator) GenerateAgentCredentials() (string, error) {
	return m.tokenGenerator.GenerateToken()
}
