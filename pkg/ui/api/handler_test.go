// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	authzv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"go.opendefense.cloud/solar/pkg/ui/auth"
	"go.opendefense.cloud/solar/pkg/ui/session"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	profileGVR   = schema.GroupVersionResource{Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "profiles"}
	targetGVR    = schema.GroupVersionResource{Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "targets"}
	namespaceGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
)

// newTestHandler wires only the dynamic-client seam — enough for the write
// handlers, which never touch the session store or clientsets.
func newTestHandler(objects ...runtime.Object) (*Handler, dynamic.Interface) {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{profileGVR: "ProfileList"}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objects...)
	h := &Handler{log: logr.Discard()}
	h.newClient = func(_ *http.Request) (dynamic.Interface, error) { return client, nil }

	return h, client
}

// testHandler wires every client seam to a fake for the handlers that also need
// a session store, typed clientset, or discovery client.
type testHandler struct {
	h         *Handler
	dyn       *dynamicfake.FakeDynamicClient
	clientset *k8sfake.Clientset
	discovery dynamic.Interface
	store     *session.Store
}

func newFullHandler(objects ...runtime.Object) *testHandler {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		profileGVR:   "ProfileList",
		targetGVR:    "TargetList",
		namespaceGVR: "NamespaceList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objects...)
	cs := k8sfake.NewSimpleClientset()
	store, _ := session.NewStore("")

	th := &testHandler{dyn: dyn, clientset: cs, store: store}
	th.h = &Handler{
		log:          logr.Discard(),
		sessionStore: store,
		newClient:    func(_ *http.Request) (dynamic.Interface, error) { return dyn, nil },
		newClientset: func(_ *session.Data) (kubernetes.Interface, error) { return cs, nil },
		newDiscovery: func() (dynamic.Interface, error) { return th.discovery, nil },
	}

	return th
}

// login stores a session and returns a cookie that resolves to it.
func (th *testHandler) login(data *session.Data) *http.Cookie {
	rec := httptest.NewRecorder()
	th.store.Set(rec, data)

	return rec.Result().Cookies()[0]
}

func profileObj(namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "solar.opendefense.cloud/v1alpha1",
		"kind":       "Profile",
		"metadata":   map[string]any{"namespace": namespace, "name": "p1"},
		"spec":       map[string]any{"releaseRef": map[string]any{"name": "rel-a"}},
	}}
}

func namespaceObj(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]any{"name": name},
	}}
}

func allowSAR(cs *k8sfake.Clientset, allowed bool) {
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authzv1.SelfSubjectAccessReview{
			Status: authzv1.SubjectAccessReviewStatus{Allowed: allowed},
		}, nil
	})
}

var _ = Describe("API Handler", func() {
	Describe("HandleCreate", func() {
		It("takes the namespace from the path, ignoring the body", func(ctx SpecContext) {
			h, client := newTestHandler()
			// Body claims namespace "evil"; the path says "default" — path must win.
			body, err := json.Marshal(profileObj("evil"))
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", strings.NewReader(string(body)))
			req.SetPathValue("namespace", "default")
			rec := httptest.NewRecorder()

			h.HandleCreate("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK), rec.Body.String())
			got, err := client.Resource(profileGVR).Namespace("default").Get(ctx, "p1", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.GetNamespace()).To(Equal("default"))
		})

		It("rejects a null body with 400", func(ctx SpecContext) {
			h, _ := newTestHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", strings.NewReader("null"))
			req.SetPathValue("namespace", "default")
			rec := httptest.NewRecorder()

			h.HandleCreate("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects an invalid body with 400", func(ctx SpecContext) {
			h, _ := newTestHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", strings.NewReader("{not json"))
			req.SetPathValue("namespace", "default")
			rec := httptest.NewRecorder()

			h.HandleCreate("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 404 for an unknown resource", func(ctx SpecContext) {
			h, _ := newTestHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", nil)
			rec := httptest.NewRecorder()

			h.HandleCreate("bogus")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 500 when the client cannot be built", func(ctx SpecContext) {
			h, _ := newTestHandler()
			h.newClient = func(*http.Request) (dynamic.Interface, error) {
				return nil, apierrors.NewInternalError(context.DeadlineExceeded)
			}
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", strings.NewReader(`{"kind":"Profile"}`))
			req.SetPathValue("namespace", "default")
			rec := httptest.NewRecorder()

			h.HandleCreate("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})

		It("propagates a k8s error status", func(ctx SpecContext) {
			th := newFullHandler(profileObj("default"))
			th.dyn.PrependReactor("create", "profiles", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "profiles"}, "p1", errors.New("exists"))
			})
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/",
				strings.NewReader(`{"apiVersion":"solar.opendefense.cloud/v1alpha1","kind":"Profile","metadata":{"name":"p1"}}`))
			req.SetPathValue("namespace", "default")
			rec := httptest.NewRecorder()

			th.h.HandleCreate("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusConflict))
		})
	})

	Describe("HandlePatch", func() {
		It("merge-patches only the given fields", func(ctx SpecContext) {
			h, client := newTestHandler(profileObj("default"))
			req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/", strings.NewReader(`{"spec":{"userdata":{"k":"v"}}}`))
			req.SetPathValue("namespace", "default")
			req.SetPathValue("name", "p1")
			rec := httptest.NewRecorder()

			h.HandlePatch("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK), rec.Body.String())
			got, _ := client.Resource(profileGVR).Namespace("default").Get(ctx, "p1", metav1.GetOptions{})
			spec, _, _ := unstructured.NestedMap(got.Object, "spec")
			Expect(spec).To(HaveKey("userdata"))
			Expect(spec).To(HaveKey("releaseRef")) // untouched by the merge patch
		})

		It("replaces a field wholesale with a JSON Patch", func(ctx SpecContext) {
			obj := profileObj("default")
			// Seed userdata with two keys; a merge patch could never drop "b".
			_ = unstructured.SetNestedMap(obj.Object, map[string]any{"a": "1", "b": "2"}, "spec", "userdata")
			h, client := newTestHandler(obj)

			req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/",
				strings.NewReader(`[{"op":"add","path":"/spec/userdata","value":{"a":"1"}}]`))
			req.Header.Set("Content-Type", "application/json-patch+json")
			req.SetPathValue("namespace", "default")
			req.SetPathValue("name", "p1")
			rec := httptest.NewRecorder()

			h.HandlePatch("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK), rec.Body.String())
			got, _ := client.Resource(profileGVR).Namespace("default").Get(ctx, "p1", metav1.GetOptions{})
			userdata, _, _ := unstructured.NestedMap(got.Object, "spec", "userdata")
			Expect(userdata).NotTo(HaveKey("b"))
			Expect(userdata["a"]).To(Equal("1"))
		})

		It("returns 404 for an unknown resource", func(ctx SpecContext) {
			h, _ := newTestHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/", nil)
			rec := httptest.NewRecorder()

			h.HandlePatch("bogus")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("propagates a k8s error status", func(ctx SpecContext) {
			th := newFullHandler()
			th.dyn.PrependReactor("patch", "profiles", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "profiles"}, "p1")
			})
			req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/", strings.NewReader(`{"spec":{}}`))
			req.SetPathValue("namespace", "default")
			req.SetPathValue("name", "p1")
			rec := httptest.NewRecorder()

			th.h.HandlePatch("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("HandleDelete", func() {
		It("removes the object", func(ctx SpecContext) {
			h, client := newTestHandler(profileObj("default"))
			req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/", nil)
			req.SetPathValue("namespace", "default")
			req.SetPathValue("name", "p1")
			rec := httptest.NewRecorder()

			h.HandleDelete("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNoContent))
			_, err := client.Resource(profileGVR).Namespace("default").Get(ctx, "p1", metav1.GetOptions{})
			Expect(err).To(HaveOccurred()) // gone
		})

		It("returns 404 for an unknown resource", func(ctx SpecContext) {
			h, _ := newTestHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/", nil)
			rec := httptest.NewRecorder()

			h.HandleDelete("bogus")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 500 when the client cannot be built", func(ctx SpecContext) {
			h, _ := newTestHandler()
			h.newClient = func(*http.Request) (dynamic.Interface, error) {
				return nil, apierrors.NewInternalError(context.DeadlineExceeded)
			}
			req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/", nil)
			req.SetPathValue("namespace", "default")
			req.SetPathValue("name", "p1")
			rec := httptest.NewRecorder()

			h.HandleDelete("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("HandleList", func() {
		It("lists resources in the namespace", func(ctx SpecContext) {
			th := newFullHandler(profileObj("default"))
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.SetPathValue("namespace", "default")
			rec := httptest.NewRecorder()

			th.h.HandleList("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK), rec.Body.String())
			var out map[string]any
			Expect(json.Unmarshal(rec.Body.Bytes(), &out)).To(Succeed())
			Expect(out).To(HaveKey("items"))
		})

		It("returns 404 for an unknown resource", func(ctx SpecContext) {
			th := newFullHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			th.h.HandleList("bogus")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 500 when the client cannot be built", func(ctx SpecContext) {
			th := newFullHandler()
			th.h.newClient = func(*http.Request) (dynamic.Interface, error) {
				return nil, apierrors.NewInternalError(context.DeadlineExceeded)
			}
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			th.h.HandleList("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("HandleGet", func() {
		It("gets a single resource", func(ctx SpecContext) {
			th := newFullHandler(profileObj("default"))
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.SetPathValue("namespace", "default")
			req.SetPathValue("name", "p1")
			rec := httptest.NewRecorder()

			th.h.HandleGet("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK), rec.Body.String())
		})

		It("propagates a k8s NotFound as 404", func(ctx SpecContext) {
			th := newFullHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.SetPathValue("namespace", "default")
			req.SetPathValue("name", "missing")
			rec := httptest.NewRecorder()

			th.h.HandleGet("profiles")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 404 for an unknown resource", func(ctx SpecContext) {
			th := newFullHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			th.h.HandleGet("bogus")(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("HandleMe", func() {
		It("reports unauthenticated when there is no session", func(ctx SpecContext) {
			th := newFullHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			th.h.HandleMe()(rec, req)

			var out map[string]any
			Expect(json.Unmarshal(rec.Body.Bytes(), &out)).To(Succeed())
			Expect(out["authenticated"]).To(BeFalse())
		})

		It("reports the user and admin capabilities", func(ctx SpecContext) {
			th := newFullHandler()
			allowSAR(th.clientset, true)
			cookie := th.login(&session.Data{Username: "alice", Groups: []string{"devs"}})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()

			th.h.HandleMe()(rec, req)

			var out map[string]any
			Expect(json.Unmarshal(rec.Body.Bytes(), &out)).To(Succeed())
			Expect(out["authenticated"]).To(BeTrue())
			Expect(out["username"]).To(Equal("alice"))
			Expect(out["canImpersonate"]).To(BeTrue())
			Expect(out["canListAllNamespaces"]).To(BeTrue())
		})

		It("includes the impersonation block when previewing as another user", func(ctx SpecContext) {
			th := newFullHandler()
			allowSAR(th.clientset, false)
			cookie := th.login(&session.Data{Username: "admin", ImpersonatingAs: "bob", ImpersonatingGroups: []string{"team"}})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()

			th.h.HandleMe()(rec, req)

			var out map[string]any
			Expect(json.Unmarshal(rec.Body.Bytes(), &out)).To(Succeed())
			imp, ok := out["impersonating"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(imp["username"]).To(Equal("bob"))
		})
	})

	Describe("access reviews", func() {
		It("CanImpersonate uses the cached value without a SAR round-trip", func(ctx SpecContext) {
			th := newFullHandler()
			allowSAR(th.clientset, false) // would say false if consulted
			cached := true
			cookie := th.login(&session.Data{Username: "alice", CanImpersonate: &cached})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)

			got, err := th.h.CanImpersonate(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeTrue())
		})

		It("CanListAllNamespaces returns false with no session", func(ctx SpecContext) {
			th := newFullHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

			got, err := th.h.CanListAllNamespaces(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeFalse())
		})

		It("CanListAllNamespaces uses the cached value", func(ctx SpecContext) {
			th := newFullHandler()
			allowSAR(th.clientset, false)
			cached := true
			cookie := th.login(&session.Data{CanListAllNamespaces: &cached})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)

			got, err := th.h.CanListAllNamespaces(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeTrue())
		})

		It("CanListAllNamespaces consults the SAR when uncached", func(ctx SpecContext) {
			th := newFullHandler()
			allowSAR(th.clientset, true)
			cookie := th.login(&session.Data{Username: "alice"})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)

			got, err := th.h.CanListAllNamespaces(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeTrue())
		})
	})

	Describe("HandleListNamespaces", func() {
		It("returns only namespaces the user has solar access to", func(ctx SpecContext) {
			th := newFullHandler()
			th.discovery = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
				runtime.NewScheme(),
				map[schema.GroupVersionResource]string{namespaceGVR: "NamespaceList"},
				namespaceObj("ns-a"), namespaceObj("ns-b"),
			)
			// ns-a grants a solar rule; ns-b grants nothing.
			th.clientset.PrependReactor("create", "selfsubjectrulesreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
				ssrr := action.(k8stesting.CreateAction).GetObject().(*authzv1.SelfSubjectRulesReview)
				var rules []authzv1.ResourceRule
				if ssrr.Spec.Namespace == "ns-a" {
					rules = []authzv1.ResourceRule{{APIGroups: []string{solarAPIGroup}, Resources: []string{"*"}, Verbs: []string{"get"}}}
				}

				return true, &authzv1.SelfSubjectRulesReview{Status: authzv1.SubjectRulesReviewStatus{ResourceRules: rules}}, nil
			})

			cookie := th.login(&session.Data{Username: "alice"})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()

			th.h.HandleListNamespaces()(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK), rec.Body.String())
			var out struct {
				Items []struct {
					Metadata struct {
						Name string `json:"name"`
					} `json:"metadata"`
				} `json:"items"`
			}
			Expect(json.Unmarshal(rec.Body.Bytes(), &out)).To(Succeed())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Items[0].Metadata.Name).To(Equal("ns-a"))
		})

		It("returns 401 without a session", func(ctx SpecContext) {
			th := newFullHandler()
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			th.h.HandleListNamespaces()(rec, req)

			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
		})
	})

	Describe("hasSolarAccess", func() {
		It("grants access for a wildcard API group", func(ctx SpecContext) {
			cs := k8sfake.NewSimpleClientset()
			cs.PrependReactor("create", "selfsubjectrulesreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, &authzv1.SelfSubjectRulesReview{Status: authzv1.SubjectRulesReviewStatus{
					ResourceRules: []authzv1.ResourceRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}},
				}}, nil
			})
			Expect(hasSolarAccess(ctx, cs, "ns", logr.Discard())).To(BeTrue())
		})

		It("excludes the namespace when the SSRR fails", func(ctx SpecContext) {
			cs := k8sfake.NewSimpleClientset()
			cs.PrependReactor("create", "selfsubjectrulesreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("boom")
			})
			Expect(hasSolarAccess(ctx, cs, "ns", logr.Discard())).To(BeFalse())
		})
	})

	Describe("clientFor", func() {
		var h *Handler

		BeforeEach(func() {
			store, _ := session.NewStore("")
			h = &Handler{
				log:          logr.Discard(),
				sessionStore: store,
				authProvider: auth.NewNoopProvider(),
				baseConfig:   &rest.Config{Host: "http://localhost:8080"},
			}
		})

		It("errors when there is no session", func(ctx SpecContext) {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			_, err := h.clientFor(req)
			Expect(err).To(HaveOccurred())
		})

		It("builds a client when a session exists", func(ctx SpecContext) {
			rec := httptest.NewRecorder()
			h.sessionStore.Set(rec, &session.Data{Username: "alice"})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.AddCookie(rec.Result().Cookies()[0])

			_, err := h.clientFor(req)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("NewHandler", func() {
		It("wires the client seams from a kubeconfig", func() {
			kubeconfig := filepath.Join(GinkgoT().TempDir(), "kubeconfig")
			content := `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: http://localhost:8080
contexts:
- name: ctx
  context:
    cluster: c
    user: u
current-context: ctx
users:
- name: u
  user: {}
`
			Expect(os.WriteFile(kubeconfig, []byte(content), 0o600)).To(Succeed())
			store, _ := session.NewStore("")

			h, err := NewHandler(kubeconfig, store, auth.NewNoopProvider(), logr.Discard())
			Expect(err).NotTo(HaveOccurred())
			Expect(h.newClient).NotTo(BeNil())
			Expect(h.newClientset).NotTo(BeNil())
			Expect(h.newDiscovery).NotTo(BeNil())
		})

		It("errors when in-cluster config is unavailable", func() {
			store, _ := session.NewStore("")
			// Empty kubeconfig → in-cluster config, which fails outside a cluster.
			_, err := NewHandler("", store, auth.NewNoopProvider(), logr.Discard())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("helpers", func() {
		It("writeK8sError maps a StatusError to its code", func() {
			rec := httptest.NewRecorder()
			writeK8sError(rec, apierrors.NewNotFound(schema.GroupResource{Resource: "profiles"}, "p1"))
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("writeK8sError maps a generic error to 500", func() {
			rec := httptest.NewRecorder()
			writeK8sError(rec, context.DeadlineExceeded)
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})

		It("writeJSON still sets the content type on an encode error", func() {
			rec := httptest.NewRecorder()
			writeJSON(rec, make(chan int)) // channels can't be JSON-encoded
			Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))
		})

		It("builds the expected list/get/watch options", func() {
			Expect(getOptions()).To(Equal(metav1.GetOptions{}))
			Expect(listOptions().Watch).To(BeFalse())
			Expect(watchOptions().Watch).To(BeTrue())
		})
	})

	Describe("HandleSSE", func() {
		It("sets stream headers and returns when the request context is cancelled", func(specCtx SpecContext) {
			th := newFullHandler()
			ctx, cancel := context.WithCancel(specCtx)
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			req.SetPathValue("namespace", "default")
			rec := httptest.NewRecorder()

			done := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				th.h.HandleSSE()(rec, req)
				close(done)
			}()

			cancel()
			Eventually(done).Should(BeClosed())
			Expect(rec.Header().Get("Content-Type")).To(Equal("text/event-stream"))
		})
	})
})
