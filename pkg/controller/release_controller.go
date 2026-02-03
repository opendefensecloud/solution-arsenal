// Copyright 2026 BWI GmbH and Artefact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	releaseFinalizer = "solar.opendefense.cloud/release-finalizer"

	// Condition types
	ConditionTypeJobScheduled = "JobScheduled"
	ConditionTypeJobSucceeded = "JobSucceeded"
	ConditionTypeJobFailed    = "JobFailed"

	// Annotation for tracking job/secret ownership
	AnnotationJobName    = "solar.opendefense.cloud/job-name"
	AnnotationSecretName = "solar.opendefense.cloud/config-secret-name"
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

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/finalizers,verbs=update
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

	// Handle deletion: cleanup job and secret, then remove finalizer
	if !res.DeletionTimestamp.IsZero() {
		log.V(1).Info("Release is being deleted")
		r.Recorder.Event(res, corev1.EventTypeWarning, "Deleting", "Release is being deleted, cleaning up resources")

		if err := r.cleanupResources(ctx, log, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to cleanup resources")
		}

		// Remove finalizer
		if slices.Contains(res.Finalizers, releaseFinalizer) {
			log.V(1).Info("Removing finalizer from resource")
			res.Finalizers = slices.DeleteFunc(res.Finalizers, func(f string) bool {
				return f == releaseFinalizer
			})
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer")
			}
		}
		return ctrlResult, nil
	}

	// Add finalizer if not present
	if !slices.Contains(res.Finalizers, releaseFinalizer) {
		log.V(1).Info("Adding finalizer to resource")
		res.Finalizers = append(res.Finalizers, releaseFinalizer)
		if err := r.Update(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer")
		}
		// Return without requeue; the Update event will trigger reconciliation again
		return ctrlResult, nil
	}

	// Check if release has already completed successfully
	if apimeta.IsStatusConditionTrue(res.Status.Conditions, ConditionTypeJobSucceeded) {
		log.V(1).Info("Release has already completed successfully, no further action needed")
		return ctrlResult, nil
	}

	// Create or update configuration secret
	configSecret, err := r.createOrUpdateConfigSecret(ctx, log, res)
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
	job, err := r.getOrCreateJob(ctx, log, res, configSecret)
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
		if changed := r.updateReleaseStatusFromJob(ctx, log, res, job); changed {
			if err := r.Status().Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to update Release status")
			}
		}

		// Check if job completed successfully
		if isJobComplete(job) && job.Status.Succeeded > 0 {
			log.V(1).Info("Job completed successfully, cleaning up job and secret")
			if err := r.cleanupResources(ctx, log, res); err != nil {
				log.Error(err, "failed to cleanup resources after successful job completion")
				// Don't fail reconciliation, job is already successful
			}
			return ctrlResult, nil
		}

		// Check if job is still running
		if !isJobComplete(job) {
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

// createOrUpdateConfigSecret creates or updates a secret containing the renderer configuration
func (r *ReleaseReconciler) createOrUpdateConfigSecret(ctx context.Context, log logr.Logger, rel *solarv1alpha1.Release) (*corev1.Secret, error) {
	// Build the renderer configuration
	cfg := renderer.Config{
		Type: "release",
		ReleaseConfig: renderer.ReleaseConfig{
			Chart: renderer.ChartConfig{
				Name:        rel.Name,
				Description: fmt.Sprintf("Release of %s", rel.Spec.ComponentVersionRef.Name),
				Version:     "1.0.0", // TODO: derive from component version
				AppVersion:  "1.0.0", // TODO: derive from component version
			},
			Input: renderer.ReleaseInput{
				Component: renderer.ReleaseComponent{}, // TODO: populate from component version
				Helm:      renderer.ResourceAccess{},   // TODO: populate from component version
				KRO:       renderer.ResourceAccess{},   // TODO: populate from component version
				Resources: make(map[string]renderer.ResourceAccess),
			},
			Values: rel.Spec.Values.Raw,
		},
		PushOptions: r.PushOptions,
	}

	// Marshal config to JSON
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal renderer config: %w", err)
	}

	// Create or get secret name
	secretName := fmt.Sprintf("%s-config", rel.Name)

	// Create or update the secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: rel.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(rel, solarv1alpha1.SchemeGroupVersion.WithKind("Release")),
			},
			Annotations: map[string]string{
				AnnotationSecretName: secretName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"config.json": configJSON,
		},
	}

	// Try to get existing secret
	existingSecret := &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: rel.Namespace}, existingSecret)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get existing secret: %w", err)
	}

	if err == nil {
		// Update existing secret
		existingSecret.Data = secret.Data
		if err := r.Update(ctx, existingSecret); err != nil {
			return nil, fmt.Errorf("failed to update secret: %w", err)
		}
		return existingSecret, nil
	}

	// Create new secret
	if err := r.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	return secret, nil
}

// getOrCreateJob creates or gets the renderer job
func (r *ReleaseReconciler) getOrCreateJob(ctx context.Context, log logr.Logger, rel *solarv1alpha1.Release, configSecret *corev1.Secret) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-renderer", rel.Name)

	// Try to get existing job
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: rel.Namespace}, job)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get existing job: %w", err)
	}

	if err == nil {
		// Job already exists
		log.V(1).Info("Job already exists", "job", jobName)
		return job, nil
	}

	// Create new job
	backoffLimit := int32(3)
	ttlSecondsAfterFinished := int32(3600) // Clean up after 1 hour

	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: rel.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(rel, solarv1alpha1.SchemeGroupVersion.WithKind("Release")),
			},
			Annotations: map[string]string{
				AnnotationJobName: jobName,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "renderer",
							Image:   r.RendererImage,
							Command: []string{r.RendererCommand},
							Args:    append(r.RendererArgs, "/etc/renderer/config.json"),
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/etc/renderer",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: configSecret.Name,
									Items: []corev1.KeyToPath{
										{
											Key:  "config.json",
											Path: "config.json",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := r.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	log.V(1).Info("Created new job", "job", jobName)
	r.Recorder.Event(rel, corev1.EventTypeNormal, "JobScheduled", fmt.Sprintf("Scheduled renderer job: %s", jobName))

	return job, nil
}

// updateReleaseStatusFromJob updates the Release status based on job status
func (r *ReleaseReconciler) updateReleaseStatusFromJob(ctx context.Context, log logr.Logger, rel *solarv1alpha1.Release, job *batchv1.Job) bool {
	if job.Status.Succeeded > 0 {
		changed := apimeta.SetStatusCondition(&rel.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeJobSucceeded,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: rel.Generation,
			Reason:             "JobSucceeded",
			Message:            fmt.Sprintf("Renderer job completed successfully at %v", job.Status.CompletionTime),
		})
		r.Recorder.Event(rel, corev1.EventTypeNormal, "JobSucceeded", "Renderer job completed successfully")
		return changed
	}

	if job.Status.Failed > 0 {
		changed := apimeta.SetStatusCondition(&rel.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeJobFailed,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: rel.Generation,
			Reason:             "JobFailed",
			Message:            "Renderer job failed",
		})
		r.Recorder.Event(rel, corev1.EventTypeWarning, "JobFailed", "Renderer job failed")
		return changed
	}

	// Job is still running
	changed := apimeta.SetStatusCondition(&rel.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeJobScheduled,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: rel.Generation,
		Reason:             "JobScheduled",
		Message:            fmt.Sprintf("Renderer job is running (active: %d, succeeded: %d, failed: %d)", job.Status.Active, job.Status.Succeeded, job.Status.Failed),
	})

	return changed
}

// cleanupResources deletes the job and secret associated with a Release
func (r *ReleaseReconciler) cleanupResources(ctx context.Context, log logr.Logger, rel *solarv1alpha1.Release) error {
	jobName := fmt.Sprintf("%s-renderer", rel.Name)
	job := &batchv1.Job{}
	if err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: rel.Namespace}, job); err == nil {
		if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			log.Error(err, "failed to delete job", "job", jobName)
			return fmt.Errorf("failed to delete job: %w", err)
		}
		log.V(1).Info("Deleted job", "job", jobName)
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get job: %w", err)
	}

	secretName := fmt.Sprintf("%s-config", rel.Name)
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: rel.Namespace}, secret); err == nil {
		if err := r.Delete(ctx, secret); err != nil {
			log.Error(err, "failed to delete secret", "secret", secretName)
			return fmt.Errorf("failed to delete secret: %w", err)
		}
		log.V(1).Info("Deleted secret", "secret", secretName)
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	return nil
}

// isJobComplete checks if a job has completed (either succeeded or failed)
func isJobComplete(job *batchv1.Job) bool {
	return job.Status.CompletionTime != nil
}
