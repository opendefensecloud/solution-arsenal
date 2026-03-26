// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	annotationJobName    = "solar.opendefense.cloud/job-name"
	annotationSecretName = "solar.opendefense.cloud/secret-name"

	// Condition types
	ConditionTypeJobScheduled = "JobScheduled"
	ConditionTypeJobSucceeded = "JobSucceeded"
	ConditionTypeJobFailed    = "JobFailed"

	ConditionTypeTaskCompleted = "TaskCompleted"
	ConditionTypeTaskFailed    = "TaskFailed"
)

// RenderTaskReconciler reconciles a RenderTask object
type RenderTaskReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Recorder            events.EventRecorder
	RendererImage       string
	RendererCommand     string
	RendererArgs        []string
	PushSecretRef       *corev1.SecretReference
	BaseURL             string
	RendererCAConfigMap string
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	// See: https://book.kubebuilder.io/reference/envtest#testing-considerations
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile moves the current state of the cluster closer to the desired state
func (r *RenderTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("RenderTask is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrlResult, nil
	}

	// Fetch the RenderTask instance
	res := &solarv1alpha1.RenderTask{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrlResult, nil
		}

		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// RenderTask instance marked for deletion, stop reconciling
	if !res.DeletionTimestamp.IsZero() {
		log.V(1).Info("RenderTask is being deleted")
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "Deleting", "Delete", "RenderTask is being deleted, cleaning up secret and job")

		return ctrlResult, nil
	}

	// Check if renderjob has already completed successfully
	sc := apimeta.FindStatusCondition(res.Status.Conditions, ConditionTypeJobSucceeded)
	if sc != nil && sc.ObservedGeneration >= res.Generation && sc.Status == metav1.ConditionTrue {
		log.V(1).Info("RenderTask has already completed successfully, no further action needed")
		return ctrlResult, nil
	}

	// Reconcile Config Secret
	configSecret := &corev1.Secret{}
	err := r.Get(ctx, r.configSecretKey(res), configSecret)
	if err != nil && apierrors.IsNotFound(err) {
		createdSecret, err := r.createConfigSecret(ctx, res)
		if err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "CreateSecretFailed", "CreateConfigSecret", fmt.Sprintf("Failed to create config secret: %s", err))
			return ctrlResult, errLogAndWrap(log, err, "failed to create secret")
		}
		configSecret = createdSecret
	} else if err != nil {
		return ctrlResult, errLogAndWrap(log, err, "could not get secret")
	}

	// Reconcile Auth Secret
	authSecret := &corev1.Secret{}
	err = r.Get(ctx, r.authSecretKey(res), authSecret)
	if err != nil && apierrors.IsNotFound(err) && r.PushSecretRef != nil {
		createdSecret, err := r.copyAuthSecret(ctx, res)
		if err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "CreateSecretFailed", "CreateAuthSecret", fmt.Sprintf("Failed to create auth secret: %s", err))
			return ctrlResult, errLogAndWrap(log, err, "failed to copy auth secret to namespace")
		}
		authSecret = createdSecret
	} else if client.IgnoreNotFound(err) != nil {
		return ctrlResult, errLogAndWrap(log, err, "could not get auth secret")
	}

	// Reconcile Job
	job := &batchv1.Job{}
	err = r.Get(ctx, r.renderJobKey(res), job)
	if err != nil && apierrors.IsNotFound(err) {
		err := r.createRenderJob(ctx, res, configSecret, authSecret)
		if err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "CreateJobFailed", "CreateJob", fmt.Sprintf("Failed to create job: %s", err))
			return ctrlResult, errLogAndWrap(log, err, "failed to create job")
		}
	} else if err != nil {
		return ctrlResult, errLogAndWrap(log, err, "could not get job")
	}

	// Update Status
	if changed := r.updateResourceStatusFromJob(ctx, res, job); changed {
		if err := r.Status().Update(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to update status")
		}
	}

	ttlDuration := time.Duration(ttlSeconds(res.Spec.FailedJobTTL)) * time.Second

	switch {
	case job.Status.Succeeded > 0:
		cleanupRenderResources(ctx, r, res, job)
		log.V(1).Info("Cleaned up after successful job")

		return ctrlResult, nil

	case job.Status.Failed > 0:
		if shouldCleanupSecrets(res, ttlDuration) {
			cleanupSecrets(ctx, r, res)
			log.V(1).Info("Cleaned up secrets after failed job TTL")

			return ctrlResult, nil
		}
		remaining := remainingTTL(res, ttlDuration)
		log.V(1).Info("Waiting for TTL to expire before cleaning up secrets", "remainingSeconds", remaining.Seconds())

		return ctrl.Result{RequeueAfter: remaining + time.Second}, nil
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

		if res.Status.ChartURL != r.referenceURL(res.Spec.Repository, res.Spec.Tag) {
			res.Status.ChartURL = r.referenceURL(res.Spec.Repository, res.Spec.Tag)
			changed = true
		}

		r.Recorder.Eventf(res, job, corev1.EventTypeNormal, "JobSucceeded", "RunJob", "Renderer job completed successfully")
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
		r.Recorder.Eventf(res, job, corev1.EventTypeWarning, "JobFailed", "RunJob", "Renderer job failed")
		log.V(1).Info("Job failed", "name", job.Name)

		return changed
	}

	return apimeta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeJobScheduled,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: res.Generation,
		Reason:             "JobScheduled",
		Message:            fmt.Sprintf("Renderer job is running (active: %d, succeeded: %d, failed: %d)", job.Status.Active, job.Status.Succeeded, job.Status.Failed),
	})
}

func (r *RenderTaskReconciler) deleteAuthSecret(ctx context.Context, res *solarv1alpha1.RenderTask) error {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, r.authSecretKey(res), secret); err != nil {
		return err
	}

	return r.Delete(ctx, secret, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func (r *RenderTaskReconciler) deleteRenderJob(ctx context.Context, res *solarv1alpha1.RenderTask) error {
	job := &batchv1.Job{}
	if err := r.Get(ctx, r.renderJobKey(res), job); err != nil {
		return err
	}

	return r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func (r *RenderTaskReconciler) deleteConfigSecret(ctx context.Context, res *solarv1alpha1.RenderTask) error {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, r.configSecretKey(res), secret); err != nil {
		return err
	}

	return r.Delete(ctx, secret, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func (r *RenderTaskReconciler) copyAuthSecret(ctx context.Context, res *solarv1alpha1.RenderTask) (*corev1.Secret, error) {
	log := ctrl.LoggerFrom(ctx)

	controllerSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: r.PushSecretRef.Name, Namespace: r.PushSecretRef.Namespace}, controllerSecret); err != nil {
		return nil, err
	}

	authSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.authSecretKey(res).Name,
			Namespace: r.authSecretKey(res).Namespace,
		},
		Type:       controllerSecret.Type,
		Data:       controllerSecret.Data,
		StringData: controllerSecret.StringData,
	}

	// Set owner references
	if err := controllerutil.SetControllerReference(res, authSecret, r.Scheme); err != nil {
		return nil, errLogAndWrap(log, err, "failed to set controller reference")
	}

	if err := r.Create(ctx, authSecret); err != nil {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "CreationFailed", "Create", "Failed to create secret: %s", err)
		return nil, errLogAndWrap(log, err, "secret creation failed")
	}

	return authSecret, nil
}

func (r *RenderTaskReconciler) createRenderJob(ctx context.Context, res *solarv1alpha1.RenderTask, configSecret, authSecret *corev1.Secret) error {
	log := ctrl.LoggerFrom(ctx)

	jobName := r.renderJobKey(res).Name
	backoffLimit := int32(3)
	ttlSecondsAfterFinished := int32(3600)
	if res.Spec.FailedJobTTL != nil {
		ttlSecondsAfterFinished = *res.Spec.FailedJobTTL
	}

	volumes := []corev1.Volume{
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
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/etc/renderer/config.json",
			SubPath:   "config.json",
			ReadOnly:  true,
		},
	}
	envVars := []corev1.EnvVar{
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
	}

	if r.RendererCAConfigMap != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "ca-bundle",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.RendererCAConfigMap,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "trust-bundle.pem",
							Path: "ca-bundle.pem",
						},
					},
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "ca-bundle",
			MountPath: "/etc/ssl/certs",
			ReadOnly:  true,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "SSL_CERT_FILE",
			Value: "/etc/ssl/certs/ca-bundle.pem",
		})
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.renderJobKey(res).Namespace,
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
							Args: append(r.RendererArgs,
								"/etc/renderer/config.json",
								fmt.Sprintf("--url=%s", r.referenceURL(res.Spec.Repository, res.Spec.Tag)),
							),
							Env:          envVars,
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	if authSecret != nil {
		switch authSecret.Type {
		case corev1.SecretTypeBasicAuth:
			job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env,
				corev1.EnvVar{
					Name: "REGISTRY_USERNAME",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: authSecret.Name,
							},
							Key: "username",
						},
					},
				},
				corev1.EnvVar{
					Name: "REGISTRY_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: authSecret.Name,
							},
							Key: "password",
						},
					},
				},
			)

		case corev1.SecretTypeDockerConfigJson:
			job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: "dockerconfig",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: authSecret.Name,
						Items: []corev1.KeyToPath{
							{
								Key:  ".dockerconfigjson",
								Path: "dockerconfig.json",
							},
						},
					},
				},
			})

			job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				Name:      "dockerconfig",
				MountPath: "/etc/renderer/dockerconfig.json",
				SubPath:   "dockerconfig.json",
				ReadOnly:  true,
			})

			job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
				Name:  "DOCKER_CONFIG",
				Value: "/etc/renderer/dockerconfig.json",
			})
		default:
		}
	}

	// Set owner references
	if err := controllerutil.SetControllerReference(res, job, r.Scheme); err != nil {
		return errLogAndWrap(log, err, "failed to set controller reference")
	}

	if err := r.Create(ctx, job); err != nil {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "CreationFailed", "Create", "Failed to create job: %s", err)
		return errLogAndWrap(log, err, "job creation failed")
	}

	res.Status.JobRef = &corev1.ObjectReference{
		APIVersion: batchv1.SchemeGroupVersion.String(),
		Kind:       "Job",
		Namespace:  job.Namespace,
		Name:       job.Name,
	}

	if err := r.Status().Update(ctx, res); err != nil {
		return errLogAndWrap(log, err, "failed to update status")
	}

	return nil
}

func (r *RenderTaskReconciler) createConfigSecret(ctx context.Context, res *solarv1alpha1.RenderTask) (*corev1.Secret, error) {
	log := ctrl.LoggerFrom(ctx)

	cfgJson, err := json.Marshal(res.Spec.RendererConfig)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.configSecretKey(res).Name,
			Namespace: r.configSecretKey(res).Namespace,
			Annotations: map[string]string{
				annotationSecretName: r.configSecretKey(res).Name,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"config.json": cfgJson,
		},
	}

	// Set owner references
	if err := controllerutil.SetControllerReference(res, secret, r.Scheme); err != nil {
		return nil, errLogAndWrap(log, err, "failed to set controller reference")
	}

	if err := r.Create(ctx, secret); err != nil {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "CreationFailed", "Create", "Failed to create secret: %s", err)
		return nil, errLogAndWrap(log, err, "secret creation failed")
	}

	res.Status.ConfigSecretRef = &corev1.ObjectReference{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "Secret",
		Namespace:  secret.Namespace,
		Name:       secret.Name,
	}

	if err := r.Status().Update(ctx, res); err != nil {
		return nil, errLogAndWrap(log, err, "failed to update status")
	}

	return secret, nil
}

func (r *RenderTaskReconciler) configSecretKey(res *solarv1alpha1.RenderTask) client.ObjectKey {
	return client.ObjectKey{
		Name:      fmt.Sprintf("render-%s", res.Name),
		Namespace: res.Namespace,
	}
}

func (r *RenderTaskReconciler) authSecretKey(res *solarv1alpha1.RenderTask) client.ObjectKey {
	return client.ObjectKey{
		Name:      fmt.Sprintf("auth-%s", res.Name),
		Namespace: res.Namespace,
	}
}

func (r *RenderTaskReconciler) renderJobKey(res *solarv1alpha1.RenderTask) client.ObjectKey {
	return client.ObjectKey{
		Name:      fmt.Sprintf("render-%s", res.Name),
		Namespace: res.Namespace,
	}
}

func (r *RenderTaskReconciler) referenceURL(repo string, tag string) string {
	base := r.BaseURL
	if !strings.HasPrefix(base, "oci://") {
		base = fmt.Sprintf("oci://%s", base)
	}
	base = strings.TrimSuffix(base, "/")
	url := fmt.Sprintf("%s/%s:%s", base, repo, tag)

	return url
}

func ttlSeconds(ttl *int32) int32 {
	if ttl != nil {
		return *ttl
	}

	return 3600
}

func shouldCleanupSecrets(res *solarv1alpha1.RenderTask, ttl time.Duration) bool {
	cond := apimeta.FindStatusCondition(res.Status.Conditions, ConditionTypeJobFailed)
	return cond != nil && time.Since(cond.LastTransitionTime.Time) >= ttl
}

func remainingTTL(res *solarv1alpha1.RenderTask, ttl time.Duration) time.Duration {
	cond := apimeta.FindStatusCondition(res.Status.Conditions, ConditionTypeJobFailed)
	if cond == nil {
		return ttl
	}
	remaining := ttl - time.Since(cond.LastTransitionTime.Time)
	if remaining < 0 {
		return 0
	}

	return remaining
}

func cleanupSecrets(ctx context.Context, r *RenderTaskReconciler, res *solarv1alpha1.RenderTask) {
	if err := r.deleteConfigSecret(ctx, res); err != nil && !apierrors.IsNotFound(err) {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "DeletionFailed", "Delete", "Failed to delete config secret", err)
	}
	if err := r.deleteAuthSecret(ctx, res); err != nil && !apierrors.IsNotFound(err) {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "DeletionFailed", "Delete", "Failed to delete auth secret", err)
	}
}

func cleanupRenderResources(ctx context.Context, r *RenderTaskReconciler, res *solarv1alpha1.RenderTask, job *batchv1.Job) {
	cleanupSecrets(ctx, r, res)
	if err := r.deleteRenderJob(ctx, res); err != nil && !apierrors.IsNotFound(err) {
		r.Recorder.Eventf(res, job, corev1.EventTypeWarning, "DeletionFailed", "Delete", "Failed to delete job", err)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RenderTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.RenderTask{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
