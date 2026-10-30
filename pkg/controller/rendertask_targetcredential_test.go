// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func targetCredentialEnv(t *testing.T, taskName string, ref *corev1.LocalObjectReference) []corev1.EnvVar {
	t.Helper()

	task := newPullSecretsTestTask(taskName)
	task.Spec.TargetCredentialSecretRef = ref

	r, c := newPullSecretsTestReconciler(nil, task)
	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		Name: task.Name, Namespace: task.Namespace,
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var found []corev1.EnvVar
	for _, e := range getRenderedJob(t, c, task.Name).Spec.Template.Spec.Containers[0].Env {
		if e.Name == "TARGET_CREDENTIAL_TOKEN" {
			found = append(found, e)
		}
	}

	return found
}

// The token must reach the Job only as a secretKeyRef, never as a value.
func TestCreateRenderJob_TargetCredential_ProjectsTokenKey(t *testing.T) {
	t.Parallel()

	got := targetCredentialEnv(t, "tgtcred", &corev1.LocalObjectReference{Name: "solar-agent-edge-01-1"})
	if len(got) != 1 {
		t.Fatalf("TARGET_CREDENTIAL_TOKEN env vars = %d, want 1", len(got))
	}
	if got[0].Value != "" {
		t.Errorf("token must not be set as a literal value, got %q", got[0].Value)
	}
	ref := got[0].ValueFrom
	if ref == nil || ref.SecretKeyRef == nil {
		t.Fatalf("no SecretKeyRef: %+v", got[0])
	}
	if ref.SecretKeyRef.Name != "solar-agent-edge-01-1" || ref.SecretKeyRef.Key != "token" {
		t.Errorf("SecretKeyRef = %s/%s, want solar-agent-edge-01-1/token", ref.SecretKeyRef.Name, ref.SecretKeyRef.Key)
	}
}

func TestCreateRenderJob_NoTargetCredential_ProjectsNothing(t *testing.T) {
	t.Parallel()

	if got := targetCredentialEnv(t, "tgtnocred", nil); len(got) != 0 {
		t.Errorf("TARGET_CREDENTIAL_TOKEN env vars = %d, want 0", len(got))
	}
}
