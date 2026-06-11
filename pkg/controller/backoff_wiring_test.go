// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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

// These tests cover the wiring of requeueAfterForCondition into TargetReconciler
// without bringing up envtest. They use controller-runtime's fake client so the
// suite stays runnable in environments without the kubebuilder etcd binary.

func newWiringTestTarget() *solarv1alpha1.Target {
	return &solarv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wiring-test",
			Namespace: "default",
			// Pre-set the finalizer so Reconcile proceeds past the finalizer
			// step and reaches the dependency-wait paths we want to test.
			Finalizers: []string{targetFinalizer},
		},
		Spec: solarv1alpha1.TargetSpec{
			RenderRegistryRef: corev1.LocalObjectReference{Name: "missing-registry"},
			Userdata:          runtime.RawExtension{Raw: []byte(`{}`)},
		},
	}
}

func newWiringTestReconciler(objs ...client.Object) (*TargetReconciler, client.Client) {
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = solarv1alpha1.AddToScheme(sch)

	c := fake.NewClientBuilder().
		WithScheme(sch).
		WithObjects(objs...).
		WithStatusSubresource(&solarv1alpha1.Target{}).
		Build()

	return &TargetReconciler{
		Client:   c,
		Scheme:   sch,
		Recorder: events.NewFakeRecorder(64),
	}, c
}

func TestTargetReconcile_MissingRegistry_FreshWait(t *testing.T) {
	t.Parallel()
	target := newWiringTestTarget()
	r, _ := newWiringTestReconciler(target)

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: target.Name, Namespace: target.Namespace},
	})
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	if result.RequeueAfter < initialLow || result.RequeueAfter > initialHigh {
		t.Errorf("RequeueAfter = %v, want in [%v, %v] (fresh wait bucket)",
			result.RequeueAfter, initialLow, initialHigh)
	}
}

func TestTargetReconcile_MissingRegistry_AgedWaitSaturates(t *testing.T) {
	t.Parallel()
	target := newWiringTestTarget()
	// Backdate the RegistryResolved condition to simulate a Target that has
	// been waiting on the missing Registry for an hour. setCondition will
	// see Status=False unchanged and preserve LastTransitionTime, so the
	// helper computes the backoff against this aged timestamp.
	// Truncate to second precision so the fake-client roundtrip doesn't break
	// the equality assertion at the end of the test (metav1.Time serialises at
	// second precision).
	hourAgo := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))
	target.Status.Conditions = []metav1.Condition{
		{
			Type:               ConditionTypeRegistryResolved,
			Status:             metav1.ConditionFalse,
			Reason:             "NotFound",
			Message:            "Registry not found: missing-registry",
			LastTransitionTime: hourAgo,
			ObservedGeneration: target.Generation,
		},
	}

	r, c := newWiringTestReconciler(target)

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: target.Name, Namespace: target.Namespace},
	})
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	if result.RequeueAfter < maxLow || result.RequeueAfter > maxHigh {
		t.Errorf("RequeueAfter = %v, want in [%v, %v] (saturated bucket)",
			result.RequeueAfter, maxLow, maxHigh)
	}

	// Verify Status was preserved (LastTransitionTime unchanged): proves that
	// the wait clock survives across reconciles, which is the whole point.
	got := &solarv1alpha1.Target{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(target), got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	cond := apimeta.FindStatusCondition(got.Status.Conditions, ConditionTypeRegistryResolved)
	if cond == nil {
		t.Fatal("RegistryResolved condition missing after reconcile")
	}
	if !cond.LastTransitionTime.Equal(&hourAgo) {
		t.Errorf("LastTransitionTime = %v, want preserved at %v", cond.LastTransitionTime, hourAgo)
	}
}
