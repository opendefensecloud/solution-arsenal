// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func newEnsureRenderArtifactScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	sch := runtime.NewScheme()
	if err := scheme.AddToScheme(sch); err != nil {
		t.Fatalf("failed to add core scheme: %v", err)
	}
	if err := solarv1alpha1.AddToScheme(sch); err != nil {
		t.Fatalf("failed to add solar scheme: %v", err)
	}

	return sch
}

func TestEnsureRenderArtifact(t *testing.T) {
	t.Parallel()

	registryRef := solarv1alpha1.ObjectReference{Name: "reg"}
	rt := &solarv1alpha1.RenderTask{
		ObjectMeta: metav1.ObjectMeta{Name: "rt", Namespace: "ns"},
		Spec: solarv1alpha1.RenderTaskSpec{
			BaseURL:    "registry.example.com",
			Repository: "ns/repo",
			Tag:        "v1",
		},
	}

	t.Run("ignores a terminating artifact and does not recreate it", func(t *testing.T) {
		t.Parallel()

		now := metav1.NewTime(time.Now())
		terminating := &solarv1alpha1.RenderArtifact{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "art",
				Namespace:         "ns",
				DeletionTimestamp: &now,
				Finalizers:        []string{renderArtifactFinalizer},
			},
			Spec: solarv1alpha1.RenderArtifactSpec{
				BaseURL:       "registry.example.com",
				Repository:    "ns/repo",
				Tag:           "v1",
				RenderTaskRef: "rt",
			},
		}
		c := fake.NewClientBuilder().
			WithScheme(newEnsureRenderArtifactScheme(t)).
			WithObjects(terminating).
			Build()
		r := &TargetReconciler{Client: c}

		if err := r.ensureRenderArtifact(context.Background(), "art", rt, registryRef); err != nil {
			t.Fatalf("expected nil error for a terminating RenderArtifact, got %v", err)
		}

		got := &solarv1alpha1.RenderArtifact{}
		if err := c.Get(context.Background(), client.ObjectKey{Name: "art", Namespace: "ns"}, got); err != nil {
			t.Fatalf("terminating artifact should still exist: %v", err)
		}
		if got.DeletionTimestamp.IsZero() {
			t.Error("artifact should still be terminating")
		}
	})

	t.Run("is a no-op for an existing non-terminating artifact", func(t *testing.T) {
		t.Parallel()

		existing := &solarv1alpha1.RenderArtifact{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "art",
				Namespace: "ns",
			},
			Spec: solarv1alpha1.RenderArtifactSpec{
				BaseURL:       "stale.example.com",
				Repository:    "ns/stale-repo",
				Tag:           "v0",
				RenderTaskRef: "stale-rt",
				RegistryRef:   &registryRef,
			},
		}
		c := fake.NewClientBuilder().
			WithScheme(newEnsureRenderArtifactScheme(t)).
			WithObjects(existing).
			Build()
		r := &TargetReconciler{Client: c}

		if err := r.ensureRenderArtifact(context.Background(), "art", rt, registryRef); err != nil {
			t.Fatalf("expected nil error for an existing artifact, got %v", err)
		}

		got := &solarv1alpha1.RenderArtifact{}
		if err := c.Get(context.Background(), client.ObjectKey{Name: "art", Namespace: "ns"}, got); err != nil {
			t.Fatalf("artifact should still exist: %v", err)
		}
		if got.Spec.BaseURL != existing.Spec.BaseURL {
			t.Errorf("artifact BaseURL should be unchanged, got %q", got.Spec.BaseURL)
		}
		if got.Spec.Repository != existing.Spec.Repository {
			t.Errorf("artifact Repository should be unchanged, got %q", got.Spec.Repository)
		}
		if got.Spec.Tag != existing.Spec.Tag {
			t.Errorf("artifact Tag should be unchanged, got %q", got.Spec.Tag)
		}
		if got.Spec.RenderTaskRef != existing.Spec.RenderTaskRef {
			t.Errorf("artifact RenderTaskRef should be unchanged, got %q", got.Spec.RenderTaskRef)
		}
		if !registryRefEqual(got.Spec.RegistryRef, &registryRef) {
			t.Errorf("artifact RegistryRef should be unchanged, got %v", got.Spec.RegistryRef)
		}
	})

	t.Run("creates the artifact when it does not exist", func(t *testing.T) {
		t.Parallel()

		c := fake.NewClientBuilder().
			WithScheme(newEnsureRenderArtifactScheme(t)).
			Build()
		r := &TargetReconciler{Client: c}

		if err := r.ensureRenderArtifact(context.Background(), "art", rt, registryRef); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		got := &solarv1alpha1.RenderArtifact{}
		if err := c.Get(context.Background(), client.ObjectKey{Name: "art", Namespace: "ns"}, got); err != nil {
			t.Fatalf("created artifact should exist: %v", err)
		}
		if got.Spec.BaseURL != rt.Spec.BaseURL || got.Spec.Repository != rt.Spec.Repository || got.Spec.Tag != rt.Spec.Tag {
			t.Errorf("artifact should carry the RenderTask coordinates, got %s/%s:%s", got.Spec.BaseURL, got.Spec.Repository, got.Spec.Tag)
		}
		if got.Spec.RenderTaskRef != rt.Name {
			t.Errorf("artifact should reference the RenderTask %q, got %q", rt.Name, got.Spec.RenderTaskRef)
		}
		if !registryRefEqual(got.Spec.RegistryRef, &registryRef) {
			t.Errorf("artifact should carry the RegistryRef %v, got %v", registryRef, got.Spec.RegistryRef)
		}
	})
}
