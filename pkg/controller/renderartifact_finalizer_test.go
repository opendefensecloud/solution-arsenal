// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"errors"
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func TestRenderArtifactDeletion(t *testing.T) {
	t.Parallel()

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
			},
		}
	}

	newBinding := func() *solarv1alpha1.RenderBinding {
		return &solarv1alpha1.RenderBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bind",
				Namespace: "ns",
			},
			Spec: solarv1alpha1.RenderBindingSpec{
				RenderArtifactRef: corev1.LocalObjectReference{Name: "art"},
			},
		}
	}

	newClient := func(t *testing.T, objs ...client.Object) client.Client {
		t.Helper()

		return fake.NewClientBuilder().
			WithScheme(newEnsureRenderArtifactScheme(t)).
			WithObjects(objs...).
			WithIndex(&solarv1alpha1.RenderBinding{}, indexRenderBindingArtifactName, func(obj client.Object) []string {
				rb := obj.(*solarv1alpha1.RenderBinding)
				if rb.Spec.RenderArtifactRef.Name == "" {
					return nil
				}

				return []string{rb.Spec.RenderArtifactRef.Name}
			}).
			Build()
	}

	reconcile := func(t *testing.T, c client.Client, apiReader client.Reader, td *stubTagDeleter) (ctrl.Result, error) {
		t.Helper()

		recorder := events.NewFakeRecorder(10)
		go func() {
			for range recorder.Events {
			}
		}()

		r := &RenderArtifactReconciler{
			Client:    c,
			APIReader: apiReader,
			DeleteTag: td.DeleteTag,
			Recorder:  recorder,
		}

		return r.Reconcile(t.Context(), ctrl.Request{
			NamespacedName: types.NamespacedName{Name: "art", Namespace: "ns"},
		})
	}

	assertArtifactDeleted := func(t *testing.T, c client.Client) {
		t.Helper()

		err := c.Get(t.Context(), client.ObjectKey{Name: "art", Namespace: "ns"}, &solarv1alpha1.RenderArtifact{})
		if !apierrors.IsNotFound(err) {
			t.Fatalf("expected the artifact to be deleted after finalizer removal, got %v", err)
		}
	}

	assertArtifactStillTerminating := func(t *testing.T, c client.Client) {
		t.Helper()

		got := &solarv1alpha1.RenderArtifact{}
		if err := c.Get(t.Context(), client.ObjectKey{Name: "art", Namespace: "ns"}, got); err != nil {
			t.Fatalf("artifact should still exist: %v", err)
		}
		if !slices.Contains(got.Finalizers, renderArtifactFinalizer) {
			t.Error("expected the finalizer to still be present")
		}
		if got.DeletionTimestamp.IsZero() {
			t.Error("artifact should still be terminating")
		}
	}

	t.Run("keeps the OCI tag and retains the finalizer when a binding still references the artifact", func(t *testing.T) {
		t.Parallel()

		td := &stubTagDeleter{}
		c := newClient(t, newTerminatingArtifact(), newBinding())

		if _, err := reconcile(t, c, c, td); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if calls := td.calls(); len(calls) != 0 {
			t.Errorf("expected no DeleteTag call while the artifact is still bound, got %v", calls)
		}
		assertArtifactStillTerminating(t, c)
	})

	t.Run("deletes the OCI tag and removes the finalizer when no binding references the artifact", func(t *testing.T) {
		t.Parallel()

		td := &stubTagDeleter{}
		c := newClient(t, newTerminatingArtifact())

		if _, err := reconcile(t, c, c, td); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if calls := td.calls(); len(calls) != 1 {
			t.Errorf("expected exactly one DeleteTag call, got %v", calls)
		}
		assertArtifactDeleted(t, c)
	})

	t.Run("keeps the finalizer when OCI cleanup fails", func(t *testing.T) {
		t.Parallel()

		td := &stubTagDeleter{}
		td.failWith(errors.New("registry temporarily unavailable"))
		c := newClient(t, newTerminatingArtifact())

		if _, err := reconcile(t, c, c, td); err == nil {
			t.Fatal("expected an error from OCI cleanup")
		}
		assertArtifactStillTerminating(t, c)
	})

	t.Run("keeps the OCI tag when the cache misses a binding the API server has", func(t *testing.T) {
		t.Parallel()

		td := &stubTagDeleter{}
		// The cached client has not seen the binding yet; the APIReader has.
		cache := newClient(t, newTerminatingArtifact())
		apiReader := newClient(t, newTerminatingArtifact(), newBinding())

		if _, err := reconcile(t, cache, apiReader, td); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if calls := td.calls(); len(calls) != 0 {
			t.Errorf("expected no DeleteTag call when the API server still has a binding, got %v", calls)
		}
		assertArtifactStillTerminating(t, cache)
	})
}
