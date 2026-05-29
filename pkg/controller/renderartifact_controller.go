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
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	// DeleteTag overrides the OCI tag deletion function used during GC.
	// Defaults to ociregistry.DeleteTag; replaced in tests.
	DeleteTag func(ctx context.Context, rawRef string, auth authn.Authenticator) error
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderartifacts/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=renderbindings,verbs=get;list;watch
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
			if err := r.cleanupOCIArtifact(ctx, artifact); err != nil {
				// Failure is already logged + event fired inside cleanupOCIArtifact.
				// Keep the finalizer by returning the error so the object stays visible
				// with the OCICleanup=False condition set.
				return ctrl.Result{}, err
			}

			// OCI cleanup succeeded — remove finalizer to allow K8s deletion.
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

	// If no bindings remain, trigger GC by deleting this object.
	// The finalizer above will intercept the deletion and handle OCI cleanup.
	if len(bindingList.Items) == 0 {
		log.V(1).Info("No RenderBindings remain for RenderArtifact — triggering GC",
			"artifact", artifact.Name)
		if err := r.Delete(ctx, artifact); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, errLogAndWrap(log, err, "failed to delete orphaned RenderArtifact")
		}
	}

	return ctrl.Result{}, nil
}

// cleanupOCIArtifact attempts to delete the OCI tag from the registry.
// On failure it sets a status condition and fires a Warning event so the user
// can see why the RenderArtifact is stuck, then returns the error to keep the
// finalizer in place.
func (r *RenderArtifactReconciler) cleanupOCIArtifact(ctx context.Context, artifact *solarv1alpha1.RenderArtifact) error {
	log := ctrl.LoggerFrom(ctx)

	registryHost := strings.TrimPrefix(strings.TrimSuffix(artifact.Spec.BaseURL, "/"), "oci://")
	rawRef := registryHost + "/" + strings.TrimPrefix(artifact.Spec.Repository, "/") + ":" + artifact.Spec.Tag
	log.V(1).Info("Attempting OCI tag cleanup", "ref", rawRef)

	deleteFn := r.DeleteTag
	if deleteFn == nil {
		deleteFn = ociregistry.DeleteTag
	}

	auth, err := r.resolveAuth(ctx, artifact, registryHost)
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
	if err := deleteFn(deleteCtx, rawRef, auth); err != nil {
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

// resolveAuth builds an authn.Authenticator from the artifact's PushSecretRef.
// Returns authn.Anonymous if no secret is configured or if loading fails.
func (r *RenderArtifactReconciler) resolveAuth(ctx context.Context, artifact *solarv1alpha1.RenderArtifact, registryHost string) (authn.Authenticator, error) {
	log := ctrl.LoggerFrom(ctx)

	if artifact.Spec.PushSecretRef == nil {
		return authn.Anonymous, nil
	}

	secretNs := artifact.Namespace
	if artifact.Spec.PushSecretNamespace != "" {
		secretNs = artifact.Spec.PushSecretNamespace
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      artifact.Spec.PushSecretRef.Name,
		Namespace: secretNs,
	}, secret); err != nil {
		log.Error(err, "Failed to get push secret for OCI auth",
			"secret", artifact.Spec.PushSecretRef.Name)

		return nil, fmt.Errorf("failed to get push secret %s/%s: %w", secretNs, artifact.Spec.PushSecretRef.Name, err)
	}

	return ociAuthFromSecret(secret, registryHost), nil
}

func ociAuthFromSecret(secret *corev1.Secret, registryHost string) authn.Authenticator {
	if secret.Type == corev1.SecretTypeBasicAuth {
		user := string(secret.Data["username"])
		pass := string(secret.Data["password"])
		if user != "" || pass != "" {
			return authn.FromConfig(authn.AuthConfig{Username: user, Password: pass})
		}

		return authn.Anonymous
	}

	data := secret.Data[corev1.DockerConfigJsonKey]
	if len(data) == 0 {
		return authn.Anonymous
	}

	var cfg struct {
		Auths map[string]authn.AuthConfig `json:"auths"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return authn.Anonymous
	}

	if ac, ok := cfg.Auths[registryHost]; ok {
		return authn.FromConfig(ac)
	}

	if ac, ok := cfg.Auths["https://"+registryHost]; ok {
		return authn.FromConfig(ac)
	}

	return authn.Anonymous
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
