// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// credentialFixture is the RenderTask owner Target and a credential Secret
// with the given labels.
func credentialFixture(secretLabels map[string]string) []client.Object {
	return []client.Object{
		&solarv1alpha1.Target{Name: "owner", Namespace: "default", UID: "owner-uid"},
		&corev1.Secret{Name: "solar-agent-owner-1", Namespace: "default", Labels: secretLabels,
			Data: map[string][]byte{AgentCredentialTokenKey: []byte("tok")}},
	}
}

var ownerCredentialLabels = map[string]string{LabelAgentCredential: "owner", LabelAgentCredentialTargetUID: "owner-uid"}

func targetCredentialEnv(t *testing.T, taskName string, ref *corev1.LocalObjectReference) []corev1.EnvVar {
	t.Helper()

	task := newPullSecretsTestTask(taskName)
	task.Spec.TargetCredentialSecretRef = ref

	r, c := newPullSecretsTestReconciler(nil, append(credentialFixture(ownerCredentialLabels), task)...)
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

	got := targetCredentialEnv(t, "tgtcred", &corev1.LocalObjectReference{Name: "solar-agent-owner-1"})
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
	if ref.SecretKeyRef.Name != "solar-agent-owner-1" || ref.SecretKeyRef.Key != "token" {
		t.Errorf("SecretKeyRef = %s/%s, want solar-agent-owner-1/token", ref.SecretKeyRef.Name, ref.SecretKeyRef.Key)
	}
}

// A Secret that is not the owner Target's credential must never reach a Job.
func TestCreateRenderJob_TargetCredential_RejectsForeignSecret(t *testing.T) {
	t.Parallel()

	for name, labels := range map[string]map[string]string{
		"unlabelled":        nil,
		"other target":      {LabelAgentCredential: "other", LabelAgentCredentialTargetUID: "owner-uid"},
		"re-created target": {LabelAgentCredential: "owner", LabelAgentCredentialTargetUID: "old-uid"},
		"missing uid label": {LabelAgentCredential: "owner"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			task := newPullSecretsTestTask("tgtforeign")
			task.Spec.TargetCredentialSecretRef = &corev1.LocalObjectReference{Name: "solar-agent-owner-1"}
			r, c := newPullSecretsTestReconciler(nil, append(credentialFixture(labels), task)...)

			if _, err := r.Reconcile(context.Background(), reconcile.Request{Name: task.Name, Namespace: task.Namespace}); err == nil {
				t.Fatal("Reconcile must fail")
			}
			jobs := &batchv1.JobList{}
			if err := c.List(context.Background(), jobs); err != nil {
				t.Fatal(err)
			}
			if len(jobs.Items) != 0 {
				t.Fatalf("no Job may be created, got %d", len(jobs.Items))
			}
		})
	}
}

func TestCreateRenderJob_TargetCredential_RejectsNonTargetOwner(t *testing.T) {
	t.Parallel()

	task := newPullSecretsTestTask("tgtowner")
	task.Spec.OwnerKind = "Profile"
	task.Spec.TargetCredentialSecretRef = &corev1.LocalObjectReference{Name: "solar-agent-owner-1"}
	r, _ := newPullSecretsTestReconciler(nil, append(credentialFixture(ownerCredentialLabels), task)...)

	if _, err := r.Reconcile(context.Background(), reconcile.Request{Name: task.Name, Namespace: task.Namespace}); err == nil {
		t.Fatal("Reconcile must fail")
	}
}

func TestCreateRenderJob_NoTargetCredential_ProjectsNothing(t *testing.T) {
	t.Parallel()

	if got := targetCredentialEnv(t, "tgtnocred", nil); len(got) != 0 {
		t.Errorf("TARGET_CREDENTIAL_TOKEN env vars = %d, want 0", len(got))
	}
}
