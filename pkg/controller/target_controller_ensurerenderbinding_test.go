// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func TestEnsureRenderBinding(t *testing.T) {
	t.Parallel()

	registryRef := solarv1alpha1.ObjectReference{Name: "reg"}
	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: "ns"},
	}

	newLiveArtifact := func() *solarv1alpha1.RenderArtifact {
		return &solarv1alpha1.RenderArtifact{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "art",
				Namespace: "ns",
			},
			Spec: solarv1alpha1.RenderArtifactSpec{
				BaseURL:       "registry.example.com",
				Repository:    "ns/repo",
				Tag:           "v1",
				RenderTaskRef: "rt",
				RegistryRef:   &registryRef,
			},
		}
	}

	newTerminatingArtifact := func() *solarv1alpha1.RenderArtifact {
		now := metav1.NewTime(time.Now())
		return &solarv1alpha1.RenderArtifact{
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
				RegistryRef:   &registryRef,
			},
		}
	}

	newClient := func(t *testing.T, objs ...client.Object) client.Client {
		t.Helper()

		return fake.NewClientBuilder().
			WithScheme(newEnsureRenderArtifactScheme(t)).
			WithObjects(objs...).
			Build()
	}

	getBinding := func(t *testing.T, c client.Client) (*solarv1alpha1.RenderBinding, error) {
		t.Helper()

		binding := &solarv1alpha1.RenderBinding{}
		err := c.Get(context.Background(), client.ObjectKey{Name: "bind", Namespace: "ns"}, binding)

		return binding, err
	}

	t.Run("creates the binding when the artifact does not exist", func(t *testing.T) {
		t.Parallel()

		c := newClient(t)
		r := &TargetReconciler{Client: c}

		if err := r.ensureRenderBinding(context.Background(), target, "art", "bind", registryRef); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		got, err := getBinding(t, c)
		if err != nil {
			t.Fatalf("created binding should exist: %v", err)
		}
		if got.Spec.RenderArtifactRef.Name != "art" {
			t.Errorf("binding should reference artifact %q, got %q", "art", got.Spec.RenderArtifactRef.Name)
		}
		if got.Spec.OwnerName != target.Name {
			t.Errorf("binding owner should be %q, got %q", target.Name, got.Spec.OwnerName)
		}
	})

	t.Run("creates the binding when the artifact exists and is not terminating", func(t *testing.T) {
		t.Parallel()

		c := newClient(t, newLiveArtifact())
		r := &TargetReconciler{Client: c}

		if err := r.ensureRenderBinding(context.Background(), target, "art", "bind", registryRef); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		if _, err := getBinding(t, c); err != nil {
			t.Fatalf("created binding should exist: %v", err)
		}
	})

	t.Run("does not create a binding when the artifact is terminating", func(t *testing.T) {
		t.Parallel()

		c := newClient(t, newTerminatingArtifact())
		r := &TargetReconciler{Client: c}

		err := r.ensureRenderBinding(context.Background(), target, "art", "bind", registryRef)
		if !errors.Is(err, errArtifactTerminating) {
			t.Fatalf("expected errArtifactTerminating, got %v", err)
		}

		_, err = getBinding(t, c)
		if !apierrors.IsNotFound(err) {
			t.Fatalf("no binding should have been created, got %v", err)
		}
	})

	t.Run("is a no-op for an existing binding even when the artifact is terminating", func(t *testing.T) {
		t.Parallel()

		existing := &solarv1alpha1.RenderBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bind",
				Namespace: "ns",
			},
			Spec: solarv1alpha1.RenderBindingSpec{
				RenderArtifactRef: corev1.LocalObjectReference{Name: "stale-art"},
				OwnerKind:         "StaleKind",
				OwnerName:         "stale-target",
				OwnerNamespace:    "stale-ns",
				RegistryRef:       &registryRef,
			},
		}
		c := newClient(t, newTerminatingArtifact(), existing)
		r := &TargetReconciler{Client: c}

		if err := r.ensureRenderBinding(context.Background(), target, "art", "bind", registryRef); err != nil {
			t.Fatalf("expected nil error for an existing binding, got %v", err)
		}

		got, err := getBinding(t, c)
		if err != nil {
			t.Fatalf("existing binding should still exist: %v", err)
		}
		if got.Spec.RenderArtifactRef.Name != existing.Spec.RenderArtifactRef.Name {
			t.Errorf("binding RenderArtifactRef should be unchanged, got %q", got.Spec.RenderArtifactRef.Name)
		}
		if got.Spec.OwnerKind != existing.Spec.OwnerKind {
			t.Errorf("binding OwnerKind should be unchanged, got %q", got.Spec.OwnerKind)
		}
		if got.Spec.OwnerName != existing.Spec.OwnerName {
			t.Errorf("binding OwnerName should be unchanged, got %q", got.Spec.OwnerName)
		}
		if got.Spec.OwnerNamespace != existing.Spec.OwnerNamespace {
			t.Errorf("binding OwnerNamespace should be unchanged, got %q", got.Spec.OwnerNamespace)
		}
		if !registryRefEqual(got.Spec.RegistryRef, &registryRef) {
			t.Errorf("binding RegistryRef should be unchanged, got %v", got.Spec.RegistryRef)
		}
	})
}
