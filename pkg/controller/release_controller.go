// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
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

const (
	releaseFinalizer = "solar.opendefense.cloud/release-finalizer"
)

// ReleaseReconciler reconciles a Release object
type ReleaseReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Recorder    record.EventRecorder
	PushOptions solarv1alpha1.PushOptions
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releases/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=rendertasks,verbs=get;list;watch;create;update;patch;delete
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
//	Get or create RenderTask
//	    ↓
//	Update release status from RenderTask
//	    ↓
//	RenderTask completed with success?
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

		if err := r.deleteRenderTask(ctx, res); err != nil {
			// TODO StatusCondition
			return ctrlResult, errLogAndWrap(log, err, "failed to delete render task")
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

	// Add finalizer if not present and not deleting
	if res.DeletionTimestamp.IsZero() {
		if !slices.Contains(res.Finalizers, releaseFinalizer) {
			log.V(1).Info("Adding finalizer to resource")
			res.Finalizers = append(res.Finalizers, releaseFinalizer)
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer")
			}
			// Return without requeue; the Update event will trigger reconciliation again
			return ctrlResult, nil
		}
	}

	// Check if release has already completed successfully
	if apimeta.IsStatusConditionTrue(res.Status.Conditions, ConditionTypeJobSucceeded) {
		log.V(1).Info("Release has already completed successfully, no further action needed")
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

func (r *ReleaseReconciler) createRenderTask(ctx context.Context, res *solarv1alpha1.Release) error {
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

func (r *ReleaseReconciler) deleteRenderTask(ctx context.Context, res *solarv1alpha1.Release) error {
	rt := &solarv1alpha1.RenderTask{}
	if err := r.Get(ctx, client.ObjectKey{Name: res.Name, Namespace: res.Namespace}, rt); err != nil {
		return err
	}
	return r.Delete(ctx, rt, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func (r *ReleaseReconciler) computeRendererConfig(ctx context.Context, res *solarv1alpha1.Release) (*solarv1alpha1.RendererConfig, error) {
	cvRef := types.NamespacedName{
		Name:      res.Spec.ComponentVersionRef.Name,
		Namespace: res.Namespace,
	}
	cv := &solarv1alpha1.ComponentVersion{}

	if err := r.Get(ctx, cvRef, cv); err != nil {
		return nil, err
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
		Type: solarv1alpha1.RendererConfigTypeRelease,
		ReleaseConfig: solarv1alpha1.ReleaseConfig{
			Chart: solarv1alpha1.ChartConfig{
				Name:        res.Name,
				Description: fmt.Sprintf("Release of %s", res.Spec.ComponentVersionRef.Name),
				Version:     version,
				AppVersion:  version,
			},
			Input: solarv1alpha1.ReleaseInput{
				Component: solarv1alpha1.ReleaseComponent{Name: cv.Spec.ComponentRef.Name},
				Helm:      cv.Spec.Helm,
				KRO:       cv.Spec.KRO,
				Resources: cv.Spec.Resources,
			},
			Values: res.Spec.Values.Raw,
		},
		PushOptions: po,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Release{}).
		Owns(&solarv1alpha1.RenderTask{}).
		Complete(r)
}
