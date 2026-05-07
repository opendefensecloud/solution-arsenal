// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/go-logr/logr"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"go.opendefense.cloud/solar/pkg/ui/auth"
	"go.opendefense.cloud/solar/pkg/ui/session"
)

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
	clientset    kubernetes.Interface
	sessionStore *session.Store
	authProvider auth.Provider
	log          logr.Logger
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

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return &Handler{
		baseConfig:   cfg,
		clientset:    clientset,
		sessionStore: store,
		authProvider: provider,
		log:          log.WithName("api"),
	}, nil
}

// clientFor returns a dynamic client for the given session.
func (h *Handler) clientFor(r *http.Request) (dynamic.Interface, error) {
	sess := h.sessionStore.Get(r)
	cfg := h.authProvider.WrapConfig(h.baseConfig, sess)

	return dynamic.NewForConfig(cfg)
}

// isAdminUser returns true when the session user is a subject of any
// ClusterRoleBinding labeled solar.opendefense.cloud/admin=true.
// Uses the BFF's own service-account credentials so the check is immune to
// any active session-level impersonation override.
func (h *Handler) isAdminUser(ctx context.Context, sess *session.Data) bool {
	bindingList, err := h.clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{
		LabelSelector: adminLabel + "=true",
	})
	if err != nil {
		h.log.Error(err, "failed to list admin ClusterRoleBindings")

		return false
	}

	for _, binding := range bindingList.Items {
		for _, subject := range binding.Subjects {
			switch subject.Kind {
			case "User":
				if subject.Name == sess.Username {
					return true
				}
			case "Group":
				if slices.Contains(sess.Groups, subject.Name) {
					return true
				}
			}
		}
	}

	return false
}

// RequireAdmin returns a middleware that rejects requests from users who are not
// listed as subjects in a ClusterRoleBinding labeled solar.opendefense.cloud/admin=true.
func (h *Handler) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := h.sessionStore.Get(r)
		if sess == nil || !h.isAdminUser(r.Context(), sess) {
			http.Error(w, "forbidden", http.StatusForbidden)

			return
		}

		next(w, r)
	}
}

// HandleMe returns the current user info, including an isAdmin flag derived
// from membership in a ClusterRoleBinding labeled solar.opendefense.cloud/admin=true
func (h *Handler) HandleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := h.sessionStore.Get(r)
		if data == nil {
			writeJSON(w, map[string]any{"authenticated": false})

			return
		}

		resp := map[string]any{
			"authenticated": true,
			"username":      data.Username,
			"groups":        data.Groups,
			"isAdmin":       h.isAdminUser(r.Context(), data),
		}

		if data.ImpersonatingAs != "" {
			resp["impersonating"] = map[string]any{
				"username": data.ImpersonatingAs,
				"groups":   data.ImpersonatingGroups,
			}
		}

		writeJSON(w, resp)
	}
}

// HandleList returns a handler that lists resources of the given type.
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

// HandleSSE returns a handler that streams resource watch events as SSE.
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

		// Watch all solar resources and multiplex into SSE via a channel
		type sseEvent struct {
			Type      string `json:"type"`
			Resource  string `json:"resource"`
			Namespace string `json:"namespace"`
		}
		events := make(chan sseEvent, 64)

		for resourceName, gvr := range resourceMap {
			go func(ctx context.Context) {
				watcher, err := client.Resource(gvr).Namespace(namespace).Watch(ctx, watchOptions())
				if err != nil {
					h.log.Error(err, "failed to watch", "resource", resourceName)

					return
				}
				defer watcher.Stop()

				for event := range watcher.ResultChan() {
					select {
					case events <- sseEvent{
						Type:      string(event.Type),
						Resource:  resourceName,
						Namespace: namespace,
					}:
					case <-ctx.Done():
						return
					}
				}
			}(r.Context())
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

// permissionRule is the JSON representation of a Kubernetes ResourceRule.
type permissionRule struct {
	Verbs     []string `json:"verbs"`
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
}

// permissionsResponse is the response body for HandlePermissions.
type permissionsResponse struct {
	Incomplete bool             `json:"incomplete"`
	Rules      []permissionRule `json:"rules"`
}

// HandlePermissions calls SelfSubjectRulesReview using the caller's own
// credentials and returns the resulting resource rules for the namespace.
// The frontend uses this to show/hide pages based on RBAC permissions.
func (h *Handler) HandlePermissions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")

		sess := h.sessionStore.Get(r)
		cfg := h.authProvider.WrapConfig(h.baseConfig, sess)

		clientset, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			h.log.Error(err, "failed to create kubernetes clientset")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		review := &authorizationv1.SelfSubjectRulesReview{
			Spec: authorizationv1.SelfSubjectRulesReviewSpec{
				Namespace: namespace,
			},
		}

		result, err := clientset.AuthorizationV1().SelfSubjectRulesReviews().Create(
			r.Context(), review, metav1.CreateOptions{},
		)
		if err != nil {
			h.log.Error(err, "failed to evaluate self-subject rules", "namespace", namespace)
			writeK8sError(w, err)

			return
		}

		rules := make([]permissionRule, 0, len(result.Status.ResourceRules))
		for _, rr := range result.Status.ResourceRules {
			rules = append(rules, permissionRule{
				Verbs:     rr.Verbs,
				APIGroups: rr.APIGroups,
				Resources: rr.Resources,
			})
		}

		writeJSON(w, permissionsResponse{
			Incomplete: result.Status.Incomplete,
			Rules:      rules,
		})
	}
}

// impersonatableLabel is the well-known label that marks a ClusterRole as
// defining an impersonatable user persona.
const impersonatableLabel = "solar.opendefense.cloud/impersonatable"

// adminLabel is the well-known label that marks a ClusterRoleBinding as
// granting solar-ui admin access.
const adminLabel = "solar.opendefense.cloud/admin"

// ImpersonationTarget describes a user persona that an admin can preview as.
type ImpersonationTarget struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
}

// listImpersonationTargets reads ClusterRoles labeled
// solar.opendefense.cloud/impersonatable=true using the BFF's own service
// account credentials (not the logged-in user's). Each ClusterRole is expected
// to grant the impersonate verb on both the "users" and "groups" resources; the
// resourceNames in those rules define the username and group membership of the
// persona respectively.
func (h *Handler) listImpersonationTargets(ctx context.Context) ([]ImpersonationTarget, error) {
	roleList, err := h.clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{
		LabelSelector: impersonatableLabel + "=true",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list impersonation ClusterRoles: %w", err)
	}

	targets := make([]ImpersonationTarget, 0, len(roleList.Items))

	for _, role := range roleList.Items {
		var username string
		var groups []string

		for _, rule := range role.Rules {
			if !slices.Contains(rule.Verbs, "impersonate") && !slices.Contains(rule.Verbs, "*") {
				continue
			}

			for _, resource := range rule.Resources {
				switch resource {
				case "users":
					// FIXME: currently only the first resourceName is used if multiple are present.
					// we could support multiple users per ClusterRole if needed
					if len(rule.ResourceNames) > 0 {
						username = rule.ResourceNames[0]
					}
				case "groups":
					groups = append(groups, rule.ResourceNames...)
				}
			}
		}

		if username != "" {
			targets = append(targets, ImpersonationTarget{
				Username: username,
				Groups:   groups,
			})
		}
	}

	return targets, nil
}

// HandleListImpersonationTargets returns the available impersonation personas
// for the admin "preview as" feature. Requires admin privileges (enforced by
// the server-level middleware); the lookup itself uses BFF credentials.
func (h *Handler) HandleListImpersonationTargets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targets, err := h.listImpersonationTargets(r.Context())
		if err != nil {
			h.log.Error(err, "failed to list impersonation targets")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		writeJSON(w, targets)
	}
}

// HandleImpersonate validates the requested username against the ClusterRole-
// defined impersonation personas and activates the override on the session.
func (h *Handler) HandleImpersonate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
			http.Error(w, "invalid request body: username is required", http.StatusBadRequest)

			return
		}

		targets, err := h.listImpersonationTargets(r.Context())
		if err != nil {
			h.log.Error(err, "failed to list impersonation targets")
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		var target *ImpersonationTarget

		for i := range targets {
			if targets[i].Username == req.Username {
				target = &targets[i]

				break
			}
		}

		if target == nil {
			http.Error(w, "unknown impersonation target", http.StatusBadRequest)

			return
		}

		if !h.sessionStore.SetImpersonation(r, target.Username, target.Groups) {
			http.Error(w, "no session found", http.StatusUnauthorized)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleClearImpersonation removes the impersonation override from the session.
func (h *Handler) HandleClearImpersonation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.sessionStore.ClearImpersonation(r) {
			http.Error(w, "no session found", http.StatusUnauthorized)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
