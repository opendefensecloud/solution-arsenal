// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	ociname "github.com/google/go-containerregistry/pkg/name"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
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
	hydratedTargetFinalizer = "solar.opendefense.cloud/hydrated-target-finalizer"
)

// HydratedTargetReconciler reconciles a HydratedTarget object
type HydratedTargetReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Recorder    record.EventRecorder
	PushOptions solarv1alpha1.PushOptions
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=hydratedtargets/finalizers,verbs=update
// FIXME: Switch out releases for profiles                      👇
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks,verbs=get;list;watch;create;update;patch;delete
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
	res := &solarv1alpha1.HydratedTarget{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrlResult, nil
		}
		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Handle deletion: cleanup job and secret, then remove finalizer
	if !res.DeletionTimestamp.IsZero() {
		log.V(1).Info("HydratedTarget is being deleted")
		r.Recorder.Event(res, corev1.EventTypeWarning, "Deleting", "HydratedTarget is being deleted, cleaning up resources")

		if err := r.deleteRenderTask(ctx, res); err != nil {
			// TODO StatusCondition
			return ctrlResult, errLogAndWrap(log, err, "failed to delete render task")
		}

		// Remove finalizer
		if slices.Contains(res.Finalizers, hydratedTargetFinalizer) {
			log.V(1).Info("Removing finalizer from resource")
			res.Finalizers = slices.DeleteFunc(res.Finalizers, func(f string) bool {
				return f == hydratedTargetFinalizer
			})
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer")
			}
		}
		return ctrlResult, nil
	}

	// Add finalizer if not present and not deleting
	if res.DeletionTimestamp.IsZero() {
		if !slices.Contains(res.Finalizers, hydratedTargetFinalizer) {
			log.V(1).Info("Adding finalizer to resource")
			res.Finalizers = append(res.Finalizers, hydratedTargetFinalizer)
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer")
			}
			// Return without requeue; the Update event will trigger reconciliation again
			return ctrlResult, nil
		}
	}

	// Check if hydrated target has already completed successfully
	if apimeta.IsStatusConditionTrue(res.Status.Conditions, ConditionTypeJobSucceeded) {
		log.V(1).Info("HydratedTarget has already completed successfully, no further action needed")
		return ctrlResult, nil
	}

	rt := &solarv1alpha1.RenderTask{}
	err := r.Get(ctx, client.ObjectKey{Name: res.Name, Namespace: res.Namespace}, rt)
	if err != nil && apierrors.IsNotFound(err) {
		return ctrlResult, r.createRenderTask(ctx, res)
	}

	// TODO r.updateRenderTask(ctx, res)

	return ctrlResult, nil
}

func (r *HydratedTargetReconciler) createRenderTask(ctx context.Context, res *solarv1alpha1.HydratedTarget) error {
	cfg, err := r.computeRendererConfig(ctx, res)
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("unexpected nil RendererConfig")
	}

	rt := &solarv1alpha1.RenderTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      res.Name,
			Namespace: res.Namespace,
		},
		Spec: solarv1alpha1.RenderTaskSpec{
			RendererConfig: *cfg,
		},
	}

	return r.Create(ctx, rt)
}

func (r *HydratedTargetReconciler) deleteRenderTask(ctx context.Context, res *solarv1alpha1.HydratedTarget) error {
	rt := &solarv1alpha1.RenderTask{}
	if err := r.Get(ctx, client.ObjectKey{Name: res.Name, Namespace: res.Namespace}, rt); err != nil {
		return err
	}
	return r.Delete(ctx, rt, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func (r *HydratedTargetReconciler) computeRendererConfig(ctx context.Context, res *solarv1alpha1.HydratedTarget) (*solarv1alpha1.RendererConfig, error) {
	resolvedReleases := map[string]solarv1alpha1.ResourceAccess{}
	for k, v := range res.Spec.Releases {
		rel := &solarv1alpha1.Release{}
		if err := r.Get(ctx, client.ObjectKey{Name: v.Name, Namespace: res.Namespace}, rel); err != nil {
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
	url, err := url.JoinPath(po.ReferenceURL, res.Namespace, fmt.Sprintf("ht-%s", res.Name))
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(url, "oci://") {
		url = fmt.Sprintf("oci://%s", url)
	}

	version := fmt.Sprintf("v0.0.%d", res.GetGeneration())
	url = fmt.Sprintf("%s:%s", url, version)

	po.ReferenceURL = url

	return &solarv1alpha1.RendererConfig{
		Type: solarv1alpha1.RendererConfigTypeHydratedTarget,
		HydratedTargetConfig: solarv1alpha1.HydratedTargetConfig{
			Chart: solarv1alpha1.ChartConfig{
				Name:        res.Name,
				Description: fmt.Sprintf("HydratedTarget of %v", resolvedReleaseNames),
				Version:     version,
				AppVersion:  version,
			},
			Input: solarv1alpha1.HydratedTargetInput{
				Releases: resolvedReleases,
				Userdata: res.Spec.Userdata,
			},
		},
		PushOptions: po,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HydratedTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.HydratedTarget{}).
		Owns(&solarv1alpha1.RenderTask{}).
		Complete(r)
}
