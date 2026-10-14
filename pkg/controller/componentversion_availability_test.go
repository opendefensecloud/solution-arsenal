// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func TestAvailabilityFromCheck(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	grace := 10 * time.Minute
	at := func(d time.Duration) metav1.Time { return metav1.NewTime(now.Add(-d)) }
	cond := func(s metav1.ConditionStatus, reason string, since time.Duration) *metav1.Condition {
		c := availableCondition(s, reason, "")
		c.LastTransitionTime = at(since)

		return &c
	}
	readErr := errors.New("connection reset")

	tests := []struct {
		name       string
		cur        *metav1.Condition
		found      bool
		err        error
		wantStatus metav1.ConditionStatus
		wantReason string
		wantSince  *metav1.Time // nil means we don't care
	}{
		{"found, no condition", nil, true, nil, metav1.ConditionTrue, solarv1alpha1.ReasonArtifactAvailable, nil},
		{"found turns red to green", cond(metav1.ConditionFalse, solarv1alpha1.ReasonArtifactMissing, time.Hour), true, nil, metav1.ConditionTrue, solarv1alpha1.ReasonArtifactAvailable, nil},
		{"read error on green", cond(metav1.ConditionTrue, solarv1alpha1.ReasonArtifactAvailable, time.Hour), false, readErr, metav1.ConditionUnknown, solarv1alpha1.ReasonRegistryUnreachable, nil},
		{"read error on red never stays red", cond(metav1.ConditionFalse, solarv1alpha1.ReasonArtifactMissing, time.Hour), false, readErr, metav1.ConditionUnknown, solarv1alpha1.ReasonRegistryUnreachable, nil},
		{"read error while pending resets pending", cond(metav1.ConditionUnknown, solarv1alpha1.ReasonArtifactMissingPending, 9*time.Minute), false, readErr, metav1.ConditionUnknown, solarv1alpha1.ReasonRegistryUnreachable, nil},
		{"first miss, no condition", nil, false, nil, metav1.ConditionUnknown, solarv1alpha1.ReasonArtifactMissingPending, new(at(0))},
		{"first miss on green", cond(metav1.ConditionTrue, solarv1alpha1.ReasonArtifactAvailable, time.Hour), false, nil, metav1.ConditionUnknown, solarv1alpha1.ReasonArtifactMissingPending, new(at(0))},
		{"miss after read error starts the timer now", cond(metav1.ConditionUnknown, solarv1alpha1.ReasonRegistryUnreachable, time.Hour), false, nil, metav1.ConditionUnknown, solarv1alpha1.ReasonArtifactMissingPending, new(at(0))},
		{"miss within grace stays pending", cond(metav1.ConditionUnknown, solarv1alpha1.ReasonArtifactMissingPending, 5*time.Minute), false, nil, metav1.ConditionUnknown, solarv1alpha1.ReasonArtifactMissingPending, new(at(5 * time.Minute))},
		{"miss exactly at grace turns red", cond(metav1.ConditionUnknown, solarv1alpha1.ReasonArtifactMissingPending, grace), false, nil, metav1.ConditionFalse, solarv1alpha1.ReasonArtifactMissing, nil},
		{"miss on red stays red", cond(metav1.ConditionFalse, solarv1alpha1.ReasonArtifactMissing, time.Hour), false, nil, metav1.ConditionFalse, solarv1alpha1.ReasonArtifactMissing, new(at(time.Hour))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := availabilityFromCheck(tc.cur, tc.found, tc.err, now, grace)
			if got.Type != solarv1alpha1.ConditionTypeAvailable || got.Status != tc.wantStatus || got.Reason != tc.wantReason {
				t.Fatalf("got %s/%s/%s, want Available/%s/%s", got.Type, got.Status, got.Reason, tc.wantStatus, tc.wantReason)
			}
			if tc.wantSince != nil && !got.LastTransitionTime.Equal(tc.wantSince) {
				t.Fatalf("lastTransitionTime = %v, want %v", got.LastTransitionTime, tc.wantSince)
			}
		})
	}
}

func TestSetAvailableCondition(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	old := metav1.NewTime(now.Add(-time.Hour))

	t.Run("entering pending from another Unknown reason restarts lastTransitionTime", func(t *testing.T) {
		conds := []metav1.Condition{{Type: solarv1alpha1.ConditionTypeAvailable, Status: metav1.ConditionUnknown, Reason: solarv1alpha1.ReasonRegistryUnreachable, LastTransitionTime: old}}
		next := availabilityFromCheck(&conds[0], false, nil, now, time.Minute)
		if !setAvailableCondition(&conds, next, now) {
			t.Fatal("expected a change")
		}
		got := apimeta.FindStatusCondition(conds, solarv1alpha1.ConditionTypeAvailable)
		if got.Reason != solarv1alpha1.ReasonArtifactMissingPending || !got.LastTransitionTime.Equal(new(metav1.NewTime(now))) {
			t.Fatalf("got %s since %v, want pending since %v", got.Reason, got.LastTransitionTime, now)
		}
	})

	t.Run("an unchanged condition reports no change", func(t *testing.T) {
		c := availableCondition(metav1.ConditionTrue, solarv1alpha1.ReasonArtifactAvailable, "manifest resolves")
		c.LastTransitionTime = old
		conds := []metav1.Condition{c}
		if setAvailableCondition(&conds, availableCondition(metav1.ConditionTrue, solarv1alpha1.ReasonArtifactAvailable, "manifest resolves"), now) {
			t.Fatal("expected no change")
		}
		if !conds[0].LastTransitionTime.Equal(&old) {
			t.Fatal("lastTransitionTime must be kept")
		}
	})
}

func TestRegistryAuth(t *testing.T) {
	ctx := context.Background()
	reg := func(secret string) *solarv1alpha1.Registry {
		r := &solarv1alpha1.Registry{Namespace: "ns", Spec: solarv1alpha1.RegistrySpec{Hostname: "oci://reg.example.com:5000/"}}
		if secret != "" {
			r.Spec.SolarSecretRef = &corev1.LocalObjectReference{Name: secret}
		}

		return r
	}
	opaque := &corev1.Secret{Namespace: "ns", Name: "opaque", Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"username": []byte("u"), "password": []byte("p")}}
	dockercfg := &corev1.Secret{Namespace: "ns", Name: "dockercfg", Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{"reg.example.com:5000":{"username":"du","password":"dp"}}}`)}}
	c := fake.NewClientBuilder().WithObjects(opaque, dockercfg).Build()

	userOf := func(t *testing.T, a authn.Authenticator) string {
		t.Helper()
		cfg, err := a.Authorization()
		if err != nil {
			t.Fatal(err)
		}

		return cfg.Username
	}

	t.Run("no secret ref is anonymous", func(t *testing.T) {
		a, err := registryAuth(ctx, c, reg(""))
		if err != nil || a != authn.Anonymous {
			t.Fatalf("got %v, %v", a, err)
		}
	})
	t.Run("opaque username/password (discovery's shape)", func(t *testing.T) {
		a, err := registryAuth(ctx, c, reg("opaque"))
		if err != nil || userOf(t, a) != "u" {
			t.Fatalf("got %v, %v", a, err)
		}
	})
	t.Run("dockerconfigjson keyed by normalized hostname", func(t *testing.T) {
		a, err := registryAuth(ctx, c, reg("dockercfg"))
		if err != nil || userOf(t, a) != "du" {
			t.Fatalf("got %v, %v", a, err)
		}
	})
	t.Run("missing secret is an error", func(t *testing.T) {
		if _, err := registryAuth(ctx, c, reg("absent")); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestComponentVersionUpdatePredicate(t *testing.T) {
	base := func() *solarv1alpha1.ComponentVersion {
		return &solarv1alpha1.ComponentVersion{Name: "cv", Namespace: "ns", ResourceVersion: "1", Generation: 1,
			Spec: solarv1alpha1.ComponentVersionSpec{ComponentName: "a", Tag: "v1"}}
	}
	p := componentVersionUpdatePredicate()
	update := func(mutate func(cv *solarv1alpha1.ComponentVersion)) bool {
		old, cur := base(), base()
		cur.ResourceVersion = "2"
		mutate(cur)

		return p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: cur})
	}

	if update(func(cv *solarv1alpha1.ComponentVersion) {
		cv.Status.Conditions = []metav1.Condition{availableCondition(metav1.ConditionTrue, solarv1alpha1.ReasonArtifactAvailable, "")}
	}) {
		t.Error("a status-only update must not trigger a reconcile")
	}
	if !update(func(cv *solarv1alpha1.ComponentVersion) {
		cv.Annotations = map[string]string{solarv1alpha1.AvailabilityRecheckAnnotation: "now"}
	}) {
		t.Error("an annotation change must trigger a reconcile")
	}
	if !update(func(cv *solarv1alpha1.ComponentVersion) { cv.Finalizers = []string{"x"} }) {
		t.Error("a finalizer change must trigger a reconcile")
	}
	if !update(func(cv *solarv1alpha1.ComponentVersion) { cv.DeletionTimestamp = new(metav1.Now()) }) {
		t.Error("a deletion must trigger a reconcile")
	}
	if !update(func(cv *solarv1alpha1.ComponentVersion) { cv.Spec.Digest = "sha256:other"; cv.Generation = 2 }) {
		t.Error("a digest change (re-push) must trigger a reconcile")
	}
}

func TestRegistryForMatchesNormalizedHostname(t *testing.T) {
	sch := scheme.Scheme
	if err := solarv1alpha1.AddToScheme(sch); err != nil {
		t.Fatal(err)
	}
	reg := &solarv1alpha1.Registry{Name: "r", Namespace: "ns", Spec: solarv1alpha1.RegistrySpec{Hostname: "oci://reg.example.com:5000/"}}
	other := &solarv1alpha1.Registry{Name: "o", Namespace: "other", Spec: solarv1alpha1.RegistrySpec{Hostname: "reg.example.com:5000"}}
	r := &ComponentVersionReconciler{Client: fake.NewClientBuilder().WithScheme(sch).WithObjects(reg, other).Build()}

	got, found, err := r.registryFor(context.Background(), &solarv1alpha1.ComponentVersion{Namespace: "ns", Spec: solarv1alpha1.ComponentVersionSpec{Registry: "reg.example.com:5000"}})
	if err != nil || !found || got.Name != "r" {
		t.Fatalf("got %v found=%v err=%v, want registry r", got, found, err)
	}

	_, found, err = r.registryFor(context.Background(), &solarv1alpha1.ComponentVersion{Namespace: "ns", Spec: solarv1alpha1.ComponentVersionSpec{Registry: "elsewhere:5000"}})
	if err != nil || found {
		t.Fatalf("found=%v err=%v, want not found", found, err)
	}
}

func TestRenderableCondition(t *testing.T) {
	cvWith := func(c *metav1.Condition) *solarv1alpha1.ComponentVersion {
		cv := &solarv1alpha1.ComponentVersion{Name: "cv"}
		if c != nil {
			cv.Status.Conditions = []metav1.Condition{*c}
		}

		return cv
	}
	avail := func(s metav1.ConditionStatus) *metav1.Condition {
		return new(availableCondition(s, "x", "msg"))
	}

	for _, tc := range []struct {
		name       string
		cv         *solarv1alpha1.ComponentVersion
		wantStatus metav1.ConditionStatus
		wantReason string
	}{
		{"available", cvWith(avail(metav1.ConditionTrue)), metav1.ConditionTrue, "ComponentVersionAvailable"},
		{"unavailable", cvWith(avail(metav1.ConditionFalse)), metav1.ConditionFalse, "ComponentVersionUnavailable"},
		{"unknown", cvWith(avail(metav1.ConditionUnknown)), metav1.ConditionUnknown, "ComponentVersionAvailabilityUnknown"},
		{"not checked yet", cvWith(nil), metav1.ConditionUnknown, "ComponentVersionAvailabilityUnknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := renderableCondition(tc.cv)
			if got.Type != ConditionTypeRenderable || got.Status != tc.wantStatus || got.Reason != tc.wantReason {
				t.Fatalf("got %s/%s/%s", got.Type, got.Status, got.Reason)
			}
		})
	}

	if c := unresolvedRenderableCondition(); c.Status != metav1.ConditionUnknown || c.Reason != "ComponentVersionAvailabilityUnknown" {
		t.Fatalf("unresolved: got %s/%s", c.Status, c.Reason)
	}
}

// A registry that accepts connections but never answers must not hold a reconcile
// worker for long, and the timeout must surface as RegistryUnreachable, not missing.
func TestCheckAvailabilityBoundsAHangingRegistry(t *testing.T) {
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer hang.Close()
	defer close(release)
	host := strings.TrimPrefix(hang.URL, "http://")

	sch := scheme.Scheme
	if err := solarv1alpha1.AddToScheme(sch); err != nil {
		t.Fatal(err)
	}
	reg := &solarv1alpha1.Registry{Name: "hang", Namespace: "ns", Spec: solarv1alpha1.RegistrySpec{Hostname: host, PlainHTTP: true}}
	r := &ComponentVersionReconciler{Client: fake.NewClientBuilder().WithScheme(sch).WithObjects(reg).Build(), AvailabilityGrace: time.Minute}
	cv := &solarv1alpha1.ComponentVersion{Namespace: "ns", Spec: solarv1alpha1.ComponentVersionSpec{
		ComponentName: "example.com/a", Repository: "test/example.com/a", Registry: host, Digest: "sha256:" + strings.Repeat("e", 64)}}

	start := time.Now()
	got, _, err := r.checkAvailability(context.Background(), cv)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("check took %v, want it bounded well below the requeue interval", elapsed)
	}
	if got.Status != metav1.ConditionUnknown || got.Reason != solarv1alpha1.ReasonRegistryUnreachable {
		t.Fatalf("got %s/%s, want Unknown/RegistryUnreachable", got.Status, got.Reason)
	}
}

func TestComponentVersionReconcilerValidate(t *testing.T) {
	valid := func() *ComponentVersionReconciler {
		return &ComponentVersionReconciler{AvailabilityInterval: 5 * time.Minute, AvailabilityGrace: 15 * time.Minute,
			AvailabilityQPS: 10, AvailabilityBurst: 20, MaxConcurrentReconciles: 4}
	}
	if err := valid().validate(); err != nil {
		t.Fatalf("defaults must be valid: %v", err)
	}
	if r := valid(); func() error { r.AvailabilityGrace = 0; return r.validate() }() != nil {
		t.Fatal("a zero grace period is allowed")
	}

	for name, mutate := range map[string]func(r *ComponentVersionReconciler){
		"zero interval":             func(r *ComponentVersionReconciler) { r.AvailabilityInterval = 0 },
		"negative interval":         func(r *ComponentVersionReconciler) { r.AvailabilityInterval = -time.Second },
		"negative grace":            func(r *ComponentVersionReconciler) { r.AvailabilityGrace = -time.Second },
		"zero qps":                  func(r *ComponentVersionReconciler) { r.AvailabilityQPS = 0 },
		"zero burst":                func(r *ComponentVersionReconciler) { r.AvailabilityBurst = 0 },
		"zero workers":              func(r *ComponentVersionReconciler) { r.MaxConcurrentReconciles = 0 },
		"no check fits in interval": func(r *ComponentVersionReconciler) { r.AvailabilityInterval = time.Second; r.AvailabilityQPS = 0.5 },
	} {
		t.Run(name, func(t *testing.T) {
			r := valid()
			mutate(r)
			if err := r.validate(); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

// A throttled check must not contact the registry and must say how long to wait,
// so the reconciler can requeue instead of holding a worker.
func TestCheckAvailabilityThrottledSkipsRegistry(t *testing.T) {
	var requests atomic.Int64
	reg := ggcrregistry.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		reg.ServeHTTP(w, r)
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	img, err := random.Image(64, 1)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := name.ParseReference(host+"/test/component-descriptors/example.com/a:v1", name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatal(err)
	}
	digest, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}

	sch := scheme.Scheme
	if err := solarv1alpha1.AddToScheme(sch); err != nil {
		t.Fatal(err)
	}
	registry := &solarv1alpha1.Registry{Name: "r", Namespace: "ns", Spec: solarv1alpha1.RegistrySpec{Hostname: host, PlainHTTP: true}}
	r := &ComponentVersionReconciler{
		Client:            fake.NewClientBuilder().WithScheme(sch).WithObjects(registry).Build(),
		AvailabilityGrace: time.Minute,
		limiter:           rate.NewLimiter(rate.Every(time.Hour), 1),
	}
	cv := &solarv1alpha1.ComponentVersion{Namespace: "ns", Spec: solarv1alpha1.ComponentVersionSpec{
		ComponentName: "example.com/a", Repository: "test/example.com/a", Registry: host, Digest: digest.String()}}

	requests.Store(0)
	got, delay, err := r.checkAvailability(context.Background(), cv)
	if err != nil || delay != 0 || got.Status != metav1.ConditionTrue || requests.Load() == 0 {
		t.Fatalf("first check: got %s/%s delay=%v requests=%d err=%v", got.Status, got.Reason, delay, requests.Load(), err)
	}

	requests.Store(0)
	_, delay, err = r.checkAvailability(context.Background(), cv)
	if err != nil {
		t.Fatal(err)
	}
	if delay <= 0 {
		t.Fatalf("second check: want a positive delay, got %v", delay)
	}
	if n := requests.Load(); n != 0 {
		t.Fatalf("a throttled check must not contact the registry, got %d requests", n)
	}
}
