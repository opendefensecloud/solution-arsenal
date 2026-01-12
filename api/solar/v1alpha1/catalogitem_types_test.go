// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCatalogItemCategory_Values(t *testing.T) {
	// Verify all category constants are defined correctly
	categories := []CatalogItemCategory{
		CatalogItemCategoryApplication,
		CatalogItemCategoryOperator,
		CatalogItemCategoryAddon,
		CatalogItemCategoryLibrary,
	}

	expected := []string{"Application", "Operator", "Addon", "Library"}
	for i, cat := range categories {
		if string(cat) != expected[i] {
			t.Errorf("expected category %q, got %q", expected[i], cat)
		}
	}
}

func TestCatalogItemPhase_Values(t *testing.T) {
	// Verify all phase constants are defined correctly
	phases := []CatalogItemPhase{
		CatalogItemPhaseDiscovered,
		CatalogItemPhaseValidating,
		CatalogItemPhaseAvailable,
		CatalogItemPhaseFailed,
		CatalogItemPhaseDeprecated,
	}

	expected := []string{"Discovered", "Validating", "Available", "Failed", "Deprecated"}
	for i, phase := range phases {
		if string(phase) != expected[i] {
			t.Errorf("expected phase %q, got %q", expected[i], phase)
		}
	}
}

func TestMaintainer_Fields(t *testing.T) {
	tests := []struct {
		name     string
		m        Maintainer
		wantName string
		wantMail string
	}{
		{
			name:     "full maintainer",
			m:        Maintainer{Name: "John Doe", Email: "john@example.com"},
			wantName: "John Doe",
			wantMail: "john@example.com",
		},
		{
			name:     "maintainer without email",
			m:        Maintainer{Name: "Jane Doe"},
			wantName: "Jane Doe",
			wantMail: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.m.Name != tt.wantName {
				t.Errorf("expected name %q, got %q", tt.wantName, tt.m.Name)
			}
			if tt.m.Email != tt.wantMail {
				t.Errorf("expected email %q, got %q", tt.wantMail, tt.m.Email)
			}
		})
	}
}

func TestAttestation_Fields(t *testing.T) {
	now := metav1.Now()
	attestation := Attestation{
		Type:      "vulnerability-scan",
		Issuer:    "security-team",
		Reference: "https://example.com/scan/123",
		Passed:    true,
		Timestamp: &now,
	}

	if attestation.Type != "vulnerability-scan" {
		t.Errorf("expected type %q, got %q", "vulnerability-scan", attestation.Type)
	}
	if attestation.Issuer != "security-team" {
		t.Errorf("expected issuer %q, got %q", "security-team", attestation.Issuer)
	}
	if attestation.Reference != "https://example.com/scan/123" {
		t.Errorf("expected reference %q, got %q", "https://example.com/scan/123", attestation.Reference)
	}
	if !attestation.Passed {
		t.Error("expected attestation to be passed")
	}
	if attestation.Timestamp == nil || attestation.Timestamp.Time.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestComponentDependency_Fields(t *testing.T) {
	tests := []struct {
		name                  string
		dep                   ComponentDependency
		wantComponent         string
		wantVersionConstraint string
		wantOptional          bool
	}{
		{
			name: "required dependency with version constraint",
			dep: ComponentDependency{
				ComponentName:     "github.com/myorg/base-component",
				VersionConstraint: ">=1.0.0",
				Optional:          false,
			},
			wantComponent:         "github.com/myorg/base-component",
			wantVersionConstraint: ">=1.0.0",
			wantOptional:          false,
		},
		{
			name: "optional dependency without version constraint",
			dep: ComponentDependency{
				ComponentName: "github.com/myorg/optional-component",
				Optional:      true,
			},
			wantComponent:         "github.com/myorg/optional-component",
			wantVersionConstraint: "",
			wantOptional:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dep.ComponentName != tt.wantComponent {
				t.Errorf("expected componentName %q, got %q", tt.wantComponent, tt.dep.ComponentName)
			}
			if tt.dep.VersionConstraint != tt.wantVersionConstraint {
				t.Errorf("expected versionConstraint %q, got %q", tt.wantVersionConstraint, tt.dep.VersionConstraint)
			}
			if tt.dep.Optional != tt.wantOptional {
				t.Errorf("expected optional %v, got %v", tt.wantOptional, tt.dep.Optional)
			}
		})
	}
}

func TestResourceRequirements_Fields(t *testing.T) {
	req := ResourceRequirements{
		CPUCores:  "2",
		MemoryMB:  "4096",
		StorageGB: "50",
	}

	if req.CPUCores != "2" {
		t.Errorf("expected cpuCores %q, got %q", "2", req.CPUCores)
	}
	if req.MemoryMB != "4096" {
		t.Errorf("expected memoryMB %q, got %q", "4096", req.MemoryMB)
	}
	if req.StorageGB != "50" {
		t.Errorf("expected storageGB %q, got %q", "50", req.StorageGB)
	}
}

func TestValidationCheck_Fields(t *testing.T) {
	now := metav1.Now()
	check := ValidationCheck{
		Name:      "signature-verification",
		Passed:    true,
		Message:   "Signature verified successfully",
		Timestamp: &now,
	}

	if check.Name != "signature-verification" {
		t.Errorf("expected name %q, got %q", "signature-verification", check.Name)
	}
	if !check.Passed {
		t.Error("expected check to be passed")
	}
	if check.Message != "Signature verified successfully" {
		t.Errorf("expected message %q, got %q", "Signature verified successfully", check.Message)
	}
	if check.Timestamp == nil || check.Timestamp.Time.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestValidationStatus_Fields(t *testing.T) {
	now := metav1.Now()
	status := ValidationStatus{
		Checks: []ValidationCheck{
			{Name: "check1", Passed: true},
			{Name: "check2", Passed: false, Message: "Failed to verify"},
		},
		LastValidatedAt: &now,
	}

	if len(status.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(status.Checks))
	}
	if status.Checks[0].Name != "check1" || !status.Checks[0].Passed {
		t.Error("first check not as expected")
	}
	if status.Checks[1].Name != "check2" || status.Checks[1].Passed {
		t.Error("second check not as expected")
	}
	if status.LastValidatedAt == nil || status.LastValidatedAt.Time.IsZero() {
		t.Error("expected lastValidatedAt to be set")
	}
}

func TestCatalogItemSpec_FullSpec(t *testing.T) {
	spec := CatalogItemSpec{
		ComponentName: "github.com/myorg/my-component",
		Version:       "1.0.0",
		Repository:    "ghcr.io/myorg/components",
		Description:   "A test component",
		Category:      CatalogItemCategoryApplication,
		Maintainers: []Maintainer{
			{Name: "Test User", Email: "test@example.com"},
		},
		Tags:                 []string{"test", "example"},
		RequiredAttestations: []string{"vulnerability-scan", "stig-compliance"},
		Dependencies: []ComponentDependency{
			{ComponentName: "github.com/myorg/base", VersionConstraint: ">=1.0.0"},
		},
		MinKubernetesVersion: "1.28",
		RequiredCapabilities: []string{"networking.k8s.io/v1/NetworkPolicy"},
		EstimatedResources: &ResourceRequirements{
			CPUCores:  "2",
			MemoryMB:  "4096",
			StorageGB: "50",
		},
		Deprecated:         false,
		DeprecationMessage: "",
	}

	// Verify required fields
	if spec.ComponentName != "github.com/myorg/my-component" {
		t.Errorf("expected componentName %q, got %q", "github.com/myorg/my-component", spec.ComponentName)
	}
	if spec.Version != "1.0.0" {
		t.Errorf("expected version %q, got %q", "1.0.0", spec.Version)
	}
	if spec.Repository != "ghcr.io/myorg/components" {
		t.Errorf("expected repository %q, got %q", "ghcr.io/myorg/components", spec.Repository)
	}

	// Verify optional fields
	if spec.Category != CatalogItemCategoryApplication {
		t.Errorf("expected category %q, got %q", CatalogItemCategoryApplication, spec.Category)
	}
	if len(spec.Maintainers) != 1 {
		t.Errorf("expected 1 maintainer, got %d", len(spec.Maintainers))
	}
	if len(spec.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(spec.Tags))
	}
	if len(spec.RequiredAttestations) != 2 {
		t.Errorf("expected 2 required attestations, got %d", len(spec.RequiredAttestations))
	}
	if len(spec.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(spec.Dependencies))
	}
	if spec.MinKubernetesVersion != "1.28" {
		t.Errorf("expected minKubernetesVersion %q, got %q", "1.28", spec.MinKubernetesVersion)
	}
	if len(spec.RequiredCapabilities) != 1 {
		t.Errorf("expected 1 required capability, got %d", len(spec.RequiredCapabilities))
	}
	if spec.EstimatedResources == nil {
		t.Error("expected estimatedResources to be set")
	}
}

func TestCatalogItemStatus_FullStatus(t *testing.T) {
	now := metav1.Now()
	status := CatalogItemStatus{
		Phase: CatalogItemPhaseAvailable,
		Validation: &ValidationStatus{
			Checks: []ValidationCheck{
				{Name: "signature", Passed: true},
			},
			LastValidatedAt: &now,
		},
		Attestations: []Attestation{
			{Type: "vuln-scan", Issuer: "security", Passed: true},
		},
		LastDiscoveredAt: &now,
		Conditions: []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				LastTransitionTime: now,
				Reason:             "Validated",
				Message:            "CatalogItem is ready",
			},
		},
		ObservedGeneration: 1,
	}

	if status.Phase != CatalogItemPhaseAvailable {
		t.Errorf("expected phase %q, got %q", CatalogItemPhaseAvailable, status.Phase)
	}
	if status.Validation == nil {
		t.Error("expected validation to be set")
	}
	if len(status.Attestations) != 1 {
		t.Errorf("expected 1 attestation, got %d", len(status.Attestations))
	}
	if status.LastDiscoveredAt == nil {
		t.Error("expected lastDiscoveredAt to be set")
	}
	if len(status.Conditions) != 1 {
		t.Errorf("expected 1 condition, got %d", len(status.Conditions))
	}
	if status.ObservedGeneration != 1 {
		t.Errorf("expected observedGeneration 1, got %d", status.ObservedGeneration)
	}
}

func TestCatalogItem_FullObject(t *testing.T) {
	now := metav1.Now()
	item := CatalogItem{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "solar.opendefense.cloud/v1alpha1",
			Kind:       "CatalogItem",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-item",
			Namespace: "default",
		},
		Spec: CatalogItemSpec{
			ComponentName: "github.com/myorg/test",
			Version:       "1.0.0",
			Repository:    "ghcr.io/myorg",
		},
		Status: CatalogItemStatus{
			Phase:            CatalogItemPhaseAvailable,
			LastDiscoveredAt: &now,
		},
	}

	if item.APIVersion != "solar.opendefense.cloud/v1alpha1" {
		t.Errorf("expected apiVersion %q, got %q", "solar.opendefense.cloud/v1alpha1", item.APIVersion)
	}
	if item.Kind != "CatalogItem" {
		t.Errorf("expected kind %q, got %q", "CatalogItem", item.Kind)
	}
	if item.Name != "test-item" {
		t.Errorf("expected name %q, got %q", "test-item", item.Name)
	}
	if item.Namespace != "default" {
		t.Errorf("expected namespace %q, got %q", "default", item.Namespace)
	}
}

func TestCatalogItemList_Fields(t *testing.T) {
	list := CatalogItemList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "solar.opendefense.cloud/v1alpha1",
			Kind:       "CatalogItemList",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "12345",
		},
		Items: []CatalogItem{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "item1", Namespace: "default"},
				Spec:       CatalogItemSpec{ComponentName: "comp1", Version: "1.0.0", Repository: "ghcr.io/test"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "item2", Namespace: "default"},
				Spec:       CatalogItemSpec{ComponentName: "comp2", Version: "2.0.0", Repository: "ghcr.io/test"},
			},
		},
	}

	if len(list.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(list.Items))
	}
	if list.Items[0].Name != "item1" {
		t.Errorf("expected first item name %q, got %q", "item1", list.Items[0].Name)
	}
	if list.Items[1].Name != "item2" {
		t.Errorf("expected second item name %q, got %q", "item2", list.Items[1].Name)
	}
}

func TestCatalogItemSpec_DeprecatedItem(t *testing.T) {
	spec := CatalogItemSpec{
		ComponentName:      "github.com/myorg/old-component",
		Version:            "1.0.0",
		Repository:         "ghcr.io/myorg",
		Deprecated:         true,
		DeprecationMessage: "Please use new-component instead",
	}

	if !spec.Deprecated {
		t.Error("expected item to be deprecated")
	}
	if spec.DeprecationMessage != "Please use new-component instead" {
		t.Errorf("expected deprecation message %q, got %q", "Please use new-component instead", spec.DeprecationMessage)
	}
}

func TestAttestation_WithoutOptionalFields(t *testing.T) {
	attestation := Attestation{
		Type:   "signature",
		Issuer: "cosign",
		Passed: true,
	}

	if attestation.Reference != "" {
		t.Errorf("expected empty reference, got %q", attestation.Reference)
	}
	if attestation.Timestamp != nil {
		t.Error("expected nil timestamp")
	}
}

func TestResourceRequirements_PartialValues(t *testing.T) {
	// Test with only some values set
	req := ResourceRequirements{
		CPUCores: "1.5",
	}

	if req.CPUCores != "1.5" {
		t.Errorf("expected cpuCores %q, got %q", "1.5", req.CPUCores)
	}
	if req.MemoryMB != "" {
		t.Errorf("expected empty memoryMB, got %q", req.MemoryMB)
	}
	if req.StorageGB != "" {
		t.Errorf("expected empty storageGB, got %q", req.StorageGB)
	}
}

func TestCondition_StandardFields(t *testing.T) {
	now := metav1.NewTime(time.Now())
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: 1,
		LastTransitionTime: now,
		Reason:             "AllChecksPass",
		Message:            "All validation checks passed",
	}

	if condition.Type != "Ready" {
		t.Errorf("expected type %q, got %q", "Ready", condition.Type)
	}
	if condition.Status != metav1.ConditionTrue {
		t.Errorf("expected status %q, got %q", metav1.ConditionTrue, condition.Status)
	}
	if condition.Reason != "AllChecksPass" {
		t.Errorf("expected reason %q, got %q", "AllChecksPass", condition.Reason)
	}
}
