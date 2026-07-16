// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"go.opendefense.cloud/solar/pkg/ui/auth"
	"go.opendefense.cloud/solar/pkg/ui/session"
)

const solarAPIGroup = "solar.opendefense.cloud"

const maxRequestBodyBytes = 1 << 20 // 1 MiB

// resourceMap maps resource names to their GVR.
var resourceMap = map[string]schema.GroupVersionResource{
	"targets":           {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "targets"},
	"releases":          {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "releases"},
	"releasebindings":   {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "releasebindings"},
	"components":        {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "components"},
	"componentversions": {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "componentversions"},
	"registries":        {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "registries"},
	"registrybindings":  {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "registrybindings"},
	"profiles":          {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "profiles"},
	"rendertasks":       {Group: "solar.opendefense.cloud", Version: "v1alpha1", Resource: "rendertasks"},
}

// Handler serves the K8s API proxy routes.
type Handler struct {
	baseConfig   *rest.Config
	sessionStore *session.Store
	authProvider auth.Provider
	log          logr.Logger
	newClient    func(r *http.Request) (dynamic.Interface, error)
}

// NewHandler creates a new API handler.
func NewHandler(kubeconfig string, store *session.Store, provider auth.Provider, log logr.Logger) (*Handler, error) {
	var cfg *rest.Config
	var err error

	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	h := &Handler{
		baseConfig:   cfg,
		sessionStore: store,
		authProvider: provider,
		log:          log.WithName("api"),
	}
	h.newClient = h.clientFor

	return h, nil
}

// clientFor returns a dynamic client for the given session.
func (h *Handler) clientFor(r *http.Request) (dynamic.Interface, error) {
	sess := h.sessionStore.Get(r)
	if sess == nil {
		return nil, fmt.Errorf("unauthorized: no session")
	}
	cfg := h.authProvider.WrapConfig(h.baseConfig, sess)

	return dynamic.NewForConfig(cfg)
}

// CanImpersonate runs a SelfSubjectAccessReview against the K8s API to ask
// whether the request's user is allowed to impersonate other users. This is
// the canonical "is admin" check: anyone permitted to impersonate users at
// the cluster level may also use the BFF's "preview as" feature.
//
// The result is cached on the session for the session's lifetime — RBAC
// doesn't change mid-session in practice, and this avoids a SSAR round-trip
// on every /auth/me call.
//
// The SSAR is always evaluated against the *real* identity even when the
// admin is currently previewing as another user; otherwise the cached answer
// would describe the previewed user's permissions, not the admin's.
//
// A missing session returns false without an error.
func (h *Handler) CanImpersonate(ctx context.Context, r *http.Request) (bool, error) {
	sess := h.sessionStore.Get(r)
	if sess == nil {
		return false, nil
	}
	if sess.CanImpersonate != nil {
		return *sess.CanImpersonate, nil
	}

	// Always evaluate against the real identity, not the active preview.
	realSess := *sess
	realSess.ImpersonatingAs = ""
	realSess.ImpersonatingGroups = nil

	cfg := h.authProvider.WrapConfig(h.baseConfig, &realSess)
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return false, fmt.Errorf("build clientset: %w", err)
	}
	res, err := cs.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &authzv1.SelfSubjectAccessReview{
		Spec: authzv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authzv1.ResourceAttributes{
				Verb:     "impersonate",
				Resource: "users",
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("SelfSubjectAccessReview: %w", err)
	}

	h.sessionStore.SetCanImpersonate(r, res.Status.Allowed)

	return res.Status.Allowed, nil
}

// CanListAllNamespaces returns true if the current identity (real or
// impersonated) is permitted to list namespaces at cluster scope. The
// frontend uses this to decide whether to offer "All namespaces" in the
// selector — without it, the cluster-wide list endpoints would just 403.
//
// Cached per session and invalidated when impersonation changes.
func (h *Handler) CanListAllNamespaces(ctx context.Context, r *http.Request) (bool, error) {
	sess := h.sessionStore.Get(r)
	if sess == nil {
		return false, nil
	}
	if sess.CanListAllNamespaces != nil {
		return *sess.CanListAllNamespaces, nil
	}

	cs, err := kubernetes.NewForConfig(h.authProvider.WrapConfig(h.baseConfig, sess))
	if err != nil {
		return false, fmt.Errorf("build clientset: %w", err)
	}
	res, err := cs.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &authzv1.SelfSubjectAccessReview{
		Spec: authzv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authzv1.ResourceAttributes{
				Verb:     "list",
				Resource: "namespaces",
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("SelfSubjectAccessReview: %w", err)
	}

	h.sessionStore.SetCanListAllNamespaces(r, res.Status.Allowed)

	return res.Status.Allowed, nil
}

// HandleMe returns the current user info, including canImpersonate which the
// frontend uses to decide whether to show the "Preview as" dropdown.
func (h *Handler) HandleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		sess := h.sessionStore.Get(r)
		if sess == nil {
			_, _ = w.Write([]byte(`{"authenticated":false}`))
			return
		}

		resp := map[string]any{
			"authenticated": true,
			"username":      sess.Username,
			"groups":        sess.Groups,
		}
		if sess.ImpersonatingAs != "" {
			resp["impersonating"] = map[string]any{
				"username": sess.ImpersonatingAs,
				"groups":   sess.ImpersonatingGroups,
			}
		}

		// Best-effort SSAR — if the apiserver call fails, fall back to
		// canImpersonate=false rather than failing the whole /me response.
		canImpersonate, err := h.CanImpersonate(r.Context(), r)
		if err != nil {
			h.log.Error(err, "SelfSubjectAccessReview (impersonate) failed; assuming non-admin")
		}
		resp["canImpersonate"] = canImpersonate

		canListAll, err := h.CanListAllNamespaces(r.Context(), r)
		if err != nil {
			h.log.Error(err, "SelfSubjectAccessReview (list namespaces) failed; assuming false")
		}
		resp["canListAllNamespaces"] = canListAll

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			h.log.Error(err, "failed to encode /me response")
		}
	}
}

// HandleList returns a handler that lists resources of the given type.
// If the route has no {namespace} path value (cluster-wide route), the
// dynamic client is called with an empty namespace, which K8s interprets
// as "across all namespaces". K8s RBAC determines whether the user is
// permitted to do that cluster-wide list.
func (h *Handler) HandleList(resource string) http.HandlerFunc {
	gvr, ok := resourceMap[resource]
	if !ok {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, fmt.Sprintf("unknown resource: %s", resource), http.StatusNotFound)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")

		client, err := h.clientFor(r)
		if err != nil {
			h.log.Error(err, "failed to create client")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		list, err := client.Resource(gvr).Namespace(namespace).List(r.Context(), listOptions())
		if err != nil {
			h.log.Error(err, "failed to list resources", "resource", resource, "namespace", namespace)
			writeK8sError(w, err)

			return
		}

		writeJSON(w, list)
	}
}

// HandleListNamespaces enumerates namespaces and returns only the ones in
// which the request's user has any SolAr API permission.
//
// Discovery uses the BFF's own credentials (not the user's), so the user
// does not need cluster-scoped `list namespaces` RBAC. Per-user filtering
// uses SelfSubjectRulesReview against each candidate namespace — every
// authenticated user may run this on themselves regardless of bindings.
//
// "Has access" is defined as: the user has at least one resource rule whose
// APIGroups includes `solar.opendefense.cloud` (or `*`). That excludes
// namespaces where the user can only see kube objects.
func (h *Handler) HandleListNamespaces() http.HandlerFunc {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}

	return func(w http.ResponseWriter, r *http.Request) {
		sess := h.sessionStore.Get(r)
		if sess == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Discovery client = BFF's own creds (kubeconfig / SA).
		discovery, err := dynamic.NewForConfig(h.baseConfig)
		if err != nil {
			h.log.Error(err, "failed to build discovery client")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}
		nsList, err := discovery.Resource(gvr).List(r.Context(), listOptions())
		if err != nil {
			h.log.Error(err, "failed to list namespaces (discovery)")
			writeK8sError(w, err)

			return
		}

		// Review client = user identity (or admin previewing as persona).
		userCS, err := kubernetes.NewForConfig(h.authProvider.WrapConfig(h.baseConfig, sess))
		if err != nil {
			h.log.Error(err, "failed to build user clientset")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		// Parallel SSRR per namespace.
		ctx := r.Context()
		var wg sync.WaitGroup
		var mu sync.Mutex
		filtered := make([]unstructured.Unstructured, 0, len(nsList.Items))
		for i := range nsList.Items {
			ns := nsList.Items[i]
			wg.Add(1)
			go func(ctx context.Context) {
				defer wg.Done()
				if hasSolarAccess(ctx, userCS, ns.GetName(), h.log) {
					mu.Lock()
					filtered = append(filtered, ns)
					mu.Unlock()
				}
			}(ctx)
		}
		wg.Wait()

		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].GetName() < filtered[j].GetName()
		})
		nsList.Items = filtered

		writeJSON(w, nsList)
	}
}

// hasSolarAccess runs SelfSubjectRulesReview in `namespace` and returns
// true if any of the returned resource rules covers the SolAr API group.
func hasSolarAccess(ctx context.Context, cs kubernetes.Interface, namespace string, log logr.Logger) bool {
	review, err := cs.AuthorizationV1().SelfSubjectRulesReviews().Create(ctx, &authzv1.SelfSubjectRulesReview{
		Spec: authzv1.SelfSubjectRulesReviewSpec{Namespace: namespace},
	}, metav1.CreateOptions{})
	if err != nil {
		log.V(1).Info("SSRR failed; excluding namespace", "namespace", namespace, "error", err.Error())
		return false
	}
	for _, rule := range review.Status.ResourceRules {
		for _, group := range rule.APIGroups {
			if group == solarAPIGroup || group == "*" {
				return true
			}
		}
	}

	return false
}

// HandleGet returns a handler that gets a single resource.
func (h *Handler) HandleGet(resource string) http.HandlerFunc {
	gvr, ok := resourceMap[resource]
	if !ok {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, fmt.Sprintf("unknown resource: %s", resource), http.StatusNotFound)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		name := r.PathValue("name")

		client, err := h.clientFor(r)
		if err != nil {
			h.log.Error(err, "failed to create client")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		obj, err := client.Resource(gvr).Namespace(namespace).Get(r.Context(), name, getOptions())
		if err != nil {
			h.log.Error(err, "failed to get resource", "resource", resource, "namespace", namespace, "name", name)
			writeK8sError(w, err)

			return
		}

		writeJSON(w, obj)
	}
}

// HandleCreate creates a resource in the namespace taken from the path.
func (h *Handler) HandleCreate(resource string) http.HandlerFunc {
	gvr, ok := resourceMap[resource]
	if !ok {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, fmt.Sprintf("unknown resource: %s", resource), http.StatusNotFound)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

		var obj unstructured.Unstructured
		if err := json.NewDecoder(r.Body).Decode(&obj.Object); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		// A JSON null/scalar decodes without error but leaves a nil object;
		// reject it rather than sending a bodyless create to the apiserver.
		if obj.Object == nil {
			http.Error(w, "request body must be a JSON object", http.StatusBadRequest)
			return
		}
		obj.SetNamespace(namespace)

		client, err := h.newClient(r)
		if err != nil {
			h.log.Error(err, "failed to create client")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		created, err := client.Resource(gvr).Namespace(namespace).Create(r.Context(), &obj, metav1.CreateOptions{})
		if err != nil {
			h.log.Error(err, "failed to create resource", "resource", resource, "namespace", namespace)
			writeK8sError(w, err)

			return
		}

		writeJSON(w, created)
	}
}

// HandlePatch applies a patch to a resource.
func (h *Handler) HandlePatch(resource string) http.HandlerFunc {
	gvr, ok := resourceMap[resource]
	if !ok {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, fmt.Sprintf("unknown resource: %s", resource), http.StatusNotFound)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		name := r.PathValue("name")
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

		patch, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		patchType := types.MergePatchType
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json-patch+json") {
			patchType = types.JSONPatchType
		}

		client, err := h.newClient(r)
		if err != nil {
			h.log.Error(err, "failed to create client")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		updated, err := client.Resource(gvr).Namespace(namespace).
			Patch(r.Context(), name, patchType, patch, metav1.PatchOptions{})
		if err != nil {
			h.log.Error(err, "failed to patch resource", "resource", resource, "namespace", namespace, "name", name)
			writeK8sError(w, err)

			return
		}

		writeJSON(w, updated)
	}
}

// HandleDelete deletes a resource
func (h *Handler) HandleDelete(resource string) http.HandlerFunc {
	gvr, ok := resourceMap[resource]
	if !ok {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, fmt.Sprintf("unknown resource: %s", resource), http.StatusNotFound)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		name := r.PathValue("name")

		client, err := h.newClient(r)
		if err != nil {
			h.log.Error(err, "failed to create client")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		if err := client.Resource(gvr).Namespace(namespace).Delete(r.Context(), name, metav1.DeleteOptions{}); err != nil {
			h.log.Error(err, "failed to delete resource", "resource", resource, "namespace", namespace, "name", name)
			writeK8sError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleSSE returns a handler that streams resource watch events as SSE.
// An empty {namespace} path value (the cluster-wide /api/events route)
// watches across all namespaces; K8s RBAC decides per-resource whether the
// user is allowed to do that, and individual watches that 403 are skipped
// rather than failing the whole stream.
func (h *Handler) HandleSSE() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")

		client, err := h.clientFor(r)
		if err != nil {
			h.log.Error(err, "failed to create client")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher.Flush()

		// Watch all solar resources and multiplex into SSE via a channel.
		// Namespace on the event is read from the object metadata so
		// cluster-wide watches still deliver the originating namespace.
		type sseEvent struct {
			Type      string `json:"type"`
			Resource  string `json:"resource"`
			Namespace string `json:"namespace"`
		}
		events := make(chan sseEvent, 64)

		ctx := r.Context()
		for resourceName, gvr := range resourceMap {
			go func(ctx context.Context) {
				watcher, err := client.Resource(gvr).Namespace(namespace).Watch(ctx, watchOptions())
				if err != nil {
					h.log.Error(err, "failed to watch", "resource", resourceName)

					return
				}
				defer watcher.Stop()

				for event := range watcher.ResultChan() {
					eventNs := namespace
					if obj, ok := event.Object.(metav1.Object); ok {
						eventNs = obj.GetNamespace()
					}
					select {
					case events <- sseEvent{
						Type:      string(event.Type),
						Resource:  resourceName,
						Namespace: eventNs,
					}:
					case <-ctx.Done():
						return
					}
				}
			}(ctx)
		}

		// Single writer goroutine — serializes all writes and respects client disconnect
		for {
			select {
			case evt := <-events:
				b, _ := json.Marshal(evt)
				fmt.Fprintf(w, "data: %s\n\n", b)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
