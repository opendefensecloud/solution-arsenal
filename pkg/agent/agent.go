// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	agentFinalizer = "solar.opendefense.cloud/agent-finalizer"

	cosignSecretNameSuffix = "cosign-pub"

	cosignSecretKey    = "cosign.pub"
	cosignSecretOldKey = "cosign.pub.old"

	ConditionTypeKeyDistributed = "KeyDistributed"
)

type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets,verbs=get;list;watch
// +kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	target := &solarv1alpha1.Target{}
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !target.DeletionTimestamp.IsZero() {
		secretKey := types.NamespacedName{
			Name:      cosignSecretName(target.Name),
			Namespace: target.Namespace,
		}
		return r.handleDeletion(ctx, target, secretKey)
	}

	if target.Status.PublicKey == "" {
		log.V(1).Info("Target has no public key yet, skipping")
		return ctrl.Result{}, nil
	}

	secretKey := types.NamespacedName{
		Name:      cosignSecretName(target.Name),
		Namespace: target.Namespace,
	}

	if err := r.ensureFinalizer(ctx, target); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureCosignSecret(ctx, target, secretKey); err != nil {
		return ctrl.Result{}, err
	}

	// Re-fetch the target to get the latest resourceVersion before updating status
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.setCondition(ctx, target, ConditionTypeKeyDistributed, metav1.ConditionTrue, "SecretReady",
		"Cosign public key Secret is up to date"); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) ensureFinalizer(ctx context.Context, target *solarv1alpha1.Target) error {
	if slices.Contains(target.Finalizers, agentFinalizer) {
		return nil
	}

	latest := &solarv1alpha1.Target{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(target), latest); err != nil {
		return err
	}

	if slices.Contains(latest.Finalizers, agentFinalizer) {
		return nil
	}

	original := latest.DeepCopy()
	latest.Finalizers = append(latest.Finalizers, agentFinalizer)
	return r.Patch(ctx, latest, client.MergeFrom(original))
}

func (r *Reconciler) handleDeletion(ctx context.Context, target *solarv1alpha1.Target, secretKey types.NamespacedName) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	secret := &corev1.Secret{}
	if err := r.Get(ctx, secretKey, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to get cosign Secret: %w", err)
		}
	} else {
		if err := r.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete cosign Secret: %w", err)
		}
		log.V(1).Info("Deleted cosign public key Secret", "secret", secretKey)
		if r.Recorder != nil {
			r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Deleted", "Delete",
				"Deleted cosign public key Secret %s", secretKey.Name)
		}
	}

	if slices.Contains(target.Finalizers, agentFinalizer) {
		latest := &solarv1alpha1.Target{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(target), latest); err != nil {
			return ctrl.Result{}, err
		}

		original := latest.DeepCopy()
		latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool {
			return s == agentFinalizer
		})
		if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) ensureCosignSecret(ctx context.Context, target *solarv1alpha1.Target, secretKey types.NamespacedName) error {
	log := ctrl.LoggerFrom(ctx)

	secret := &corev1.Secret{}
	err := r.Get(ctx, secretKey, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get cosign Secret: %w", err)
	}

	desiredData := buildSecretData(target)

	if err == nil {
		if secretDataMatches(secret.Data, desiredData) {
			return nil
		}

		secret.Data = desiredData
		if err := r.Update(ctx, secret); err != nil {
			return fmt.Errorf("failed to update cosign Secret: %w", err)
		}
		log.V(1).Info("Updated cosign public key Secret", "secret", secretKey)
		if r.Recorder != nil {
			r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Updated", "Update",
				"Updated cosign public key Secret %s", secretKey.Name)
		}
		return nil
	}

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretKey.Name,
			Namespace: secretKey.Namespace,
			Labels: map[string]string{
				"solar.opendefense.cloud/component": "cosign-pubkey",
				"solar.opendefense.cloud/target":    target.Name,
			},
		},
		Data: desiredData,
		Type: corev1.SecretTypeOpaque,
	}

	if err := controllerutil.SetControllerReference(target, secret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	if err := r.Create(ctx, secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create cosign Secret: %w", err)
		}
		return nil
	}

	log.V(1).Info("Created cosign public key Secret", "secret", secretKey)
	if r.Recorder != nil {
		r.Recorder.Eventf(target, nil, corev1.EventTypeNormal, "Created", "Create",
			"Created cosign public key Secret %s", secretKey.Name)
	}
	return nil
}

func buildSecretData(target *solarv1alpha1.Target) map[string][]byte {
	data := map[string][]byte{
		cosignSecretKey: []byte(target.Status.PublicKey),
	}

	if target.Status.PreviousPublicKey != "" {
		data[cosignSecretOldKey] = []byte(target.Status.PreviousPublicKey)
	}

	return data
}

func secretDataMatches(existing, desired map[string][]byte) bool {
	for k, v := range desired {
		if !slices.Equal(existing[k], v) {
			return false
		}
	}
	for k := range existing {
		if _, ok := desired[k]; !ok {
			return false
		}
	}
	return true
}

func (r *Reconciler) setCondition(ctx context.Context, target *solarv1alpha1.Target, condType string, status metav1.ConditionStatus, reason, message string) error {
	changed := apimeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: target.Generation,
		Reason:             reason,
		Message:            message,
	})
	if changed {
		return r.Status().Update(ctx, target)
	}
	return nil
}

func cosignSecretName(targetName string) string {
	return targetName + "-" + cosignSecretNameSuffix
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Target{}).
		Complete(r)
}
