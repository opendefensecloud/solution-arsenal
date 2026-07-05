// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VerificationConfig defines signature verification settings for a registry.
type VerificationConfig struct {
	// Enabled enables signature verification for artifacts from this registry.
	Enabled bool `json:"enabled"`
	// KeySecretRef references a Secret in the same namespace containing trusted
	// public keys for signature verification.
	// +optional
	KeySecretRef *corev1.LocalObjectReference `json:"keySecretRef,omitempty"`
}

// RegistrySpec defines the desired state of a Registry.
type RegistrySpec struct {
	// Hostname is the registry endpoint (e.g. "registry.example.com:5000").
	Hostname string `json:"hostname"`
	// PlainHTTP uses HTTP instead of HTTPS for connections to this registry.
	// +optional
	PlainHTTP bool `json:"plainHTTP,omitempty"`
	// SolarSecretRef references a Secret in the same namespace with credentials
	// to access this registry from the SolAr cluster. Required if this registry
	// is used as a render target.
	// +optional
	SolarSecretRef *corev1.LocalObjectReference `json:"solarSecretRef,omitempty"`
	// TargetPullSecretName is the name of the Secret on the target cluster that
	// contains credentials to pull from this registry. SolAr renders this name
	// into target manifests (e.g. Flux OCIRepository.spec.secretRef.name) but
	// never reads the Secret itself. The cluster maintainer must provision a
	// Secret with this name on each target. Omit for anonymous pull.
	// +optional
	TargetPullSecretName string `json:"targetPullSecretName,omitempty"`
	// Flavor identifies the registry type for discovery webhook routing (e.g. "zot").
	// Required when WebhookPath is set.
	// +optional
	Flavor string `json:"flavor,omitempty"`
	// WebhookPath is the HTTP path on which the discovery worker listens for
	// push notifications from this registry. Leave empty to disable webhook-based
	// discovery; set ScanInterval to enable scan mode instead.
	// +optional
	WebhookPath string `json:"webhookPath,omitempty"`
	// ScanInterval controls how often the discovery worker performs a full scan
	// of this registry. Leave unset to disable scan mode entirely.
	// +optional
	ScanInterval *metav1.Duration `json:"scanInterval,omitempty"`
	// Verification configures cosign signature verification for artifacts
	// discovered from this registry.
	// +optional
	Verification *VerificationConfig `json:"verification,omitempty"`
}

// RegistryStatus defines the observed state of a Registry.
type RegistryStatus struct {
	// Conditions represent the latest available observations of a Registry's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Registry represents an OCI registry that can be used as a source or destination for artifacts.
type Registry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   RegistrySpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status RegistryStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RegistryList contains a list of Registry resources.
type RegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Registry `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (r *Registry) GetSingularName() string {
	return "registry"
}

func (r *Registry) ShortNames() []string {
	return []string{"reg"}
}

// GetURL returns the base URL of this registry, including scheme.
func (r *Registry) GetURL() string {
	scheme := "https"
	if r.Spec.PlainHTTP {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, r.Spec.Hostname)
}
