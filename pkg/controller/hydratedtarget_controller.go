// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-logr/logr"
	ociname "github.com/google/go-containerregistry/pkg/name"
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

// HydratedTargetReconciler reconciles a HydratedTarget object
type HydratedTargetReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	RendererImage   string
	RendererCommand string
	RendererArgs    []string
	PushOptions     renderer.PushOptions
}

// Ensure HydratedTargetReconciler implements ConfigBuilder
var _ ConfigBuilder = (*HydratedTargetReconciler)(nil)

// Implement ConfigBuilder interface
func (r *HydratedTargetReconciler) BuildConfig(ctx context.Context, log logr.Logger, obj RenderJobObject) ([]byte, error) {
	adapter := obj.(*hydratedTargetAdapter)
	ht := adapter.HydratedTarget
	_ = ht

	// Resolve the Releases
	resolvedReleases := map[string]solarv1alpha1.ResourceAccess{}
	for k, v := range ht.Spec.Releases {
		rel := &solarv1alpha1.Release{}
		if err := r.Get(ctx, client.ObjectKey{Name: v.Name, Namespace: ht.Namespace}, rel); err != nil {
			return nil, err
		}

		ref, err := ociname.ParseReference(rel.Status.ChartURL)
		if err != nil {
			return nil, err
		}

		repo, err := url.JoinPath(ref.Context().RegistryStr(), ref.Context().RepositoryStr())
		if err != nil {
			return nil, err
		}

		resolvedReleases[k] = solarv1alpha1.ResourceAccess{
			Repository: strings.TrimPrefix(repo, "oci://"),
			Tag:        ref.Identifier(),
		}
	}

	resolvedReleaseNames := []string{}
	for k := range resolvedReleases {
		resolvedReleaseNames = append(resolvedReleaseNames, k)
	}

	po := r.PushOptions
	url, err := url.JoinPath(po.ReferenceURL, ht.Namespace, fmt.Sprintf("ht-%s", ht.Name))
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(url, "oci://") {
		url = fmt.Sprintf("oci://%s", url)
	}

	version := fmt.Sprintf("v0.0.%d", ht.GetGeneration()) // FIXME: Generate the Version
	url = fmt.Sprintf("%s:%s", url, version)

	po.ReferenceURL = url

	// Build the renderer configuration
	cfg := renderer.Config{
		Type: renderer.TypeHydratedTarget,
		HydratedTargetConfig: renderer.HydratedTargetConfig{
			Chart: renderer.ChartConfig{
				Name:        ht.Name,
				Description: fmt.Sprintf("HydratedTarget of %v", resolvedReleaseNames),
				Version:     version,
				AppVersion:  version,
			},
			Input: renderer.HydratedTargetInput{
				Releases: resolvedReleases,
				Userdata: ht.Spec.Userdata,
			},
		},
		PushOptions: po,
	}

	return json.Marshal(cfg)
}

func (r *HydratedTargetReconciler) GetRecorder() record.EventRecorder {
	return r.Recorder
}

func (r *HydratedTargetReconciler) GetRendererImage() string {
	return r.RendererImage
}

func (r *HydratedTargetReconciler) GetRendererCommand() string {
	return r.RendererCommand
}

func (r *HydratedTargetReconciler) GetRendererArgs() []string {
	return r.RendererArgs
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets/finalizers,verbs=update
// FIXME: Switch out releases for profiles                      👇
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile moves the current state of the cluster closer to the desired state
//
// Reconciliation Flow:
//
//	HydratedTarget created
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
func (r *HydratedTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("HydratedTarget is being reconciled", "req", req)

	// Fetch the HydratedTarget instance
	ht := &solarv1alpha1.HydratedTarget{}
	if err := r.Get(ctx, req.NamespacedName, ht); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrlResult, nil
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Create helper with HydratedTarget-specific config builder
	helper := &RenderJobHelper{
		Client:        r.Client,
		Scheme:        r.Scheme,
		ConfigBuilder: r,
	}

	// Create an adapter that wraps HydratedTarget as RenderJobObject
	adapter := &hydratedTargetAdapter{HydratedTarget: ht}

	// Handle deletion: cleanup job and secret, then remove finalizer
	if !ht.DeletionTimestamp.IsZero() {
		log.V(1).Info("HydratedTarget is being deleted")
		r.Recorder.Event(ht, corev1.EventTypeWarning, "Deleting", "HydratedTarget is being deleted, cleaning up resources")

		if err := helper.CleanupResources(ctx, log, adapter); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to cleanup resources")
		}

		if err := helper.RemoveFinalizer(ctx, log, adapter, ht); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer")
		}
		return ctrlResult, nil
	}

	// Add finalizer if not present
	added, err := helper.EnsureFinalizer(ctx, log, adapter, ht)
	if err != nil {
		return ctrlResult, errLogAndWrap(log, err, "failed to ensure finalizer")
	}
	if added {
		// Return without requeue; the Update event will trigger reconciliation again
		return ctrlResult, nil
	}

	// Check if hydrated target has already completed successfully
	if apimeta.IsStatusConditionTrue(ht.Status.Conditions, ConditionTypeJobSucceeded) {
		log.V(1).Info("HydratedTarget has already completed successfully, no further action needed")
		return ctrlResult, nil
	}

	// Create or update configuration secret
	configSecret, err := helper.CreateOrUpdateConfigSecret(ctx, log, adapter)
	if err != nil {
		r.Recorder.Event(ht, corev1.EventTypeWarning, "ConfigFailed", fmt.Sprintf("Failed to create config secret: %v", err))
		if changed := apimeta.SetStatusCondition(&ht.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeJobScheduled,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: ht.Generation,
			Reason:             "ConfigSecretFailed",
			Message:            fmt.Sprintf("Failed to create config secret: %v", err),
		}); changed {
			if err := r.Status().Update(ctx, ht); err != nil {
				log.Error(err, "failed to update HydratedTarget status")
			}
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to create config secret")
	}

	ht.Status.ConfigSecretRef = &corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Secret",
		Name:       configSecret.Name,
		Namespace:  configSecret.Namespace,
		UID:        configSecret.UID,
	}

	// Get or create the job
	job, err := helper.GetOrCreateJob(ctx, log, adapter, configSecret)
	if err != nil {
		r.Recorder.Event(ht, corev1.EventTypeWarning, "JobFailed", fmt.Sprintf("Failed to create or get job: %v", err))
		if changed := apimeta.SetStatusCondition(&ht.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeJobScheduled,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: ht.Generation,
			Reason:             "JobCreationFailed",
			Message:            fmt.Sprintf("Failed to create job: %v", err),
		}); changed {
			if err := r.Status().Update(ctx, ht); err != nil {
				log.Error(err, "failed to update HydratedTarget status")
			}
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to create job")
	}

	if job != nil {
		ht.Status.JobRef = &corev1.ObjectReference{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       job.Name,
			Namespace:  job.Namespace,
			UID:        job.UID,
		}

		// Check job status and update status if required
		if changed := helper.UpdateResourceStatusFromJob(ctx, log, adapter, job); changed {
			if err := r.Status().Update(ctx, ht); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to update HydratedTarget status")
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
func (r *HydratedTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.HydratedTarget{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

// Adapter to use HydratedTarget directly as RenderJobObject while delegating to client.Update
type hydratedTargetAdapter struct {
	*solarv1alpha1.HydratedTarget
}

func (a *hydratedTargetAdapter) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *hydratedTargetAdapter) SetConditions(conditions []metav1.Condition) {
	a.Status.Conditions = conditions
}

func (a *hydratedTargetAdapter) GetResourceName() string {
	return a.Name
}

func (a *hydratedTargetAdapter) GetObjectReference() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: solarv1alpha1.SchemeGroupVersion.String(),
		Kind:       "HydratedTarget",
		Name:       a.Name,
		Namespace:  a.Namespace,
		UID:        a.UID,
	}
}

func (a *hydratedTargetAdapter) SetJobRef(ref *corev1.ObjectReference) {
	a.Status.JobRef = ref
}

func (a *hydratedTargetAdapter) SetConfigSecretRef(ref *corev1.ObjectReference) {
	a.Status.ConfigSecretRef = ref
}

func (a *hydratedTargetAdapter) RuntimeObject() runtime.Object {
	return a.HydratedTarget
}
