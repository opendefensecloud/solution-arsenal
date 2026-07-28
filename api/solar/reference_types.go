// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

// ObjectReference references another resource by name, optionally in a different
// namespace. When Namespace is empty, the referenced resource is assumed to live in
// the same namespace as the referencing object. Cross-namespace references require a
// ReferenceGrant in the referenced resource's namespace that grants access to the
// referencing object's namespace.
type ObjectReference struct {
	// Name is the name of the referenced resource.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace is the namespace of the referenced resource. If empty, the resource is
	// assumed to be in the same namespace as the referencing object.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}
