// Copyright 2026 BWI GmbH and Artefact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"slices"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	discoveryFinalizer = "solar.opendefense.cloud/discovery-finalizer"
)

// DiscoveryReconciler reconciles a Discovery object
type DiscoveryReconciler struct {
	client.Client
	ClientSet     kubernetes.Interface
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	WorkerImage   string
	WorkerCommand []string
	WorkerArgs    []string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=discoveries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=discoveries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=discoveries/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile moves the current state of the cluster closer to the desired state
func (r *DiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	// Fetch the Order instance
	res := &solarv1alpha1.Discovery{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrlResult, nil
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Handle deletion: cleanup artifact workflows, then remove finalizer
	if !res.DeletionTimestamp.IsZero() {
		log.V(1).Info("Discovery is being deleted")
		r.Recorder.Event(res, corev1.EventTypeWarning, "Deleting", "Discovery is being deleted, cleaning up worker")

		// Cleanup worker, if exists
		if err := r.Delete(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: res.Namespace, Name: res.Name}}); err != nil && !apierrors.IsNotFound(err) {
			return ctrlResult, errLogAndWrap(log, err, "pod deletion failed")
		}

		// Remove finalizer
		if slices.Contains(res.Finalizers, discoveryFinalizer) {
			log.V(1).Info("Removing finalizer from resource")
			res.Finalizers = slices.DeleteFunc(res.Finalizers, func(f string) bool {
				return f == discoveryFinalizer
			})
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer")
			}
		}
	}

	// Add finalizer if not present and not deleting
	if res.DeletionTimestamp.IsZero() {
		if !slices.Contains(res.Finalizers, discoveryFinalizer) {
			log.V(1).Info("Adding finalizer to resource")
			res.Finalizers = append(res.Finalizers, discoveryFinalizer)
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer")
			}
			// Return without requeue; the Update event will trigger reconciliation again
			return ctrlResult, nil
		}
	}

	pod, err := r.ClientSet.CoreV1().Pods(res.Namespace).Get(ctx, discoveryPrefixed(res.Name), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		r.Recorder.Eventf(res, corev1.EventTypeWarning, "Reconcile", "Failed to get pod", err)
		return ctrlResult, errLogAndWrap(log, err, "failed to get pod information")
	}

	// No pod yet, create it.
	if pod == nil || pod.Name == "" {
		if err := r.createPod(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to create pod")
		}
		return ctrlResult, nil
	}

	// Pod exists, check if it's up to date with our configuration and if it is healthy.
	if res.Status.PodGeneration != res.GetGeneration() {
		// Recreate pod, configuration mismatch
		r.Recorder.Eventf(res, corev1.EventTypeNormal, "Reconcile", "Configuration changed. Replacing pod.")
		if err := r.ClientSet.CoreV1().Secrets(res.Namespace).Delete(ctx, discoveryPrefixed(res.Name), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			r.Recorder.Eventf(res, corev1.EventTypeWarning, "DeletionFailed", "Failed to delete secret", err)
			return ctrlResult, errLogAndWrap(log, err, "secret deletion failed")
		}
		if err := r.ClientSet.CoreV1().Pods(res.Namespace).Delete(ctx, discoveryPrefixed(res.Name), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			r.Recorder.Eventf(res, corev1.EventTypeWarning, "DeletionFailed", "Failed to delete pod", err)
			return ctrlResult, errLogAndWrap(log, err, "pod deletion failed")
		}
		if err := r.createPod(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to create pod")
		}
		return ctrlResult, nil
	}

	// TODO: Check pods health

	return ctrlResult, nil
}

// createPod creates a new pod for the given discovery resource
func (r *DiscoveryReconciler) createPod(ctx context.Context, res *solarv1alpha1.Discovery) error {
	log := ctrl.LoggerFrom(ctx)

	// Create secret
	// TODO: Use the actual configuration file instead of a dummy one
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: res.Namespace,
			Name:      discoveryPrefixed(res.Name),
		},
		StringData: map[string]string{
			"config.yaml": "not implemented",
		},
	}
	_, err := r.ClientSet.CoreV1().Secrets(res.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		r.Recorder.Eventf(res, corev1.EventTypeWarning, "CreationFailed", "Failed to create secret", err)
		return errLogAndWrap(log, err, "failed to create secret")
	}
	r.Recorder.Eventf(res, corev1.EventTypeNormal, "PodCreate", "Secret created")

	// Set owner references
	if err := controllerutil.SetControllerReference(res, secret, r.Scheme); err != nil {
		return errLogAndWrap(log, err, "failed to set controller reference")
	}

	// Create pod
	var args []string
	args = append(args, r.WorkerArgs...)
	args = append(args, "--config", "/etc/worker/config.yaml")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        discoveryPrefixed(res.Name),
			Namespace:   res.Namespace,
			Labels:      res.Labels,
			Annotations: res.Annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "worker",
					Image:   r.WorkerImage,
					Command: r.WorkerCommand,
					Args:    args,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							ReadOnly:  true,
							MountPath: "/etc/worker"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: res.Name,
						},
					},
				},
			},
		},
	}

	// Set owner references
	if err := controllerutil.SetControllerReference(res, pod, r.Scheme); err != nil {
		return errLogAndWrap(log, err, "failed to set controller reference")
	}

	_, err = r.ClientSet.CoreV1().Pods(res.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		r.Recorder.Eventf(res, corev1.EventTypeWarning, "PodCreate", "Failed to create pod", err)
		return errLogAndWrap(log, err, "failed to create pod")
	}
	r.Recorder.Eventf(res, corev1.EventTypeNormal, "PodCreate", "Worker pod created")

	// Update discovery version in status
	res.Status.PodGeneration = res.GetGeneration()
	if err := r.Status().Update(ctx, res); err != nil {
		return errLogAndWrap(log, err, "failed to update status")
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Discovery{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func discoveryPrefixed(discoveryName string) string {
	return fmt.Sprintf("discovery-%s", discoveryName)
}
