// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func TestReconcile_CreatesSecretForTargetWithPublicKey(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := solarv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	pubKey := "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...\n-----END PUBLIC KEY-----"

	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "test-ns",
		},
	}
	target.Status.PublicKey = pubKey

	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&solarv1alpha1.Target{}).WithObjects(target).Build()
	r := &Reconciler{
		Client: client,
		Scheme: scheme,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-target",
			Namespace: "test-ns",
		},
	}

	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("unexpected requeue: %+v", result)
	}

	secret := &corev1.Secret{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "test-target-cosign-pub", Namespace: "test-ns"}, secret); err != nil {
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

	scheme := runtime.NewScheme()
	if err := solarv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "test-ns",
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(target).Build()
	r := &Reconciler{
		Client: client,
		Scheme: scheme,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-target",
			Namespace: "test-ns",
		},
	}

	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("unexpected requeue: %+v", result)
	}

	secretList := &corev1.SecretList{}
	if err := client.List(context.Background(), secretList); err != nil {
		t.Fatal(err)
	}
	if len(secretList.Items) != 0 {
		t.Fatalf("expected no secrets, got %d", len(secretList.Items))
	}
}

func TestReconcile_UpdatesSecretOnKeyRotation(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := solarv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	oldKey := "-----BEGIN PUBLIC KEY-----\nOLDKEY\n-----END PUBLIC KEY-----"
	newKey := "-----BEGIN PUBLIC KEY-----\nNEWKEY\n-----END PUBLIC KEY-----"

	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "test-ns",
		},
	}
	target.Status.PublicKey = newKey
	target.Status.PreviousPublicKey = oldKey

	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&solarv1alpha1.Target{}).WithObjects(target).Build()
	r := &Reconciler{
		Client: client,
		Scheme: scheme,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-target",
			Namespace: "test-ns",
		},
	}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secret := &corev1.Secret{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "test-target-cosign-pub", Namespace: "test-ns"}, secret); err != nil {
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

	scheme := runtime.NewScheme()
	if err := solarv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

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
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target-cosign-pub",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"cosign.pub": []byte(pubKey),
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&solarv1alpha1.Target{}).WithObjects(target, existingSecret).Build()
	r := &Reconciler{
		Client: client,
		Scheme: scheme,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-target",
			Namespace: "test-ns",
		},
	}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secretList := &corev1.SecretList{}
	if err := client.List(context.Background(), secretList); err != nil {
		t.Fatal(err)
	}
	if len(secretList.Items) != 0 {
		t.Fatalf("expected secrets to be deleted, got %d", len(secretList.Items))
	}

	updatedTarget := &solarv1alpha1.Target{}
	err := client.Get(context.Background(), types.NamespacedName{Name: "test-target", Namespace: "test-ns"}, updatedTarget)
	if err == nil {
		if contains(updatedTarget.Finalizers, agentFinalizer) {
			t.Fatalf("expected finalizer to be removed, got %v", updatedTarget.Finalizers)
		}
	}
}

func TestReconcile_HandlesNonExistentTarget(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := solarv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &Reconciler{
		Client: client,
		Scheme: scheme,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent",
			Namespace: "test-ns",
		},
	}

	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("unexpected requeue: %+v", result)
	}
}

func TestReconcile_NoOpWhenSecretAlreadyMatches(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := solarv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	pubKey := "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...\n-----END PUBLIC KEY-----"

	target := &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "test-ns",
		},
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

	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&solarv1alpha1.Target{}).WithObjects(target, existingSecret).Build()
	r := &Reconciler{
		Client: client,
		Scheme: scheme,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-target",
			Namespace: "test-ns",
		},
	}

	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("unexpected requeue: %+v", result)
	}

	secret := &corev1.Secret{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "test-target-cosign-pub", Namespace: "test-ns"}, secret); err != nil {
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

var _ = ctrl.Request{}
