// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/renderer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReleaseReconciler reconciles a Release object
type ReleaseReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	RendererImage   string
	RendererCommand string
	RendererArgs    []string
	PushOptions     renderer.PushOptions
}

// Ensure ReleaseReconciler implements ConfigBuilder
var _ ConfigBuilder = (*ReleaseReconciler)(nil)

// Implement ConfigBuilder interface
func (r *ReleaseReconciler) BuildConfig(ctx context.Context, log logr.Logger, obj RenderJobObject) ([]byte, error) {
	adapter := obj.(*releaseAdapter)
	rel := adapter.Release

	cvRef := types.NamespacedName{
		Name:      rel.Spec.ComponentVersionRef.Name,
		Namespace: rel.Namespace,
	}
	cv := &solarv1alpha1.ComponentVersion{}

	if err := r.Get(ctx, cvRef, cv); err != nil {
		return nil, err
	}

	// Build the renderer configuration
	cfg := renderer.Config{
		Type: renderer.TypeRelease,
		ReleaseConfig: renderer.ReleaseConfig{
			Chart: renderer.ChartConfig{
				Name:        rel.Name,
				Description: fmt.Sprintf("Release of %s", rel.Spec.ComponentVersionRef.Name),
				Version:     cv.Spec.Tag,
				AppVersion:  cv.Spec.Tag,
			},
			Input: renderer.ReleaseInput{
				Component: renderer.ReleaseComponent{Name: cv.Spec.ComponentRef.Name},
				Helm:      cv.Spec.Helm,
				KRO:       cv.Spec.KRO,
				Resources: cv.Spec.Resources,
			},
			Values: rel.Spec.Values.Raw,
		},
		PushOptions: r.PushOptions,
	}

	// Marshal config to JSON
	return json.Marshal(cfg)
}

func (r *ReleaseReconciler) GetRecorder() record.EventRecorder {
	return r.Recorder
}

func (r *ReleaseReconciler) GetRendererImage() string {
	return r.RendererImage
}

func (r *ReleaseReconciler) GetRendererCommand() string {
	return r.RendererCommand
}

func (r *ReleaseReconciler) GetRendererArgs() []string {
	return r.RendererArgs
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile moves the current state of the cluster closer to the desired state
//
// Reconciliation Flow:
//
//	Release created
//	    ↓
//	Add finalizer
//	    ↓
//	Check if already succeeded → YES → Return (no-op)
//	    ↓ NO
//	Create/update config secret
//	    ↓
//	Get or create job
//	    ↓
//	Update release status from job
//	    ↓
//	Job completed with success?
//	    ├→ YES → Cleanup resources → Return
//	    └→ NO → Still running? → Requeue in 5s
func (r *ReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("Release is being reconciled", "req", req)

	// Fetch the Release instance
	res := &solarv1alpha1.Release{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrlResult, nil
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Create helper with Release-specific config builder
	helper := &RenderJobHelper{
		Client:        r.Client,
		Scheme:        r.Scheme,
		ConfigBuilder: r,
	}

	// Create an adapter that wraps Release as RenderJobObject
	adapter := &releaseAdapter{Release: res}

	// Handle deletion: cleanup job and secret, then remove finalizer
	if !res.DeletionTimestamp.IsZero() {
		log.V(1).Info("Release is being deleted")
		r.Recorder.Event(res, corev1.EventTypeWarning, "Deleting", "Release is being deleted, cleaning up resources")

		if err := helper.CleanupResources(ctx, log, adapter); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to cleanup resources")
		}

		if err := helper.RemoveFinalizer(ctx, log, adapter, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer")
		}
		return ctrlResult, nil
	}

	// Add finalizer if not present
	added, err := helper.EnsureFinalizer(ctx, log, adapter, res)
	if err != nil {
		return ctrlResult, errLogAndWrap(log, err, "failed to ensure finalizer")
	}
	if added {
		// Return without requeue; the Update event will trigger reconciliation again
		return ctrlResult, nil
	}

	// Check if release has already completed successfully
	if apimeta.IsStatusConditionTrue(res.Status.Conditions, ConditionTypeJobSucceeded) {
		log.V(1).Info("Release has already completed successfully, no further action needed")
		return ctrlResult, nil
	}

	// Create or update configuration secret
	configSecret, err := helper.CreateOrUpdateConfigSecret(ctx, log, adapter)
	if err != nil {
		r.Recorder.Event(res, corev1.EventTypeWarning, "ConfigFailed", fmt.Sprintf("Failed to create config secret: %v", err))
		if changed := apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeJobScheduled,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: res.Generation,
			Reason:             "ConfigSecretFailed",
			Message:            fmt.Sprintf("Failed to create config secret: %v", err),
		}); changed {
			if err := r.Status().Update(ctx, res); err != nil {
				log.Error(err, "failed to update Release status")
			}
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to create config secret")
	}

	res.Status.ConfigSecretRef = &corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Secret",
		Name:       configSecret.Name,
		Namespace:  configSecret.Namespace,
		UID:        configSecret.UID,
	}

	// Get or create the job
	job, err := helper.GetOrCreateJob(ctx, log, adapter, configSecret)
	if err != nil {
		r.Recorder.Event(res, corev1.EventTypeWarning, "JobFailed", fmt.Sprintf("Failed to create or get job: %v", err))
		if changed := apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeJobScheduled,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: res.Generation,
			Reason:             "JobCreationFailed",
			Message:            fmt.Sprintf("Failed to create job: %v", err),
		}); changed {
			if err := r.Status().Update(ctx, res); err != nil {
				log.Error(err, "failed to update Release status")
			}
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to create job")
	}

	if job != nil {
		res.Status.JobRef = &corev1.ObjectReference{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       job.Name,
			Namespace:  job.Namespace,
			UID:        job.UID,
		}

		// Check job status and update status if required
		if changed := helper.UpdateResourceStatusFromJob(ctx, log, adapter, job); changed {
			if err := r.Status().Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to update Release status")
			}
		}

		// Check if job completed successfully
		if IsJobComplete(job) && job.Status.Succeeded > 0 {
			log.V(1).Info("Job completed successfully, cleaning up job and secret")
			if err := helper.CleanupResources(ctx, log, adapter); err != nil {
				log.Error(err, "failed to cleanup resources after successful job completion")
				// Don't fail reconciliation, job is already successful
			}
			return ctrlResult, nil
		}

		// Check if job is still running
		if !IsJobComplete(job) {
			log.V(1).Info("Job is still running, requeue after 5 seconds")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	return ctrlResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Release{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

// Adapter to use Release directly as RenderJobObject while delegating to client.Update
type releaseAdapter struct {
	*solarv1alpha1.Release
}

func (a *releaseAdapter) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *releaseAdapter) SetConditions(conditions []metav1.Condition) {
	a.Status.Conditions = conditions
}

func (a *releaseAdapter) GetResourceName() string {
	return a.Name
}

func (a *releaseAdapter) GetObjectReference() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: solarv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Release",
		Name:       a.Name,
		Namespace:  a.Namespace,
		UID:        a.UID,
	}
}

func (a *releaseAdapter) SetJobRef(ref *corev1.ObjectReference) {
	a.Status.JobRef = ref
}

func (a *releaseAdapter) SetConfigSecretRef(ref *corev1.ObjectReference) {
	a.Status.ConfigSecretRef = ref
}

func (a *releaseAdapter) RuntimeObject() runtime.Object {
	return a.Release
}
