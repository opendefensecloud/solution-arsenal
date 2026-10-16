// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"slices"
	"time"

	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/ociregistry"
)

const (
	// availabilityCheckTimeout bounds one registry check so a hanging registry
	// cannot block a reconcile worker. A timeout appears as status
	// RegistryUnreachable.
	availabilityCheckTimeout = 10 * time.Second

	// availabilityQPS and availabilityBurst cap registry checks per second across
	// all ComponentVersions, bounding the controller's egress to registries.
	// availabilityMaxConcurrentReconciles is how many ComponentVersions are
	// reconciled in parallel, i.e. how many slow registries can be waited on at once.
	availabilityQPS                     = 10
	availabilityBurst                   = 20
	availabilityMaxConcurrentReconciles = 4
)

// ComponentVersionReconciler manages the componentVersionFinalizer on each
// ComponentVersion so deletion is observable by other controllers. It also
// maintains the Available condition by re-checking the backing descriptor
// manifest every AvailabilityInterval.
type ComponentVersionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	WatchNamespace string
	// AvailabilityInterval is how often the backing descriptor manifest is re-checked.
	AvailabilityInterval time.Duration
	// AvailabilityGrace is how long a manifest must stay missing before Available=False.
	AvailabilityGrace time.Duration

	// limiter is built in SetupWithManager; nil means unlimited.
	limiter *rate.Limiter
}

// validate rejects settings that would silently stop or starve availability checks.
func (r *ComponentVersionReconciler) validate() error {
	switch {
	case r.AvailabilityInterval <= 0:
		return fmt.Errorf("availability interval must be positive, got %v", r.AvailabilityInterval)
	case r.AvailabilityGrace < 0:
		return fmt.Errorf("availability grace must not be negative, got %v", r.AvailabilityGrace)
	}

	return nil
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=registries,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get

func (r *ComponentVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("ComponentVersion is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	cv := &solarv1alpha1.ComponentVersion{}
	if err := r.Get(ctx, req.NamespacedName, cv); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get ComponentVersion: %w", err)
	}

	// Handle deletion: remove the self-finalizer.
	if !cv.DeletionTimestamp.IsZero() {
		if slices.Contains(cv.Finalizers, componentVersionFinalizer) {
			latest := &solarv1alpha1.ComponentVersion{}
			if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get latest ComponentVersion for finalizer removal: %w", err)
			}
			original := latest.DeepCopy()
			latest.Finalizers = slices.DeleteFunc(latest.Finalizers, func(s string) bool { return s == componentVersionFinalizer })
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from ComponentVersion: %w", err)
			}
		}

		return ctrl.Result{}, nil
	}

	// Ensure self-finalizer exists.
	if !slices.Contains(cv.Finalizers, componentVersionFinalizer) {
		latest := &solarv1alpha1.ComponentVersion{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get latest ComponentVersion for finalizer addition: %w", err)
		}
		if !slices.Contains(latest.Finalizers, componentVersionFinalizer) {
			original := latest.DeepCopy()
			latest.Finalizers = append(latest.Finalizers, componentVersionFinalizer)
			if err := r.Patch(ctx, latest, client.MergeFrom(original)); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to add finalizer to ComponentVersion: %w", err)
			}
		}
	}

	return r.reconcileAvailability(ctx, cv)
}

// reconcileAvailability re-checks the backing descriptor manifest, stores the
// Available condition when it changed, and schedules the next check.
func (r *ComponentVersionReconciler) reconcileAvailability(ctx context.Context, cv *solarv1alpha1.ComponentVersion) (ctrl.Result, error) {
	cond, delay, err := r.checkAvailability(ctx, cv)
	if err != nil {
		return ctrl.Result{}, err
	}
	if delay > 0 {
		// Throttled: nothing was checked, so keep the current condition and come back later.
		return ctrl.Result{RequeueAfter: delay}, nil
	}
	cond.ObservedGeneration = cv.Generation

	original := cv.DeepCopy()
	if setAvailableCondition(&cv.Status.Conditions, cond, time.Now()) {
		if err := r.Status().Patch(ctx, cv, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update ComponentVersion status: %w", err)
		}
	}

	return ctrl.Result{RequeueAfter: r.AvailabilityInterval}, nil
}

// checkAvailability computes the next Available condition. Registry problems are
// expressed in the condition; only API errors are returned. A positive delay means
// the rate limit allowed no registry check yet and the condition was not computed.
func (r *ComponentVersionReconciler) checkAvailability(ctx context.Context, cv *solarv1alpha1.ComponentVersion) (metav1.Condition, time.Duration, error) {
	digest := cv.Spec.Digest
	repo := cv.DescriptorRepository()
	if digest == "" || repo == "" {
		return availableCondition(metav1.ConditionUnknown, solarv1alpha1.ReasonNoDigest,
			"no manifest digest recorded; delete the ComponentVersion so discovery re-creates it"), 0, nil
	}

	reg, found, err := r.registryFor(ctx, cv)
	if err != nil {
		return metav1.Condition{}, 0, err
	}
	if !found {
		return availableCondition(metav1.ConditionUnknown, solarv1alpha1.ReasonRegistryNotConfigured,
			"no Registry in namespace "+cv.Namespace+" serves "+cv.Spec.Registry), 0, nil
	}

	auth, err := registryAuth(ctx, r.Client, reg)
	if err != nil {
		return availableCondition(metav1.ConditionUnknown, solarv1alpha1.ReasonRegistryUnreachable, err.Error()), 0, nil
	}

	// Take a token only right before contacting the registry, so CVs that are never
	// checked (NoDigest, RegistryNotConfigured) don't use up the egress budget.
	if r.limiter != nil {
		res := r.limiter.Reserve()
		if d := res.Delay(); d > 0 {
			res.Cancel()

			return metav1.Condition{}, d, nil
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, availabilityCheckTimeout)
	defer cancel()
	ref := fmt.Sprintf("%s/%s@%s", cv.Spec.Registry, repo, digest)
	exists, checkErr := ociregistry.ManifestExists(checkCtx, ref, auth, reg.Spec.PlainHTTP)

	cur := apimeta.FindStatusCondition(cv.Status.Conditions, solarv1alpha1.ConditionTypeAvailable)

	return availabilityFromCheck(cur, exists, checkErr, time.Now(), r.AvailabilityGrace), 0, nil
}

// registryFor returns the Registry in the CV's namespace whose hostname serves cv.spec.registry.
func (r *ComponentVersionReconciler) registryFor(ctx context.Context, cv *solarv1alpha1.ComponentVersion) (*solarv1alpha1.Registry, bool, error) {
	list := &solarv1alpha1.RegistryList{}
	if err := r.List(ctx, list, client.InNamespace(cv.Namespace)); err != nil {
		return nil, false, fmt.Errorf("failed to list Registries in namespace %s: %w", cv.Namespace, err)
	}
	for i := range list.Items {
		if normalizeRegistryHost(list.Items[i].Spec.Hostname) == cv.Spec.Registry {
			return &list.Items[i], true, nil
		}
	}

	return nil, false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.validate(); err != nil {
		return fmt.Errorf("invalid ComponentVersion controller settings: %w", err)
	}
	r.limiter = rate.NewLimiter(rate.Limit(availabilityQPS), availabilityBurst)

	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.ComponentVersion{}, builder.WithPredicates(componentVersionUpdatePredicate())).
		WithOptions(crcontroller.Options{MaxConcurrentReconciles: availabilityMaxConcurrentReconciles}).
		Complete(r)
}
