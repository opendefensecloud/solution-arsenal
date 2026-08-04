// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/ociregistry"
)

const (
	renderArtifactFinalizer = "solar.opendefense.cloud/render-artifact-finalizer"
	ConditionTypeOCICleanup = "OCICleanup"
)

// RenderArtifactReconciler reconciles RenderArtifact objects.
// It sets status.ChartURL and acts as the GC controller: when the last RenderBinding
// referencing a RenderArtifact is removed, it attempts to delete the OCI tag
// and then deletes the RenderArtifact object itself.
//
// OCI tag deletion failures are surfaced as a status condition and a Warning event
// so users have visibility; the finalizer is kept until the deletion succeeds,
// making the artifact object "stuck" in a visible state.
type RenderArtifactReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  events.EventRecorder
	APIReader client.Reader
	// DeleteTag overrides the OCI tag deletion function used during GC.
	// Defaults to ociregistry.DeleteTag; replaced in tests.
	DeleteTag func(ctx context.Context, rawRef string, auth authn.Authenticator, insecure bool) error
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderbindings,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registries,verbs=get
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=referencegrants,verbs=get;list
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get

func (r *RenderArtifactReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("RenderArtifact is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	artifact := &solarv1alpha1.RenderArtifact{}
	if err := r.Get(ctx, req.NamespacedName, artifact); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get RenderArtifact")
	}

	// Handle deletion: attempt OCI tag cleanup, surface errors explicitly, then remove finalizer.
	if !artifact.DeletionTimestamp.IsZero() {
		if slices.Contains(artifact.Finalizers, renderArtifactFinalizer) {
			bound, err := r.renderArtifactBound(ctx, artifact)
			if err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to re-check RenderBindings for terminating RenderArtifact")
			}

			if bound {
				// A binding still references this artifact: someone still needs the OCI tag,
				// so keep it. The finalizer is retained on purpose: as long as the binding
				// exists the object must not be deleted, or the tag would be orphaned in the
				// registry (e.g. during namespace teardown nothing would recreate it). Once
				// the binding is gone (Target deletion removes its bindings), a follow-up
				// reconcile takes the not-bound path below and cleans up the tag.
				log.V(1).Info("RenderArtifact is terminating but still referenced by a RenderBinding; keeping OCI tag",
					"artifact", artifact.Name)
				r.Recorder.Eventf(artifact, nil, corev1.EventTypeNormal, "OCICleanupSkipped", "Delete",
					"RenderArtifact is terminating but still referenced by a RenderBinding; keeping OCI tag")

				return ctrl.Result{}, nil
			} else {
				if err := r.cleanupOCIArtifact(ctx, artifact); err != nil {
					// Failure is already logged + event fired inside cleanupOCIArtifact.
					// Keep the finalizer by returning the error so the object stays visible
					// with the OCICleanup=False condition set.
					return ctrl.Result{}, err
				}
			}

			// Remove finalizer to allow K8s deletion.
			latest := artifact.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool {
				return s == renderArtifactFinalizer
			})
			if err := r.Patch(ctx, latest, client.MergeFrom(artifact)); err != nil {
				return ctrl.Result{}, errLogAndWrap(log, err, "failed to remove finalizer from RenderArtifact")
			}
		}

		return ctrl.Result{}, nil
	}

	// Ensure finalizer is set.
	if !slices.Contains(artifact.Finalizers, renderArtifactFinalizer) {
		latest := artifact.DeepCopy()
		latest.Finalizers = append(latest.Finalizers, renderArtifactFinalizer)
		if err := r.Patch(ctx, latest, client.MergeFrom(artifact)); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to add finalizer to RenderArtifact")
		}

		return ctrl.Result{}, nil
	}

	// Populate status.ChartURL from spec coordinates if not yet set.
	chartURL := renderChartURL(artifact.Spec.BaseURL, artifact.Spec.Repository, artifact.Spec.Tag)
	if artifact.Status.ChartURL != chartURL {
		base := artifact.DeepCopy()
		artifact.Status.ChartURL = chartURL
		if err := r.Status().Patch(ctx, artifact, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to update RenderArtifact status")
		}
	}

	// List RenderBindings referencing this artifact.
	bindingList := &solarv1alpha1.RenderBindingList{}
	if err := r.List(ctx, bindingList,
		client.InNamespace(artifact.Namespace),
		client.MatchingFields{indexRenderBindingArtifactName: artifact.Name},
	); err != nil {
		return ctrl.Result{}, errLogAndWrap(log, err, "failed to list RenderBindings for RenderArtifact")
	}

	if len(bindingList.Items) > 0 {
		// While at least one binding exists, keep the artifact's RegistryRef pinned to
		// a binding that still exists.
		if err := r.repinCredentials(ctx, artifact, bindingList.Items); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to re-pin RenderArtifact credentials")
		}
	} else {
		// If no bindings remain, trigger GC by deleting this object.
		// The finalizer above will intercept the deletion and handle OCI cleanup.
		// Confirm via direct API call — cache may lag on concurrent creates.
		confirmed := &solarv1alpha1.RenderBindingList{}
		if err := r.APIReader.List(ctx, confirmed, client.InNamespace(artifact.Namespace)); err != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to confirm RenderBinding absence via API")
		}
		for i := range confirmed.Items {
			if confirmed.Items[i].Spec.RenderArtifactRef.Name == artifact.Name {
				// A binding exists in the API server that the cache missed.
				return ctrl.Result{}, nil
			}
		}
		log.V(1).Info("No RenderBindings remain for RenderArtifact — triggering GC",
			"artifact", artifact.Name)
		if err := r.Delete(ctx, artifact); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete orphaned RenderArtifact")
		}
	}

	return ctrl.Result{}, nil
}

// renderArtifactBound reports whether any RenderBinding still references the artifact.
// Used during deletion so the OCI tag is not deleted while a Target still needs it.
// Confirms via APIReader because the cache may lag on concurrent binding creates.
func (r *RenderArtifactReconciler) renderArtifactBound(ctx context.Context, artifact *solarv1alpha1.RenderArtifact) (bool, error) {
	bindingList := &solarv1alpha1.RenderBindingList{}
	if err := r.List(ctx, bindingList,
		client.InNamespace(artifact.Namespace),
		client.MatchingFields{indexRenderBindingArtifactName: artifact.Name},
	); err != nil {
		return false, err
	}
	if len(bindingList.Items) > 0 {
		return true, nil
	}

	confirmed := &solarv1alpha1.RenderBindingList{}
	if err := r.APIReader.List(ctx, confirmed, client.InNamespace(artifact.Namespace)); err != nil {
		return false, err
	}
	for i := range confirmed.Items {
		if confirmed.Items[i].Spec.RenderArtifactRef.Name == artifact.Name {
			return true, nil
		}
	}

	return false, nil
}

// cleanupOCIArtifact attempts to delete the OCI tag from the registry.
// On failure it sets a status condition and fires a Warning event so the user
// can see why the RenderArtifact is stuck, then returns the error to keep the
// finalizer in place.
func (r *RenderArtifactReconciler) cleanupOCIArtifact(ctx context.Context, artifact *solarv1alpha1.RenderArtifact) error {
	log := ctrl.LoggerFrom(ctx)

	registryHost := normalizeRegistryHost(artifact.Spec.BaseURL)
	rawRef := registryHost + "/" + strings.TrimPrefix(artifact.Spec.Repository, "/") + ":" + artifact.Spec.Tag
	log.V(1).Info("Attempting OCI tag cleanup", "ref", rawRef)

	deleteFn := r.DeleteTag
	if deleteFn == nil {
		deleteFn = ociregistry.DeleteTag
	}

	auth, plainHTTP, err := r.resolveAuth(ctx, artifact, registryHost)
	if err != nil {
		log.Error(err, "Failed to resolve OCI auth; RenderArtifact will remain until secret is accessible",
			"artifact", artifact.Name)
		r.Recorder.Eventf(artifact, nil, corev1.EventTypeWarning,
			"OCICleanupFailed", "Delete",
			"Failed to resolve OCI auth for %s: %s", rawRef, err.Error())

		latest := artifact.DeepCopy()
		apimeta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeOCICleanup,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: artifact.Generation,
			Reason:             "AuthFailed",
			Message:            err.Error(),
		})
		if sErr := r.Status().Patch(ctx, latest, client.MergeFrom(artifact)); sErr != nil {
			log.Error(sErr, "failed to update status condition after OCI auth failure")
		}

		return err
	}

	deleteCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := deleteFn(deleteCtx, rawRef, auth, plainHTTP); err != nil {
		// If the tag is already gone, proceed normally.
		var transportErr *transport.Error
		if errors.As(err, &transportErr) && transportErr.StatusCode == http.StatusNotFound {
			log.V(1).Info("OCI tag already absent — skipping delete", "ref", rawRef)
			return nil
		}

		log.Error(err, "Failed to delete OCI tag; RenderArtifact will remain until deletion succeeds",
			"ref", rawRef, "artifact", artifact.Name)
		r.Recorder.Eventf(artifact, nil, corev1.EventTypeWarning,
			"OCICleanupFailed", "Delete",
			"Failed to delete OCI tag %s: %s", rawRef, err.Error())

		latest := artifact.DeepCopy()
		apimeta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeOCICleanup,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: artifact.Generation,
			Reason:             "DeleteFailed",
			Message:            err.Error(),
		})
		// Status patch, if it fails, the event + log are visible in kubectl
		if sErr := r.Status().Patch(ctx, latest, client.MergeFrom(artifact)); sErr != nil {
			log.Error(sErr, "failed to update status condition after OCI cleanup failure")
		}

		return err
	}

	log.V(1).Info("OCI tag deleted successfully", "ref", rawRef)
	r.Recorder.Eventf(artifact, nil, corev1.EventTypeNormal,
		"OCICleanupSucceeded", "Delete",
		"Successfully deleted OCI tag %s", rawRef)

	return nil
}

// repinCredentials keeps artifact.Spec.RegistryRef pinned to the Registry snapshotted on
// a still-existing RenderBinding. Bindings that carry no RegistryRef are not candidates;
// among the rest the lowest name wins, so repeated reconciles converge instead of flapping
// between equally-valid choices.
// Because this runs on every RenderBinding create/update/delete event (see
// mapRenderBindingToArtifact), the artifact's pinned RegistryRef is always synced to a
// binding that exists, including immediately after the second-to-last binding is
// removed, which is exactly the moment that matters: it leaves the artifact holding a
// Registry reference that was valid for the binding that survives until the final
// removal, which is what the finalizer step needs to delete the OCI tag.
func (r *RenderArtifactReconciler) repinCredentials(ctx context.Context, artifact *solarv1alpha1.RenderArtifact, bindings []solarv1alpha1.RenderBinding) error {
	slices.SortFunc(bindings, func(a, b solarv1alpha1.RenderBinding) int { return strings.Compare(a.Name, b.Name) })

	// RegistryRef is optional, so bindings written before it existed carry nil. Skip those
	// instead of pinning nil over a working reference
	// If no binding carries a reference, keep what the artifact
	// already has: a stale-but-valid ref deletes the tag, nil does not.
	idx := slices.IndexFunc(bindings, func(b solarv1alpha1.RenderBinding) bool {
		return b.Spec.RegistryRef != nil
	})
	if idx < 0 {
		return nil
	}
	chosen := bindings[idx]

	if registryRefEqual(artifact.Spec.RegistryRef, chosen.Spec.RegistryRef) {
		return nil
	}

	latest := artifact.DeepCopy()
	latest.Spec.RegistryRef = chosen.Spec.RegistryRef

	return r.Patch(ctx, latest, client.MergeFrom(artifact))
}

func registryRefEqual(a, b *solarv1alpha1.ObjectReference) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

func (r *RenderArtifactReconciler) resolveAuth(ctx context.Context, artifact *solarv1alpha1.RenderArtifact, registryHost string) (authn.Authenticator, bool, error) {
	log := ctrl.LoggerFrom(ctx)

	if artifact.Spec.RegistryRef == nil {
		return authn.Anonymous, false, nil
	}

	registryNamespace := artifact.Namespace
	if artifact.Spec.RegistryRef.Namespace != "" {
		registryNamespace = artifact.Spec.RegistryRef.Namespace
	}

	// RegistryRef is meant to be controller-owned, but nothing stops a principal with
	// create/update on RenderArtifact from authoring one. Riding the Target's grant would
	// then hand those credentials to anyone who can write a RenderArtifact in a namespace
	// some Target happens to be granted from, so the grant must name RenderArtifact itself.
	if registryNamespace != artifact.Namespace {
		granted, err := registryGranted(ctx, r.APIReader, registryNamespace, "RenderArtifact", artifact.Namespace)
		if err != nil {
			return nil, false, fmt.Errorf("failed to check ReferenceGrant for Registry %s/%s: %w",
				registryNamespace, artifact.Spec.RegistryRef.Name, err)
		}
		if !granted {
			return nil, false, fmt.Errorf(
				"no ReferenceGrant in namespace %s with from[].kind=RenderArtifact, from[].namespace=%s and to[].kind=Registry "+
					"allows RenderArtifact %s/%s to access Registry %s/%s",
				registryNamespace, artifact.Namespace,
				artifact.Namespace, artifact.Name, registryNamespace, artifact.Spec.RegistryRef.Name)
		}
	}

	registry := &solarv1alpha1.Registry{}
	if err := r.APIReader.Get(ctx, client.ObjectKey{
		Name:      artifact.Spec.RegistryRef.Name,
		Namespace: registryNamespace,
	}, registry); err != nil {
		return nil, false, fmt.Errorf("failed to get Registry %s/%s: %w", registryNamespace, artifact.Spec.RegistryRef.Name, err)
	}

	// The grant authorizes this namespace to use the Registry, not to use its credentials
	// against an arbitrary host. spec.baseURL is what the delete is aimed at, and a Registry
	// secret may hold auths for several hosts, so refuse unless the artifact points at the
	// Registry's own hostname. Artifacts the Target controller produced always do
	if artifactHost, registryHostname := registryHost, normalizeRegistryHost(registry.Spec.Hostname); artifactHost != registryHostname {
		return nil, false, fmt.Errorf(
			"RenderArtifact %s/%s targets host %q but Registry %s/%s serves %q; refusing to use its credentials",
			artifact.Namespace, artifact.Name, artifactHost, registryNamespace, registry.Name, registryHostname)
	}

	if registry.Spec.SolarSecretRef == nil {
		return authn.Anonymous, registry.Spec.PlainHTTP, nil
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      registry.Spec.SolarSecretRef.Name,
		Namespace: registry.Namespace,
	}, secret); err != nil {
		log.Error(err, "Failed to get push secret for OCI auth",
			"secret", registry.Spec.SolarSecretRef.Name)

		return nil, false, fmt.Errorf("failed to get push secret %s/%s: %w", registry.Namespace, registry.Spec.SolarSecretRef.Name, err)
	}

	auth, err := ociAuthFromSecret(secret, registryHost)
	if err != nil {
		// A malformed dockerconfigjson is a configuration error; log it so the operator
		// is aware, but fall back to anonymous rather than blocking OCI cleanup.
		log.Error(err, "Malformed push secret; falling back to anonymous OCI auth",
			"secret", fmt.Sprintf("%s/%s", registry.Namespace, registry.Spec.SolarSecretRef.Name))
	}

	return auth, registry.Spec.PlainHTTP, nil
}

// normalizeRegistryHost strips the oci:// scheme and any trailing slash so a
// Registry hostname and an artifact baseURL can be compared as written by either side.
func normalizeRegistryHost(s string) string {
	return strings.TrimPrefix(strings.TrimSuffix(s, "/"), "oci://")
}

// ociAuthFromSecret extracts OCI credentials from a Kubernetes Secret.
// callers should log the error and decide whether to fall back to anonymous or abort.
func ociAuthFromSecret(secret *corev1.Secret, registryHost string) (authn.Authenticator, error) {
	if secret.Type == corev1.SecretTypeBasicAuth {
		user := string(secret.Data["username"])
		pass := string(secret.Data["password"])
		if user != "" || pass != "" {
			return authn.FromConfig(authn.AuthConfig{Username: user, Password: pass}), nil
		}

		return authn.Anonymous, nil
	}

	data := secret.Data[corev1.DockerConfigJsonKey]
	if len(data) == 0 {
		return authn.Anonymous, nil
	}

	var cfg struct {
		Auths map[string]authn.AuthConfig `json:"auths"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return authn.Anonymous, fmt.Errorf("failed to parse dockerconfigjson in secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	if ac, ok := cfg.Auths[registryHost]; ok {
		return authn.FromConfig(ac), nil
	}

	if ac, ok := cfg.Auths["https://"+registryHost]; ok {
		return authn.FromConfig(ac), nil
	}

	return authn.Anonymous, nil
}

// mapRenderBindingToArtifact maps a RenderBinding event to a reconcile request
// for the RenderArtifact it references, so the GC controller is triggered on
// every RenderBinding deletion.
func mapRenderBindingToArtifact(_ context.Context, obj client.Object) []reconcile.Request {
	rb, ok := obj.(*solarv1alpha1.RenderBinding)
	if !ok {
		return nil
	}

	if rb.Spec.RenderArtifactRef.Name == "" {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      rb.Spec.RenderArtifactRef.Name,
				Namespace: rb.Namespace,
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RenderArtifactReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.RenderArtifact{}).
		Watches(
			&solarv1alpha1.RenderBinding{},
			handler.EnqueueRequestsFromMapFunc(mapRenderBindingToArtifact),
		).
		Complete(r)
}
