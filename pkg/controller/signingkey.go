// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	signingKeyField      = "cosign.key"
	signingPasswordField = "cosign.password"
)

// validateSigningKey checks the shape of the cosign key in the Secret, not its
// password. Decrypting runs scrypt, which needs more memory than the controller's
// limit, and the renderer checks the password before it pushes anything.
// Errors never include key material.
func validateSigningKey(s *corev1.Secret) error {
	raw := s.Data[signingKeyField]
	if len(raw) == 0 {
		return fmt.Errorf("secret %s has no %s", s.Name, signingKeyField)
	}
	block, _ := pem.Decode(raw)
	// The PEM types are cosign's CosignPrivateKeyPemType and SigstorePrivateKeyPemType,
	// spelled out here to keep cosign out of the controller binary.
	if block == nil || (block.Type != "ENCRYPTED SIGSTORE PRIVATE KEY" && block.Type != "ENCRYPTED COSIGN PRIVATE KEY") {
		return fmt.Errorf("%s in secret %s is not an encrypted cosign private key", signingKeyField, s.Name)
	}

	return nil
}

// signingKeyTag identifies key and password by the first 8 hex chars of the sha256
// over both. A wrong password is only found by the renderer, so fixing it has to
// give a new tag and task name, or the failed task would never be retried.
func signingKeyTag(s *corev1.Secret) string {
	h := sha256.New()
	h.Write(s.Data[signingKeyField])
	h.Write([]byte{0})
	h.Write(s.Data[signingPasswordField])

	return hex.EncodeToString(h.Sum(nil))[:8]
}

// resolveSigningKey returns the Target's validated signing key Secret and keeps
// ConditionTypeSigningKeyResolved in sync. nil, true, nil means unsigned. With
// ready=false the condition is set and the caller stops: the Secret watch
// re-enqueues the Target once the key changes.
func (r *TargetReconciler) resolveSigningKey(ctx context.Context, target *solarv1alpha1.Target) (key *corev1.Secret, ready bool, err error) {
	if target.Spec.Signing == nil {
		if apimeta.RemoveStatusCondition(&target.Status.Conditions, ConditionTypeSigningKeyResolved) {
			if err := r.Status().Update(ctx, target); err != nil {
				return nil, false, fmt.Errorf("failed to remove Target status condition %s: %w", ConditionTypeSigningKeyResolved, err)
			}
		}

		return nil, true, nil
	}

	name := target.Spec.Signing.KeySecretRef.Name
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: target.Namespace, Name: name}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, fmt.Errorf("failed to get signing key Secret %s: %w", name, err)
		}

		return nil, false, r.setCondition(ctx, target, ConditionTypeSigningKeyResolved, metav1.ConditionFalse, ReasonNotFound,
			"Signing key Secret not found: "+name)
	}

	if err := validateSigningKey(secret); err != nil {
		return nil, false, r.setCondition(ctx, target, ConditionTypeSigningKeyResolved, metav1.ConditionFalse, ReasonInvalidSigningKey,
			err.Error())
	}

	if err := r.setCondition(ctx, target, ConditionTypeSigningKeyResolved, metav1.ConditionTrue, ReasonResolved,
		"Signing key resolved: "+name); err != nil {
		return nil, false, err
	}

	return secret, true, nil
}

// mapSigningKeySecretToTargets enqueues the Targets in the Secret's namespace
// that use it as their signing key.
func (r *TargetReconciler) mapSigningKeySecretToTargets(ctx context.Context, obj client.Object) []reconcile.Request {
	targets := &solarv1alpha1.TargetList{}
	if err := r.List(ctx, targets, client.InNamespace(obj.GetNamespace())); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to list Targets for signing key Secret", "secret", obj.GetName())

		return nil
	}

	var requests []reconcile.Request
	for i := range targets.Items {
		t := &targets.Items[i]
		if t.Spec.Signing != nil && t.Spec.Signing.KeySecretRef.Name == obj.GetName() {
			requests = append(requests, reconcile.Request{Namespace: t.Namespace, Name: t.Name})
		}
	}

	return requests
}
