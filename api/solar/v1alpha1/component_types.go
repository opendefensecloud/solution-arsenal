// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentSpec defines the desired state of a Component.
// It contains metadata about an OCM component's repository location
type ComponentSpec struct {
	// Scheme is the scheme to access the component.
	Scheme string `json:"scheme"`

	// Registry is the registry where the component is stored.
	Registry string `json:"registry"`

	// Repository is the repository where the component is stored.
	Repository string `json:"repository"`

	// Name is the raw OCM component name (e.g. "opendefense.cloud/arc").
	// Together with Scheme, Registry, Repository and a ComponentVersion's
	// Tag it forms the OCM component version reference the renderer resolves.
	Name string `json:"name"`
}

// ComponentStatus defines the observed state of a Component.
type ComponentStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Component represents an OCM component available in the solution catalog.
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ComponentSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ComponentStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComponentList contains a list of Component resources.
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Component `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// OCMRef returns the OCM component version reference for tag, in the form
// "<scheme>://<registry>/<namespace>//<name>:<tag>" — the form
// ocm-kit's compver.SplitRef expects.
//
// Discovery stores Repository as "<namespace>/<name>"
//
// Returns "" when Name is unset, which is the case for Components written
// before the field existed. Callers treat that as "no values template".
// Components written before Spec.Name existed cannot be resolved back to
// an OCM reference, since the object name is sanitized and lossy.
func (c *Component) OCMRef(tag string) string {
	if c.Spec.Name == "" {
		return ""
	}

	namespace := strings.Trim(strings.TrimSuffix(c.Spec.Repository, c.Spec.Name), "/")

	return fmt.Sprintf("%s://%s/%s//%s:%s", c.Spec.Scheme, c.Spec.Registry, namespace, c.Spec.Name, tag)
}

func (c *Component) GetSingularName() string {
	return "component"
}

func (c *Component) ShortNames() []string {
	return []string{"comp"}
}
