// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Generic finalizer for render job reconciliation
	RenderJobFinalizer = "solar.opendefense.cloud/render-job-finalizer"

	// Condition types
	ConditionTypeJobScheduled = "JobScheduled"
	ConditionTypeJobSucceeded = "JobSucceeded"
	ConditionTypeJobFailed    = "JobFailed"

	// Annotation for tracking job/secret ownership
	AnnotationJobName    = "solar.opendefense.cloud/job-name"
	AnnotationSecretName = "solar.opendefense.cloud/config-secret-name"
)

// RenderJobObject is the interface that must be implemented by objects that can be reconciled
// by the RenderJobHelper (e.g., Release, HydratedTarget)
type RenderJobObject interface {
	client.Object
	GetConditions() []metav1.Condition
	SetConditions(conditions []metav1.Condition)
	GetGeneration() int64
	GetResourceName() string                     // Name for the job/secret (e.g., "release-name" or "target-name")
	GetNamespace() string                        // Kubernetes namespace
	GetObjectReference() *corev1.ObjectReference // For owner references
	SetJobRef(ref *corev1.ObjectReference)
	SetConfigSecretRef(ref *corev1.ObjectReference)
	// RuntimeObject returns the underlying runtime.Object for event recording
	RuntimeObject() runtime.Object
}

// ConfigBuilder is the interface that concrete reconcilers must implement to provide
// configuration specific to their resource type
type ConfigBuilder interface {
	// BuildConfig builds the configuration data (as bytes) for the render job
	// The returned data will be stored in a Secret at data["config.json"]
	BuildConfig(ctx context.Context, log logr.Logger, obj RenderJobObject) ([]byte, error)
	// GetRecorder returns the event recorder for this reconciler
	GetRecorder() record.EventRecorder
	// GetRendererImage returns the renderer container image
	GetRendererImage() string
	// GetRendererCommand returns the renderer container command
	GetRendererCommand() string
	// GetRendererArgs returns the renderer container arguments (excluding config file path)
	GetRendererArgs() []string
}

// RenderJobHelper provides shared functionality for reconciling render jobs across different resource types
type RenderJobHelper struct {
	Client        client.Client
	Scheme        *runtime.Scheme
	ConfigBuilder ConfigBuilder
}

// EnsureFinalizer adds the render job finalizer if not present
// underlying should be the actual registered resource type (e.g., *Release, *HydratedTarget)
func (h *RenderJobHelper) EnsureFinalizer(ctx context.Context, log logr.Logger, obj RenderJobObject, underlying client.Object) (bool, error) {
	if slices.Contains(obj.GetFinalizers(), RenderJobFinalizer) {
		return false, nil
	}

	log.V(1).Info("Adding finalizer to resource")
	finalizers := obj.GetFinalizers()
	finalizers = append(finalizers, RenderJobFinalizer)
	obj.SetFinalizers(finalizers)
	if err := h.Client.Update(ctx, underlying); err != nil {
		return false, fmt.Errorf("failed to add finalizer: %w", err)
	}
	return true, nil
}

// RemoveFinalizer removes the render job finalizer if present
// underlying should be the actual registered resource type (e.g., *Release, *HydratedTarget)
func (h *RenderJobHelper) RemoveFinalizer(ctx context.Context, log logr.Logger, obj RenderJobObject, underlying client.Object) error {
	if !slices.Contains(obj.GetFinalizers(), RenderJobFinalizer) {
		return nil
	}

	log.V(1).Info("Removing finalizer from resource")
	finalizers := obj.GetFinalizers()
	finalizers = slices.DeleteFunc(finalizers, func(f string) bool {
		return f == RenderJobFinalizer
	})
	obj.SetFinalizers(finalizers)
	if err := h.Client.Update(ctx, underlying); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}
	return nil
}

// CreateOrUpdateConfigSecret creates or updates a secret containing the render configuration
func (h *RenderJobHelper) CreateOrUpdateConfigSecret(ctx context.Context, log logr.Logger, obj RenderJobObject) (*corev1.Secret, error) {
	// Create or get secret name
	secretName := fmt.Sprintf("%s-config", obj.GetResourceName())

	// Try to get existing secret
	existingSecret := &corev1.Secret{}
	err := h.Client.Get(ctx, client.ObjectKey{Name: secretName, Namespace: obj.GetNamespace()}, existingSecret)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get existing secret: %w", err)
	}

	if err == nil {
		return existingSecret, nil
	}

	// NOTE: We return the existing secret early without updating it.
	//       Computing the configuration is expensive and unnecessary after the job was scheduled.

	// Build the configuration using the ConfigBuilder
	configData, err := h.ConfigBuilder.BuildConfig(ctx, log, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to build configuration: %w", err)
	}

	// Create the secret object with owner reference
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: obj.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         solarv1alpha1.SchemeGroupVersion.String(),
					Kind:               getObjectKind(obj),
					Name:               obj.GetName(),
					UID:                obj.GetUID(),
					Controller:         boolPtr(true),
					BlockOwnerDeletion: boolPtr(true),
				},
			},
			Annotations: map[string]string{
				AnnotationSecretName: secretName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"config.json": configData,
		},
	}

	// Create new secret
	if err := h.Client.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	return secret, nil
}

// GetOrCreateJob creates or gets the render job
func (h *RenderJobHelper) GetOrCreateJob(ctx context.Context, log logr.Logger, obj RenderJobObject, configSecret *corev1.Secret) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-renderer", obj.GetResourceName())

	// Try to get existing job
	job := &batchv1.Job{}
	err := h.Client.Get(ctx, client.ObjectKey{Name: jobName, Namespace: obj.GetNamespace()}, job)
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
			Namespace: obj.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         solarv1alpha1.SchemeGroupVersion.String(),
					Kind:               getObjectKind(obj),
					Name:               obj.GetName(),
					UID:                obj.GetUID(),
					Controller:         boolPtr(true),
					BlockOwnerDeletion: boolPtr(true),
				},
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
							Image:   h.ConfigBuilder.GetRendererImage(),
							Command: []string{h.ConfigBuilder.GetRendererCommand()},
							Args:    append(h.ConfigBuilder.GetRendererArgs(), "/etc/renderer/config.json"),
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

	if err := h.Client.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	log.V(1).Info("Created new job", "job", jobName)
	h.ConfigBuilder.GetRecorder().Event(obj.RuntimeObject(), corev1.EventTypeNormal, "JobScheduled", fmt.Sprintf("Scheduled renderer job: %s", jobName))

	return job, nil
}

// UpdateResourceStatusFromJob updates the resource status based on job status
func (h *RenderJobHelper) UpdateResourceStatusFromJob(ctx context.Context, log logr.Logger, obj RenderJobObject, job *batchv1.Job) bool {
	conditions := obj.GetConditions()
	changed := false

	if job.Status.Succeeded > 0 {
		changed = apimeta.SetStatusCondition(&conditions, metav1.Condition{
			Type:               ConditionTypeJobSucceeded,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: obj.GetGeneration(),
			Reason:             "JobSucceeded",
			Message:            fmt.Sprintf("Renderer job completed successfully at %v", job.Status.CompletionTime),
		})
		obj.SetConditions(conditions)
		h.ConfigBuilder.GetRecorder().Event(obj.RuntimeObject(), corev1.EventTypeNormal, "JobSucceeded", "Renderer job completed successfully")
		return changed
	}

	if job.Status.Failed > 0 {
		changed = apimeta.SetStatusCondition(&conditions, metav1.Condition{
			Type:               ConditionTypeJobFailed,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: obj.GetGeneration(),
			Reason:             "JobFailed",
			Message:            "Renderer job failed",
		})
		obj.SetConditions(conditions)
		h.ConfigBuilder.GetRecorder().Event(obj.RuntimeObject(), corev1.EventTypeWarning, "JobFailed", "Renderer job failed")
		return changed
	}

	// Job is still running
	changed = apimeta.SetStatusCondition(&conditions, metav1.Condition{
		Type:               ConditionTypeJobScheduled,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: obj.GetGeneration(),
		Reason:             "JobScheduled",
		Message:            fmt.Sprintf("Renderer job is running (active: %d, succeeded: %d, failed: %d)", job.Status.Active, job.Status.Succeeded, job.Status.Failed),
	})
	obj.SetConditions(conditions)

	return changed
}

// CleanupResources deletes the job and secret associated with a resource
func (h *RenderJobHelper) CleanupResources(ctx context.Context, log logr.Logger, obj RenderJobObject) error {
	jobName := fmt.Sprintf("%s-renderer", obj.GetResourceName())
	job := &batchv1.Job{}
	if err := h.Client.Get(ctx, client.ObjectKey{Name: jobName, Namespace: obj.GetNamespace()}, job); err == nil {
		if err := h.Client.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			log.Error(err, "failed to delete job", "job", jobName)
			return fmt.Errorf("failed to delete job: %w", err)
		}
		log.V(1).Info("Deleted job", "job", jobName)
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get job: %w", err)
	}

	secretName := fmt.Sprintf("%s-config", obj.GetResourceName())
	secret := &corev1.Secret{}
	if err := h.Client.Get(ctx, client.ObjectKey{Name: secretName, Namespace: obj.GetNamespace()}, secret); err == nil {
		if err := h.Client.Delete(ctx, secret); err != nil {
			log.Error(err, "failed to delete secret", "secret", secretName)
			return fmt.Errorf("failed to delete secret: %w", err)
		}
		log.V(1).Info("Deleted secret", "secret", secretName)
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	return nil
}

// IsJobComplete checks if a job has completed (either succeeded or failed)
func IsJobComplete(job *batchv1.Job) bool {
	return job.Status.CompletionTime != nil
}

// Helper functions for adapters

func getObjectKind(obj RenderJobObject) string {
	// Determine the kind based on the object type
	switch obj.(type) {
	case *releaseAdapter:
		return "Release"
	case *hydratedTargetAdapter:
		return "HydratedTarget"
	default:
		// Fallback - should not happen if used correctly
		return "Unknown"
	}
}

func boolPtr(b bool) *bool {
	return &b
}
