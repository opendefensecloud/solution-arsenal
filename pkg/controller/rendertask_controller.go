// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	renderTaskFinalizer = "solar.opendefense.cloud/rendertask-finalizer"

	annotationJobName    = "solar.opendefense.cloud/job-name"
	annotationSecretName = "solar.opendefense.cloud/secret-name"

	// Condition types
	ConditionTypeJobScheduled = "JobScheduled"
	ConditionTypeJobSucceeded = "JobSucceeded"
	ConditionTypeJobFailed    = "JobFailed"
	ConditionTypeSecretSynced = "SecretSynced"

	ConditionTypeTaskCompleted = "TaskCompleted"
	ConditionTypeTaskFailed    = "TaskFailed"
)

// RenderTaskReconciler reconciles a RenderTask object
type RenderTaskReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	RendererImage   string
	RendererCommand string
	RendererArgs    []string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile moves the current state of the cluster closer to the desired state
func (r *RenderTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("RenderTask is being reconciled", "req", req)

	// Fetch the RenderTask instance
	res := &solarv1alpha1.RenderTask{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrlResult, nil
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Handle deletion: cleanup job and secret, then remove finalizer
	if !res.DeletionTimestamp.IsZero() {
		log.V(1).Info("RenderTask is being deleted")
		r.Recorder.Event(res, corev1.EventTypeWarning, "Deleting", "RenderTask is being deleted, cleaning up secret and job")

		// Cleanup render resources, if exists
		if err := r.deleteRenderJob(ctx, res); err != nil && !apierrors.IsNotFound(err) {
			return ctrlResult, errLogAndWrap(log, err, "failed to clean up render job")
		}

		if err := r.deleteRenderSecret(ctx, res); err != nil && !apierrors.IsNotFound(err) {
			return ctrlResult, errLogAndWrap(log, err, "failed to clean up render job")
		}

		// Remove finalizer
		if slices.Contains(res.Finalizers, renderTaskFinalizer) {
			log.V(1).Info("Removing finalizer from resource")
			res.Finalizers = slices.DeleteFunc(res.Finalizers, func(f string) bool {
				return f == renderTaskFinalizer
			})
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer")
			}
		}
		return ctrlResult, nil
	}

	// Add finalizer if not present and not deleting
	if res.DeletionTimestamp.IsZero() {
		if !slices.Contains(res.Finalizers, renderTaskFinalizer) {
			log.V(1).Info("Adding finalizer to resource")
			res.Finalizers = append(res.Finalizers, renderTaskFinalizer)
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer")
			}
			// Return without requeue; the Update event will trigger reconciliation again
			return ctrlResult, nil
		}
	}

	// Check if renderjob has already completed successfully
	sc := apimeta.FindStatusCondition(res.Status.Conditions, ConditionTypeJobSucceeded)
	if sc != nil && sc.ObservedGeneration == res.Generation && sc.Status == metav1.ConditionTrue {
		log.V(1).Info("RenderTask has already completed successfully, no further action needed")
		return ctrlResult, nil
	}

	// Check if renderjob has already failed
	fc := apimeta.FindStatusCondition(res.Status.Conditions, ConditionTypeJobFailed)
	if fc != nil && fc.ObservedGeneration == res.Generation && fc.Status == metav1.ConditionTrue {
		log.V(1).Info("RenderTask has already failed, no further action needed")
		return ctrlResult, nil
	}

	// Reconcile Secret
	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: renderPrefixed(res.Name), Namespace: res.Namespace}, secret)
	if err != nil && apierrors.IsNotFound(err) {
		err := r.createRenderSecret(ctx, res)
		if err != nil {
			r.Recorder.Event(res, corev1.EventTypeWarning, "CreateSecretFailed", fmt.Sprintf("Failed to create secret: %v", err))
			if changed := apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
				Type:               ConditionTypeSecretSynced,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: res.Generation,
				Reason:             "SecretCreationFailed",
				Message:            fmt.Sprintf("Failed to create secret: %v", err),
			}); changed {
				if err := r.Status().Update(ctx, res); err != nil {
					log.Error(err, "failed to update RenderTask status")
				}
			}
		}
		return ctrlResult, err
	}
	if err != nil {
		return ctrlResult, errLogAndWrap(log, err, "could not get secret")
	}

	// Reconcile Job
	job := &batchv1.Job{}
	err = r.Get(ctx, client.ObjectKey{Name: renderPrefixed(res.Name), Namespace: res.Namespace}, job)
	if err != nil && apierrors.IsNotFound(err) {
		return ctrlResult, r.createRenderJob(ctx, res, secret)
	}
	if err != nil {
		return ctrlResult, errLogAndWrap(log, err, "could not get job")
	}

	if changed := r.updateResourceStatusFromJob(ctx, res, job); changed {
		if err := r.Status().Update(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to update status")
		}
	}

	// Check if we need to clean up
	if isJobComplete(job) && job.Status.Succeeded > 0 {
		if err := r.deleteRenderJob(ctx, res); err != nil && !apierrors.IsNotFound(err) {
			r.Recorder.Eventf(res, corev1.EventTypeWarning, "DeletionFailed", "Failed to delete job", err)
			return ctrlResult, nil
		}
		if err := r.deleteRenderSecret(ctx, res); err != nil && !apierrors.IsNotFound(err) {
			r.Recorder.Eventf(res, corev1.EventTypeWarning, "DeletionFailed", "Failed to delete secret", err)
			return ctrlResult, nil
		}
		log.V(1).Info("Cleaned up after successful job")
		return ctrlResult, nil
	}

	// Check if job is still running
	if !isJobComplete(job) {
		log.V(1).Info("Job is still running, requeue after 5 seconds")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrlResult, nil
}

// updateResourceStatusFromJob updates the resource status based on job status
func (r *RenderTaskReconciler) updateResourceStatusFromJob(ctx context.Context, res *solarv1alpha1.RenderTask, job *batchv1.Job) (changed bool) {
	log := ctrl.LoggerFrom(ctx)

	if job == nil {
		changed = apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeJobScheduled,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: res.Generation,
			Reason:             "DoesNotExist",
			Message:            "Renderer job does not exist",
		})
		return changed
	}

	if job.Status.Succeeded > 0 {
		changed = apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeJobSucceeded,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: res.Generation,
			Reason:             "JobSucceeded",
			Message:            fmt.Sprintf("Renderer job completed successfully at %v", job.Status.CompletionTime),
		})

		r.Recorder.Event(res, corev1.EventTypeNormal, "JobSucceeded", "Renderer job completed successfully")
		log.V(1).Info("Job succeeded", "name", job.Name)
		return changed
	}

	if job.Status.Failed > 0 {
		changed = apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeJobFailed,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: res.Generation,
			Reason:             "JobFailed",
			Message:            "Renderer job failed",
		})
		r.Recorder.Event(res, corev1.EventTypeWarning, "JobFailed", "Renderer job failed")
		log.V(1).Info("Job failed", "name", job.Name)
		return changed
	}

	// Job is still running
	log.V(1).Info("Job is still running", "name", job.Name)
	return apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeJobScheduled,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: res.Generation,
		Reason:             "JobScheduled",
		Message:            fmt.Sprintf("Renderer job is running (active: %d, succeeded: %d, failed: %d)", job.Status.Active, job.Status.Succeeded, job.Status.Failed),
	})
}

func (r *RenderTaskReconciler) deleteRenderJob(ctx context.Context, res *solarv1alpha1.RenderTask) error {
	job := &batchv1.Job{}
	if err := r.Get(ctx, client.ObjectKey{Name: renderPrefixed(res.Name), Namespace: res.Namespace}, job); err != nil {
		return err
	}
	return r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func (r *RenderTaskReconciler) deleteRenderSecret(ctx context.Context, res *solarv1alpha1.RenderTask) error {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: renderPrefixed(res.Name), Namespace: res.Namespace}, secret); err != nil {
		return err
	}
	return r.Delete(ctx, secret, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func (r *RenderTaskReconciler) createRenderJob(ctx context.Context, res *solarv1alpha1.RenderTask, secret *corev1.Secret) error {
	log := ctrl.LoggerFrom(ctx)

	jobName := renderPrefixed(res.Name)
	backoffLimit := int32(3)
	ttlSecondsAfterFinished := int32(3600) // Clean up after 1 hour

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: res.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         solarv1alpha1.SchemeGroupVersion.String(),
					Kind:               res.Kind,
					Name:               res.Name,
					UID:                res.GetUID(),
					Controller:         boolPtr(true),
					BlockOwnerDeletion: boolPtr(true),
				},
			},
			Annotations: map[string]string{
				annotationJobName: jobName,
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
									SecretName: secret.Name,
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
		r.Recorder.Eventf(res, corev1.EventTypeWarning, "CreationFailed", "Failed to create job", err)
		return errLogAndWrap(log, err, "job creation failed")
	}

	// Set owner references
	if err := controllerutil.SetControllerReference(res, job, r.Scheme); err != nil {
		return errLogAndWrap(log, err, "failed to set controller reference")
	}

	// Set Reference in Status
	if err := r.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, job); err != nil {
		return errLogAndWrap(log, err, "could not fetch job")
	}

	res.Status.JobRef = &corev1.ObjectReference{
		APIVersion: job.APIVersion,
		Kind:       job.Kind,
		Namespace:  job.Namespace,
		Name:       job.Name,
	}

	if err := r.Status().Update(ctx, res); err != nil {
		return errLogAndWrap(log, err, "failed to update status")
	}

	return nil
}

func (r *RenderTaskReconciler) createRenderSecret(ctx context.Context, res *solarv1alpha1.RenderTask) error {
	log := ctrl.LoggerFrom(ctx)

	cfgJson, err := json.Marshal(res.Spec.RendererConfig)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      renderPrefixed(res.Name),
			Namespace: res.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         solarv1alpha1.SchemeGroupVersion.String(),
					Kind:               res.Kind,
					Name:               res.Name,
					UID:                res.UID,
					Controller:         boolPtr(true),
					BlockOwnerDeletion: boolPtr(true),
				},
			},
			Annotations: map[string]string{
				annotationSecretName: renderPrefixed(res.Name),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"config.json": cfgJson,
		},
	}

	if err := r.Create(ctx, secret); err != nil {
		r.Recorder.Eventf(res, corev1.EventTypeWarning, "CreationFailed", "Failed to create secret", err)
		return errLogAndWrap(log, err, "secret creation failed")
	}

	// Set owner references
	if err := controllerutil.SetControllerReference(res, secret, r.Scheme); err != nil {
		return errLogAndWrap(log, err, "failed to set controller reference")
	}

	// Set Reference in Status
	if err := r.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, secret); err != nil {
		return errLogAndWrap(log, err, "could not fetch job")
	}

	res.Status.ConfigSecretRef = &corev1.ObjectReference{
		APIVersion: secret.APIVersion,
		Kind:       secret.Kind,
		Namespace:  secret.Namespace,
		Name:       secret.Name,
	}

	if err := r.Status().Update(ctx, res); err != nil {
		return errLogAndWrap(log, err, "failed to update status")
	}

	return nil
}

// renderPrefixed returns the "render-" prefixed string
func renderPrefixed(r string) string {
	return fmt.Sprintf("render-%s", r)
}

// isJobComplete returns true if the Job is complete
func isJobComplete(job *batchv1.Job) bool {
	return job.Status.CompletionTime != nil
}

func boolPtr(b bool) *bool {
	return &b
}

// SetupWithManager sets up the controller with the Manager.
func (r *RenderTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.RenderTask{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
