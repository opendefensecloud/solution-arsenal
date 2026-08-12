// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// The renderer reads the OCM component from the source registry to render a
// component's helm values template. These tests cover how the credentials for
// that read reach the render Job.

func newSourceSecret(name string, secretType corev1.SecretType, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Type: secretType,
		Data: data,
	}
}

func basicAuthData() map[string][]byte {
	return map[string][]byte{
		"username": []byte("admin"),
		"password": []byte("admin"),
	}
}

// sourceCredEnv returns the SOURCE_REGISTRY_* env vars on the rendered Job,
// keyed by name.
func sourceCredEnv(job *batchv1.Job) map[string]*corev1.EnvVarSource {
	found := map[string]*corev1.EnvVarSource{}

	for _, env := range job.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "SOURCE_REGISTRY_USERNAME" || env.Name == "SOURCE_REGISTRY_PASSWORD" {
			found[env.Name] = env.ValueFrom
		}
	}

	return found
}

func reconcileWithSourceSecret(t *testing.T, taskName string, secret *corev1.Secret) *batchv1.Job {
	t.Helper()

	task := newPullSecretsTestTask(taskName)

	objs := []client.Object{task}
	if secret != nil {
		task.Spec.SourceSecretRef = &corev1.LocalObjectReference{Name: secret.Name}
		objs = append(objs, secret)
	}

	r, c := newPullSecretsTestReconciler(nil, objs...)

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: task.Name, Namespace: task.Namespace},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	return getRenderedJob(t, c, task.Name)
}

// An Opaque Secret carrying username/password is the shape the e2e fixtures and
// the discovery worker already use for a Registry's solarSecretRef, so gating on
// the Secret type would silently drop the credentials.
func TestCreateRenderJob_SourceSecret_OpaqueWithKeys_ProjectsEnv(t *testing.T) {
	t.Parallel()

	job := reconcileWithSourceSecret(t, "srcopaque",
		newSourceSecret("src-opaque", corev1.SecretTypeOpaque, basicAuthData()))

	env := sourceCredEnv(job)
	if len(env) != 2 {
		t.Fatalf("SOURCE_REGISTRY_* env vars = %d, want 2 (%+v)", len(env), env)
	}

	user := env["SOURCE_REGISTRY_USERNAME"]
	if user == nil || user.SecretKeyRef == nil {
		t.Fatalf("SOURCE_REGISTRY_USERNAME has no SecretKeyRef: %+v", user)
	}
	if got := user.SecretKeyRef.Name; got != "src-opaque" {
		t.Errorf("username secret name = %q, want %q", got, "src-opaque")
	}
	if got := user.SecretKeyRef.Key; got != "username" {
		t.Errorf("username secret key = %q, want %q", got, "username")
	}
	if got := env["SOURCE_REGISTRY_PASSWORD"].SecretKeyRef.Key; got != "password" {
		t.Errorf("password secret key = %q, want %q", got, "password")
	}
}

func TestCreateRenderJob_SourceSecret_BasicAuthType_ProjectsEnv(t *testing.T) {
	t.Parallel()

	job := reconcileWithSourceSecret(t, "srcbasic",
		newSourceSecret("src-basic", corev1.SecretTypeBasicAuth, basicAuthData()))

	if got := len(sourceCredEnv(job)); got != 2 {
		t.Errorf("SOURCE_REGISTRY_* env vars = %d, want 2", got)
	}
}

// A Secret missing either key must be skipped
func TestCreateRenderJob_SourceSecret_MissingKeys_ProjectsNothing(t *testing.T) {
	t.Parallel()

	job := reconcileWithSourceSecret(t, "srcpartial",
		newSourceSecret("src-partial", corev1.SecretTypeOpaque, map[string][]byte{
			"username": []byte("admin"),
		}))

	if got := len(sourceCredEnv(job)); got != 0 {
		t.Errorf("SOURCE_REGISTRY_* env vars = %d, want 0", got)
	}
}

// Present-but-empty values authenticate no better than absent ones, and
// projecting them would mask the fact that no usable credential was supplied.
func TestCreateRenderJob_SourceSecret_EmptyValues_ProjectsNothing(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		data map[string][]byte
	}{
		{"empty username", map[string][]byte{"username": []byte(""), "password": []byte("admin")}},
		{"empty password", map[string][]byte{"username": []byte("admin"), "password": []byte("")}},
		{"both empty", map[string][]byte{"username": []byte(""), "password": []byte("")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job := reconcileWithSourceSecret(t, "srcempty"+strings.ReplaceAll(tc.name, " ", ""),
				newSourceSecret("src-empty-"+strings.ReplaceAll(tc.name, " ", "-"), corev1.SecretTypeOpaque, tc.data))

			if got := len(sourceCredEnv(job)); got != 0 {
				t.Errorf("SOURCE_REGISTRY_* env vars = %d, want 0", got)
			}
		})
	}
}

// A Registry's solarSecretRef may equally be a dockerconfigjson Secret, the same
// shape the push path accepts.
func TestCreateRenderJob_SourceSecret_DockerConfigJSON_MountsConfig(t *testing.T) {
	t.Parallel()

	job := reconcileWithSourceSecret(t, "srcdocker",
		newSourceSecret("src-docker", corev1.SecretTypeDockerConfigJson, map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"eA=="}}}`),
		}))

	var env *corev1.EnvVar
	for i := range job.Spec.Template.Spec.Containers[0].Env {
		if job.Spec.Template.Spec.Containers[0].Env[i].Name == "SOURCE_DOCKER_CONFIG" {
			env = &job.Spec.Template.Spec.Containers[0].Env[i]
		}
	}
	if env == nil {
		t.Fatal("SOURCE_DOCKER_CONFIG env var not set")
	}
	if env.Value != sourceDockerConfigPath {
		t.Errorf("SOURCE_DOCKER_CONFIG = %q, want %q", env.Value, sourceDockerConfigPath)
	}

	// The mount must not collide with the push secret's docker config
	var mount *corev1.VolumeMount
	for i := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
		if job.Spec.Template.Spec.Containers[0].VolumeMounts[i].Name == "source-dockerconfig" {
			mount = &job.Spec.Template.Spec.Containers[0].VolumeMounts[i]
		}
	}
	if mount == nil {
		t.Fatal("source-dockerconfig volume mount missing")
	}
	if mount.MountPath != sourceDockerConfigPath {
		t.Errorf("mount path = %q, want %q", mount.MountPath, sourceDockerConfigPath)
	}
	if mount.MountPath == "/etc/renderer/dockerconfig.json" {
		t.Error("source docker config collides with the push secret mount")
	}

	// Basic-auth env vars must not also appear for a docker config secret.
	if got := len(sourceCredEnv(job)); got != 0 {
		t.Errorf("SOURCE_REGISTRY_* env vars = %d, want 0", got)
	}
}

func TestCreateRenderJob_NoSourceSecretRef_ProjectsNothing(t *testing.T) {
	t.Parallel()

	job := reconcileWithSourceSecret(t, "srcnone", nil)

	if got := len(sourceCredEnv(job)); got != 0 {
		t.Errorf("SOURCE_REGISTRY_* env vars = %d, want 0", got)
	}
}

// Guards the API contract that keeps credential material out of etcd: only the
// Secret name may travel on the RenderTask spec.
func TestRenderTaskSpec_SourceSecretRef_CarriesNameOnly(t *testing.T) {
	t.Parallel()

	ref := &corev1.LocalObjectReference{Name: "src-creds"}
	spec := solarv1alpha1.RenderTaskSpec{SourceSecretRef: ref}

	if spec.SourceSecretRef.Name != "src-creds" {
		t.Errorf("SourceSecretRef.Name = %q, want %q", spec.SourceSecretRef.Name, "src-creds")
	}
}
