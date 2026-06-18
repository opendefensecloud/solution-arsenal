// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// These tests cover the wiring of RendererImagePullSecrets into the Job
// PodSpec rendered by RenderTaskReconciler. They use the fake client to stay
// independent of envtest (which needs the kubebuilder etcd binary).

func newPullSecretsTestTask(name string) *solarv1alpha1.RenderTask {
	return &solarv1alpha1.RenderTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: solarv1alpha1.RenderTaskSpec{
			Repository:     "example.com/charts/test",
			Tag:            "v1",
			BaseURL:        "oci://registry.example.com",
			OwnerName:      "owner",
			OwnerNamespace: "default",
			OwnerKind:      "Target",
		},
	}
}

func newPullSecretsTestReconciler(pullSecrets []string, objs ...client.Object) (*RenderTaskReconciler, client.Client) {
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = solarv1alpha1.AddToScheme(sch)

	c := fake.NewClientBuilder().
		WithScheme(sch).
		WithObjects(objs...).
		WithStatusSubresource(&solarv1alpha1.RenderTask{}).
		Build()

	return &RenderTaskReconciler{
		Client:                   c,
		Scheme:                   sch,
		Recorder:                 events.NewFakeRecorder(64),
		RendererImage:            "ghcr.io/opendefensecloud/solar-renderer:test",
		RendererCommand:          "/solar-renderer",
		RendererImagePullSecrets: pullSecrets,
	}, c
}

func getRenderedJob(t *testing.T, c client.Client, taskName string) *batchv1.Job {
	t.Helper()
	jobList := &batchv1.JobList{}
	if err := c.List(context.Background(), jobList); err != nil {
		t.Fatalf("List jobs: %v", err)
	}
	for i := range jobList.Items {
		if jobList.Items[i].Name == "render-"+taskName {
			return &jobList.Items[i]
		}
	}
	t.Fatalf("no job named render-%s found in fake client (found %d job(s))", taskName, len(jobList.Items))

	return nil
}

func TestCreateRenderJob_NoPullSecrets_LeavesPodSpecEmpty(t *testing.T) {
	t.Parallel()
	task := newPullSecretsTestTask("noseclet")
	r, c := newPullSecretsTestReconciler(nil, task)

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: task.Name, Namespace: task.Namespace},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	job := getRenderedJob(t, c, task.Name)
	if got := job.Spec.Template.Spec.ImagePullSecrets; len(got) != 0 {
		t.Errorf("ImagePullSecrets = %+v, want empty", got)
	}
}

func TestCreateRenderJob_PullSecrets_AppearOnPodSpec(t *testing.T) {
	t.Parallel()
	task := newPullSecretsTestTask("withseclet")
	r, c := newPullSecretsTestReconciler([]string{"pull-a", "pull-b"}, task)

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: task.Name, Namespace: task.Namespace},
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	job := getRenderedJob(t, c, task.Name)
	want := []corev1.LocalObjectReference{
		{Name: "pull-a"},
		{Name: "pull-b"},
	}
	got := job.Spec.Template.Spec.ImagePullSecrets
	if len(got) != len(want) {
		t.Fatalf("ImagePullSecrets length = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ImagePullSecrets[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
