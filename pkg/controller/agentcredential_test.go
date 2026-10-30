// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func TestCredentialIndex(t *testing.T) {
	cases := map[string]struct {
		name string
		want int
		ok   bool
	}{
		"first":                {"solar-agent-edge-01-1", 1, true},
		"tenth":                {"solar-agent-edge-01-10", 10, true},
		"other target":         {"solar-agent-edge-01-x-1", 0, false}, // target "edge-01-x" is someone else
		"no suffix":            {"solar-agent-edge-01", 0, false},
		"zero is not an index": {"solar-agent-edge-01-0", 0, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := credentialIndex("edge-01", &corev1.Secret{Name: c.name})
			if got != c.want || ok != c.ok {
				t.Fatalf("credentialIndex(%q) = %d, %v; want %d, %v", c.name, got, ok, c.want, c.ok)
			}
		})
	}
}

func TestAgentRulesAreScopedToOneTarget(t *testing.T) {
	rules := agentRules("edge-01")
	if len(rules) != 2 {
		t.Fatalf("want 2 rules, got %d", len(rules))
	}
	for _, r := range rules {
		if len(r.ResourceNames) != 1 || r.ResourceNames[0] != "edge-01" {
			t.Fatalf("rule not scoped to edge-01: %+v", r)
		}
		for _, v := range r.Verbs {
			if v == "create" || v == "list" || v == "watch" || v == "*" {
				t.Fatalf("rule grants %q: %+v", v, r)
			}
		}
	}
}

func TestReconcileDisabledWithoutAPIServerURL(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := solarv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	target := &solarv1alpha1.Target{Name: "edge-01", Namespace: "default"}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(target).WithStatusSubresource(target).Build()
	r := &AgentCredentialReconciler{Client: c, Scheme: scheme, APIReader: c}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(target)}); err != nil {
		t.Fatal(err)
	}

	got := &solarv1alpha1.Target{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(target), got); err != nil {
		t.Fatal(err)
	}
	cond := apimeta.FindStatusCondition(got.Status.Conditions, ConditionTypeAgentCredentialReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != ReasonAgentCredentialDisabled {
		t.Fatalf("want AgentCredentialReady=False/Disabled, got %+v", cond)
	}
	err := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "solar-agent-edge-01"}, &corev1.ServiceAccount{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("ServiceAccount must not exist, got err %v", err)
	}
}

func secretWith(created time.Time, expires string, rotation string) *corev1.Secret {
	return &corev1.Secret{
		Name:              "solar-agent-t-1",
		UID:               "uid-1",
		CreationTimestamp: metav1.NewTime(created),
		Annotations:       map[string]string{AnnotationExpiresAt: expires, AnnotationRotationRequest: rotation},
	}
}

func TestRotationDueAtEightyPercent(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := secretWith(created, created.Add(100*time.Hour).Format(time.RFC3339), "")

	if due, at := rotationDue(s, created.Add(79*time.Hour)); due || !at.Equal(created.Add(80*time.Hour)) {
		t.Fatalf("at 79h: due=%v at=%v", due, at)
	}
	if due, _ := rotationDue(s, created.Add(80*time.Hour)); !due {
		t.Fatal("at 80h: want due")
	}
}

func TestRotationDueWithUnreadableExpiry(t *testing.T) {
	// A Secret we cannot date is treated as due: re-minting is safe, an expired token is not.
	if due, _ := rotationDue(secretWith(time.Now(), "garbage", ""), time.Now()); !due {
		t.Fatal("want due")
	}
}

func TestRotationRequested(t *testing.T) {
	target := &solarv1alpha1.Target{Annotations: map[string]string{AnnotationRotateAgentCredential: "2"}}
	if rotationRequested(target, secretWith(time.Now(), "", "2")) {
		t.Fatal("same value answered already")
	}
	if !rotationRequested(target, secretWith(time.Now(), "", "1")) {
		t.Fatal("new value must rotate")
	}
	if rotationRequested(&solarv1alpha1.Target{}, secretWith(time.Now(), "", "")) {
		t.Fatal("no annotation, no rotation")
	}
}

func TestCredentialTagUsesUID(t *testing.T) {
	a := &corev1.Secret{Name: "solar-agent-t-1", UID: "uid-a"}
	b := &corev1.Secret{Name: "solar-agent-t-1", UID: "uid-b"} // same name, re-created after a revoke
	if credentialTag(a) == credentialTag(b) {
		t.Fatal("a reused Secret name must give a new tag")
	}
	if len(credentialTag(a)) != 8 {
		t.Fatalf("want 8 hex chars, got %q", credentialTag(a))
	}
}

func TestReportShowsCredential(t *testing.T) {
	s := &corev1.Secret{UID: "uid-a"}
	report := &solarv1alpha1.TargetReport{Spec: solarv1alpha1.TargetReportSpec{Releases: []solarv1alpha1.ReleaseReport{
		{Name: "other", Revision: "v0.0.3-abcdef12"},
		{Name: "agent", Revision: "v0.0.1-0011aabb-" + credentialTag(s)},
	}}}
	if !reportShowsCredential(report, s) {
		t.Fatal("want true")
	}
	report.Spec.Releases[1].Revision = "v0.0.1-0011aabb-00000000"
	if reportShowsCredential(report, s) {
		t.Fatal("want false")
	}
}

func TestCurrentAgentCredentialIgnoresSecretWithoutToken(t *testing.T) {
	target := &solarv1alpha1.Target{Name: "edge", Namespace: "ns"}
	lbl := map[string]string{LabelAgentCredential: "edge"}
	c := fake.NewClientBuilder().WithObjects(
		&corev1.Secret{Name: "solar-agent-edge-1", Namespace: "ns", Labels: lbl, Data: map[string][]byte{AgentCredentialTokenKey: []byte("t1")}},
		&corev1.Secret{Name: "solar-agent-edge-2", Namespace: "ns", Labels: lbl}, // minting in progress
	).Build()
	cur, err := currentAgentCredential(context.Background(), c, target)
	if err != nil || cur == nil || cur.Name != "solar-agent-edge-1" {
		t.Fatalf("got %v, %v; want solar-agent-edge-1", cur, err)
	}
}

func revokedFixture(t *testing.T) (client.Client, *solarv1alpha1.Target) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := solarv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	target := &solarv1alpha1.Target{Name: "edge-01", Namespace: "default",
		Annotations: map[string]string{AnnotationAgentCredential: AgentCredentialRevoked}}
	secret := &corev1.Secret{
		Name: "solar-agent-edge-01-1", Namespace: "default",
		Labels: map[string]string{LabelAgentCredential: "edge-01"},
		Data:   map[string][]byte{AgentCredentialTokenKey: []byte("tok")},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(target, secret).WithStatusSubresource(target).Build()

	return c, target
}

func TestRevokedTargetIsRevokedEvenWhenDisabled(t *testing.T) {
	c, target := revokedFixture(t)
	r := &AgentCredentialReconciler{Client: c, Scheme: c.Scheme(), APIReader: c}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(target)}); err != nil {
		t.Fatal(err)
	}

	err := c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "solar-agent-edge-01-1"}, &corev1.Secret{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("credential Secret must be deleted, got err %v", err)
	}
	got := &solarv1alpha1.Target{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(target), got); err != nil {
		t.Fatal(err)
	}
	cond := apimeta.FindStatusCondition(got.Status.Conditions, ConditionTypeAgentCredentialReady)
	if cond == nil || cond.Reason != ReasonAgentCredentialRevoked {
		t.Fatalf("want reason Revoked, got %+v", cond)
	}
}

func TestCurrentAgentCredentialNilWhenRevoked(t *testing.T) {
	c, target := revokedFixture(t)

	got, err := currentAgentCredential(context.Background(), c, target)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("want nil for revoked Target, got %s", got.Name)
	}
}
