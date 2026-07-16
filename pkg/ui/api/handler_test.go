// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

var profileGVR = schema.GroupVersionResource{
	Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "profiles",
}

func newTestHandler(objects ...runtime.Object) (*Handler, dynamic.Interface) {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{profileGVR: "ProfileList"}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objects...)
	h := &Handler{log: logr.Discard()}
	h.newClient = func(_ *http.Request) (dynamic.Interface, error) { return client, nil }

	return h, client
}

func profileObj(namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "solar.opendefense.cloud/v1alpha1",
		"kind":       "Profile",
		"metadata":   map[string]any{"namespace": namespace, "name": "p1"},
		"spec":       map[string]any{"releaseRef": map[string]any{"name": "rel-a"}},
	}}
}

func TestHandleCreate_OverridesNamespaceFromPath(t *testing.T) {
	h, client := newTestHandler()
	// Body claims namespace "evil"; the path says "default" — path must win.
	body, err := json.Marshal(profileObj("evil"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("namespace", "default")
	rec := httptest.NewRecorder()

	h.HandleCreate("profiles")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	got, err := client.Resource(profileGVR).Namespace("default").Get(req.Context(), "p1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected object in namespace default: %v", err)
	}
	if got.GetNamespace() != "default" {
		t.Fatalf("namespace = %q, want default", got.GetNamespace())
	}
}

func TestHandleCreate_RejectsNullBody(t *testing.T) {
	h, _ := newTestHandler()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader([]byte("null")))
	req.SetPathValue("namespace", "default")
	rec := httptest.NewRecorder()

	h.HandleCreate("profiles")(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400 for null body", rec.Code)
	}
}

func TestHandlePatch_MergesSpec(t *testing.T) {
	h, client := newTestHandler(profileObj("default"))
	patch := []byte(`{"spec":{"userdata":{"k":"v"}}}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/", bytes.NewReader(patch))
	req.SetPathValue("namespace", "default")
	req.SetPathValue("name", "p1")
	rec := httptest.NewRecorder()

	h.HandlePatch("profiles")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	got, _ := client.Resource(profileGVR).Namespace("default").Get(req.Context(), "p1", metav1.GetOptions{})
	spec, _, _ := unstructured.NestedMap(got.Object, "spec")
	if _, ok := spec["userdata"]; !ok {
		t.Fatalf("patch did not merge userdata; spec=%v", spec)
	}
	if _, ok := spec["releaseRef"]; !ok {
		t.Fatalf("patch clobbered releaseRef; spec=%v", spec)
	}
}

func TestHandlePatch_JSONPatchReplacesWholesale(t *testing.T) {
	// Seed userdata with two keys; a merge patch could never drop "b".
	obj := profileObj("default")
	_ = unstructured.SetNestedMap(obj.Object, map[string]any{"a": "1", "b": "2"}, "spec", "userdata")
	h, client := newTestHandler(obj)

	// JSON Patch "add" replaces the whole userdata object → "b" is gone.
	patch := []byte(`[{"op":"add","path":"/spec/userdata","value":{"a":"1"}}]`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/", bytes.NewReader(patch))
	req.Header.Set("Content-Type", "application/json-patch+json")
	req.SetPathValue("namespace", "default")
	req.SetPathValue("name", "p1")
	rec := httptest.NewRecorder()

	h.HandlePatch("profiles")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	got, _ := client.Resource(profileGVR).Namespace("default").Get(req.Context(), "p1", metav1.GetOptions{})
	userdata, _, _ := unstructured.NestedMap(got.Object, "spec", "userdata")
	if _, ok := userdata["b"]; ok {
		t.Fatalf("json patch did not replace userdata wholesale; still has b: %v", userdata)
	}
	if userdata["a"] != "1" {
		t.Fatalf("json patch lost a; userdata=%v", userdata)
	}
}

func TestHandleDelete_RemovesObject(t *testing.T) {
	h, client := newTestHandler(profileObj("default"))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("name", "p1")
	rec := httptest.NewRecorder()

	h.HandleDelete("profiles")(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want 204", rec.Code)
	}
	_, err := client.Resource(profileGVR).Namespace("default").Get(req.Context(), "p1", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected object to be deleted")
	}
}
