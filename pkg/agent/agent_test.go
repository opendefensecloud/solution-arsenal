// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

type countingClient struct {
	client.Client
	secretUpdateCount int
}

func (c *countingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*corev1.Secret); ok {
		c.secretUpdateCount++
	}
	return c.Client.Update(ctx, obj, opts...)
}

type errorClient struct {
	client.Client
	getErr error
}

func (c *errorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.getErr
}

func scheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := solarv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestReconcile_CreatesSecretForTargetWithPublicKey(t *testing.T) {
	t.Parallel()
	s := scheme(t)

	pubKey := "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...\n-----END PUBLIC KEY-----"

	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{Name: "test-target", Namespace: "test-ns"},
	}
	target.Status.PublicKey = pubKey

	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&solarv1alpha1.Target{}).WithObjects(target).Build()
	r := &Reconciler{Client: cl, Scheme: s}

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-target", Namespace: "test-ns"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("unexpected requeue: %+v", result)
	}

	secret := &corev1.Secret{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "test-target-cosign-pub", Namespace: "test-ns"}, secret); err != nil {
		t.Fatalf("expected Secret to be created: %v", err)
	}
	if string(secret.Data["cosign.pub"]) != pubKey {
		t.Fatalf("expected cosign.pub to be %q, got %q", pubKey, string(secret.Data["cosign.pub"]))
	}
	if secret.Labels["solar.opendefense.cloud/component"] != "cosign-pubkey" {
		t.Fatalf("expected component label, got %v", secret.Labels)
	}
	if secret.Labels["solar.opendefense.cloud/target"] != "test-target" {
		t.Fatalf("expected target label, got %v", secret.Labels)
	}
}

func TestReconcile_SkipsTargetWithoutPublicKey(t *testing.T) {
	t.Parallel()
	s := scheme(t)

	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{Name: "test-target", Namespace: "test-ns"},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&solarv1alpha1.Target{}).WithObjects(target).Build()
	cc := &countingClient{Client: cl}
	r := &Reconciler{Client: cc, Scheme: s}

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-target", Namespace: "test-ns"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("unexpected requeue: %+v", result)
	}
	if cc.secretUpdateCount != 0 {
		t.Fatalf("expected no Secret updates, got %d", cc.secretUpdateCount)
	}

	secretList := &corev1.SecretList{}
	if err := cl.List(context.Background(), secretList); err != nil {
		t.Fatal(err)
	}
	if len(secretList.Items) != 0 {
		t.Fatalf("expected no secrets, got %d", len(secretList.Items))
	}
}

func TestReconcile_RemovesFinalizerOnDeletingTargetWithoutPublicKey(t *testing.T) {
	t.Parallel()
	s := scheme(t)

	now := metav1.Now()
	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-target",
			Namespace:         "test-ns",
			DeletionTimestamp: &now,
			Finalizers:        []string{agentFinalizer},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&solarv1alpha1.Target{}).WithObjects(target).Build()
	r := &Reconciler{Client: cl, Scheme: s}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-target", Namespace: "test-ns"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &solarv1alpha1.Target{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: "test-target", Namespace: "test-ns"}, updated)
	if err == nil {
		if contains(updated.Finalizers, agentFinalizer) {
			t.Fatalf("expected finalizer to be removed, got %v", updated.Finalizers)
		}
	}
}

func TestReconcile_UpdatesSecretOnKeyRotation(t *testing.T) {
	t.Parallel()
	s := scheme(t)

	oldKey := "-----BEGIN PUBLIC KEY-----\nOLDKEY\n-----END PUBLIC KEY-----"
	newKey := "-----BEGIN PUBLIC KEY-----\nNEWKEY\n-----END PUBLIC KEY-----"

	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{Name: "test-target", Namespace: "test-ns"},
	}
	target.Status.PublicKey = newKey
	target.Status.PreviousPublicKey = oldKey

	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target-cosign-pub",
			Namespace: "test-ns",
			Labels: map[string]string{
				"solar.opendefense.cloud/component": "cosign-pubkey",
				"solar.opendefense.cloud/target":    "test-target",
			},
		},
		Data: map[string][]byte{
			"cosign.pub": []byte(oldKey),
		},
		Type: corev1.SecretTypeOpaque,
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&solarv1alpha1.Target{}).WithObjects(target, existingSecret).Build()
	r := &Reconciler{Client: cl, Scheme: s}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-target", Namespace: "test-ns"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secret := &corev1.Secret{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "test-target-cosign-pub", Namespace: "test-ns"}, secret); err != nil {
		t.Fatalf("expected Secret to exist: %v", err)
	}
	if string(secret.Data["cosign.pub"]) != newKey {
		t.Fatalf("expected cosign.pub to be new key, got %q", string(secret.Data["cosign.pub"]))
	}
	if string(secret.Data["cosign.pub.old"]) != oldKey {
		t.Fatalf("expected cosign.pub.old to be old key, got %q", string(secret.Data["cosign.pub.old"]))
	}
}

func TestReconcile_DeletesSecretOnTargetDeletion(t *testing.T) {
	t.Parallel()
	s := scheme(t)

	pubKey := "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...\n-----END PUBLIC KEY-----"

	now := metav1.Now()
	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-target",
			Namespace:         "test-ns",
			DeletionTimestamp: &now,
			Finalizers:        []string{agentFinalizer},
		},
	}
	target.Status.PublicKey = pubKey

	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-target-cosign-pub", Namespace: "test-ns"},
		Data:       map[string][]byte{"cosign.pub": []byte(pubKey)},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&solarv1alpha1.Target{}).WithObjects(target, existingSecret).Build()
	r := &Reconciler{Client: cl, Scheme: s}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-target", Namespace: "test-ns"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secretList := &corev1.SecretList{}
	if err := cl.List(context.Background(), secretList); err != nil {
		t.Fatal(err)
	}
	if len(secretList.Items) != 0 {
		t.Fatalf("expected secrets to be deleted, got %d", len(secretList.Items))
	}

	updatedTarget := &solarv1alpha1.Target{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: "test-target", Namespace: "test-ns"}, updatedTarget)
	if err == nil {
		if contains(updatedTarget.Finalizers, agentFinalizer) {
			t.Fatalf("expected finalizer to be removed, got %v", updatedTarget.Finalizers)
		}
	}
}

func TestReconcile_HandlesNonExistentTarget(t *testing.T) {
	t.Parallel()
	s := scheme(t)

	cl := fake.NewClientBuilder().WithScheme(s).Build()
	r := &Reconciler{Client: cl, Scheme: s}

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: "test-ns"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("unexpected requeue: %+v", result)
	}
}

func TestReconcile_PropagatesGetError(t *testing.T) {
	t.Parallel()
	s := scheme(t)

	fakeErr := errors.New("transient API error")
	cl := fake.NewClientBuilder().WithScheme(s).Build()
	ec := &errorClient{Client: cl, getErr: fakeErr}
	r := &Reconciler{Client: ec, Scheme: s}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-target", Namespace: "test-ns"},
	})
	if err == nil {
		t.Fatal("expected error to be propagated, got nil")
	}
	if !errors.Is(err, fakeErr) {
		t.Fatalf("expected error to wrap %v, got %v", fakeErr, err)
	}
}

func TestReconcile_NoOpWhenSecretAlreadyMatches(t *testing.T) {
	t.Parallel()
	s := scheme(t)

	pubKey := "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...\n-----END PUBLIC KEY-----"

	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{Name: "test-target", Namespace: "test-ns"},
	}
	target.Status.PublicKey = pubKey

	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target-cosign-pub",
			Namespace: "test-ns",
			Labels: map[string]string{
				"solar.opendefense.cloud/component": "cosign-pubkey",
				"solar.opendefense.cloud/target":    "test-target",
			},
		},
		Data: map[string][]byte{
			"cosign.pub": []byte(pubKey),
		},
		Type: corev1.SecretTypeOpaque,
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&solarv1alpha1.Target{}).WithObjects(target, existingSecret).Build()
	cc := &countingClient{Client: cl}
	r := &Reconciler{Client: cc, Scheme: s}

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-target", Namespace: "test-ns"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("unexpected requeue: %+v", result)
	}
	if cc.secretUpdateCount != 0 {
		t.Fatalf("expected zero Secret updates, got %d", cc.secretUpdateCount)
	}

	secret := &corev1.Secret{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "test-target-cosign-pub", Namespace: "test-ns"}, secret); err != nil {
		t.Fatalf("expected Secret to exist: %v", err)
	}
	if string(secret.Data["cosign.pub"]) != pubKey {
		t.Fatalf("expected cosign.pub to be unchanged, got %q", string(secret.Data["cosign.pub"]))
	}
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
