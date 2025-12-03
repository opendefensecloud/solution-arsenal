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
)

// Mutator is the interface for mutating Solar API resources.
type Mutator interface {
	// MutateCreate mutates a resource on creation.
	MutateCreate(ctx context.Context, obj runtime.Object) error

	// MutateUpdate mutates a resource on update.
	MutateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error
}

// MutatorRegistry holds mutators for different resource types.
type MutatorRegistry struct {
	mutators map[string]Mutator
}

// NewMutatorRegistry creates a new mutator registry.
func NewMutatorRegistry() *MutatorRegistry {
	return &MutatorRegistry{
		mutators: make(map[string]Mutator),
	}
}

// Register registers a mutator for a resource type.
func (r *MutatorRegistry) Register(kind string, m Mutator) {
	r.mutators[kind] = m
}

// Get returns the mutator for a resource type.
func (r *MutatorRegistry) Get(kind string) (Mutator, bool) {
	m, ok := r.mutators[kind]
	return m, ok
}

// MutationResult contains the result of a mutation.
type MutationResult struct {
	Mutated bool
	Object  runtime.Object
	Error   error
}

// NewMutationResult creates a successful mutation result.
func NewMutationResult(obj runtime.Object, mutated bool) *MutationResult {
	return &MutationResult{
		Mutated: mutated,
		Object:  obj,
	}
}

// WithError creates an error mutation result.
func (r *MutationResult) WithError(err error) *MutationResult {
	r.Error = err
	return r
}

// ChainMutator chains multiple mutators together.
type ChainMutator struct {
	mutators []Mutator
}

// NewChainMutator creates a new chain mutator.
func NewChainMutator(mutators ...Mutator) *ChainMutator {
	return &ChainMutator{
		mutators: mutators,
	}
}

// MutateCreate applies all mutators in order.
func (c *ChainMutator) MutateCreate(ctx context.Context, obj runtime.Object) error {
	for _, m := range c.mutators {
		if err := m.MutateCreate(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}

// MutateUpdate applies all mutators in order.
func (c *ChainMutator) MutateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	for _, m := range c.mutators {
		if err := m.MutateUpdate(ctx, oldObj, newObj); err != nil {
			return err
		}
	}
	return nil
}

// DefaultsMutator sets default values on resources.
type DefaultsMutator struct {
	defaultFuncs map[string]func(runtime.Object)
}

// NewDefaultsMutator creates a new defaults mutator.
func NewDefaultsMutator() *DefaultsMutator {
	return &DefaultsMutator{
		defaultFuncs: make(map[string]func(runtime.Object)),
	}
}

// RegisterDefault registers a default function for a kind.
func (m *DefaultsMutator) RegisterDefault(kind string, fn func(runtime.Object)) {
	m.defaultFuncs[kind] = fn
}

// MutateCreate applies defaults on creation.
func (m *DefaultsMutator) MutateCreate(ctx context.Context, obj runtime.Object) error {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if fn, ok := m.defaultFuncs[kind]; ok {
		fn(obj)
	}
	return nil
}

// MutateUpdate is a no-op for defaults (only applied on create).
func (m *DefaultsMutator) MutateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return nil
}
