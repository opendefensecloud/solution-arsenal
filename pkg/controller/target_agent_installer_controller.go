// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// ConditionTypeAgentInstalled reflects whether solar-agent has been
// installed onto a Target's cluster via its AgentAccessSecretRef.
const ConditionTypeAgentInstalled = "AgentInstalled"

// AgentInstaller installs solar-agent onto the cluster reachable via
// restConfig. HelmAgentInstaller is the real implementation; tests inject a
// fake so this reconciler's tests never make real Helm calls.
type AgentInstaller interface {
	Install(ctx context.Context, restConfig *rest.Config, target *solarv1alpha1.Target) error
}

// TargetAgentInstallerReconciler installs solar-agent onto a Target's own
// cluster when the Target carries an AgentAccessSecretRef, instead of
// waiting for it to be deployed there manually. See "Workflow B" in
// docs/superpowers/specs/2026-07-07-solar-agent-design.md.
type TargetAgentInstallerReconciler struct {
	client.Client
	Installer AgentInstaller
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *TargetAgentInstallerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	target := &solarv1alpha1.Target{}
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get target")
	}

	if target.Spec.AgentAccessSecretRef == nil {
		return ctrl.Result{}, nil
	}

	if apimeta.IsStatusConditionTrue(target.Status.Conditions, ConditionTypeAgentInstalled) {
		return ctrl.Result{}, nil
	}

	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{Namespace: target.Namespace, Name: target.Spec.AgentAccessSecretRef.Name}

	if err := r.Get(ctx, secretKey, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 30 * time.Second},
				r.setCondition(ctx, target, metav1.ConditionFalse, "SecretNotFound", err.Error())
		}

		return ctrl.Result{}, errLogAndWrap(log, err, "failed to get agent access secret")
	}

	kubeconfig, ok := secret.Data["kubeconfig"]
	if !ok {
		return ctrl.Result{}, r.setCondition(ctx, target, metav1.ConditionFalse, "MissingKubeconfigKey",
			fmt.Sprintf("secret %s has no %q key", secret.Name, "kubeconfig"))
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return ctrl.Result{}, r.setCondition(ctx, target, metav1.ConditionFalse, "InvalidKubeconfig", err.Error())
	}

	if err := r.Installer.Install(ctx, restConfig, target); err != nil {
		log.Error(err, "installing solar-agent")

		return ctrl.Result{RequeueAfter: 30 * time.Second},
			r.setCondition(ctx, target, metav1.ConditionFalse, "InstallFailed", err.Error())
	}

	return ctrl.Result{}, r.setCondition(ctx, target, metav1.ConditionTrue, "Installed", "solar-agent installed")
}

func (r *TargetAgentInstallerReconciler) setCondition(ctx context.Context, target *solarv1alpha1.Target, status metav1.ConditionStatus, reason, message string) error {
	changed := apimeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeAgentInstalled,
		Status:             status,
		ObservedGeneration: target.Generation,
		Reason:             reason,
		Message:            message,
	})
	if changed {
		if err := r.Status().Update(ctx, target); err != nil {
			return fmt.Errorf("failed to update target agent-installed condition: %w", err)
		}
	}

	return nil
}

func (r *TargetAgentInstallerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Target{}).
		Named("target-agent-installer").
		Complete(r)
}
