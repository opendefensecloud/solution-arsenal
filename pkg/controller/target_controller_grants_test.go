// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func newRegistryGrant(fromNamespace, toKind string) *solarv1alpha1.ReferenceGrant {
	return &solarv1alpha1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grant",
			Namespace: "registry-ns",
		},
		Spec: solarv1alpha1.ReferenceGrantSpec{
			From: []solarv1alpha1.ReferenceGrantFromSubject{
				{Group: solarGroup, Kind: "Target", Namespace: fromNamespace},
			},
			To: []solarv1alpha1.ReferenceGrantToTarget{
				{Group: solarGroup, Kind: toKind},
			},
		},
	}
}

func TestGrantPermitsRegistryAccess(t *testing.T) {
	t.Parallel()

	t.Run("permits when From matches Target/fromNamespace and To lists Registry", func(t *testing.T) {
		t.Parallel()
		grant := newRegistryGrant("target-ns", "Registry")
		if !grantPermitsRegistryAccess(grant, "target-ns") {
			t.Error("expected access to be permitted")
		}
	})

	t.Run("denies when fromNamespace does not match", func(t *testing.T) {
		t.Parallel()
		grant := newRegistryGrant("target-ns", "Registry")
		if grantPermitsRegistryAccess(grant, "other-ns") {
			t.Error("expected access to be denied")
		}
	})

	t.Run("denies when To does not list Registry", func(t *testing.T) {
		t.Parallel()
		grant := newRegistryGrant("target-ns", "Profile")
		if grantPermitsRegistryAccess(grant, "target-ns") {
			t.Error("expected access to be denied")
		}
	})
}

func TestRegistryGranted(t *testing.T) {
	t.Parallel()

	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = solarv1alpha1.AddToScheme(sch)

	t.Run("returns true when a matching grant exists in the registry namespace", func(t *testing.T) {
		t.Parallel()
		grant := newRegistryGrant("target-ns", "Registry")
		c := fake.NewClientBuilder().WithScheme(sch).WithObjects(grant).Build()
		r := &TargetReconciler{Client: c, Scheme: sch}

		granted, err := r.registryGranted(context.Background(), "registry-ns", "target-ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !granted {
			t.Error("expected registry access to be granted")
		}
	})

	t.Run("returns false when no grant exists", func(t *testing.T) {
		t.Parallel()
		c := fake.NewClientBuilder().WithScheme(sch).Build()
		r := &TargetReconciler{Client: c, Scheme: sch}

		granted, err := r.registryGranted(context.Background(), "registry-ns", "target-ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if granted {
			t.Error("expected registry access to be denied")
		}
	})

	t.Run("returns false when the only grant is for a different namespace", func(t *testing.T) {
		t.Parallel()
		grant := newRegistryGrant("other-ns", "Registry")
		c := fake.NewClientBuilder().WithScheme(sch).WithObjects(grant).Build()
		r := &TargetReconciler{Client: c, Scheme: sch}

		granted, err := r.registryGranted(context.Background(), "registry-ns", "target-ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if granted {
			t.Error("expected registry access to be denied")
		}
	})

	t.Run("returns an error when the List call fails", func(t *testing.T) {
		t.Parallel()
		// Scheme without ReferenceGrant registered so listing fails.
		bareScheme := runtime.NewScheme()
		_ = scheme.AddToScheme(bareScheme)
		c := fake.NewClientBuilder().WithScheme(bareScheme).Build()
		r := &TargetReconciler{Client: c, Scheme: bareScheme}

		granted, err := r.registryGranted(context.Background(), "registry-ns", "target-ns")
		if err == nil {
			t.Fatal("expected an error when List fails")
		}
		if granted {
			t.Error("expected registry access to be denied on error")
		}
	})
}

func TestGrantsRegistryResource(t *testing.T) {
	t.Parallel()

	t.Run("permits when To lists a Registry resource", func(t *testing.T) {
		t.Parallel()
		grant := newRegistryGrant("target-ns", "Registry")
		if !grantsRegistryResource(grant) {
			t.Error("expected grant to authorize Registry resources")
		}
	})

	t.Run("denies when To lists a non-Registry resource", func(t *testing.T) {
		t.Parallel()
		grant := newRegistryGrant("target-ns", "Profile")
		if grantsRegistryResource(grant) {
			t.Error("expected grant not to authorize Registry resources")
		}
	})
}
