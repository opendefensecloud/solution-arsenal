// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	// ConditionTypePreflightReady aggregates all preflight checks for a Target.
	ConditionTypePreflightReady = "PreflightReady"
)

// TargetReportReconciler projects a TargetReport's preflight checks onto its
// Target's status as the PreflightReady aggregate.
type TargetReportReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// APIReader reads uncached: TargetReconciler also writes Target.status, so
	// a stale cached read could silently overwrite a concurrent update. An
	// uncached read makes that race surface as a conflict instead — the error
	// is returned as-is and controller-runtime's backoff retries with a fresh
	// read.
	APIReader client.Reader
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	WatchNamespace string
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targetreports,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/status,verbs=get;update;patch

// Reconcile syncs one TargetReport's preflight checks onto the Target's
// status conditions as the PreflightReady aggregate. A deleted report leaves
// the last observed condition in place; a missing Target awaits ownerRef
// garbage collection of its report.
func (r *TargetReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	report := &solarv1alpha1.TargetReport{}
	if err := r.Get(ctx, req.NamespacedName, report); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get TargetReport: %w", err)
	}

	target := &solarv1alpha1.Target{}
	if err := r.APIReader.Get(ctx, req.NamespacedName, target); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get Target: %w", err)
	}

	if !syncPreflight(report, target) {
		return ctrl.Result{}, nil
	}

	if err := r.Status().Update(ctx, target); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update Target preflight conditions: %w", err)
	}

	log.V(1).Info("Synced preflight conditions", "target", target.Name)

	return ctrl.Result{}, nil
}

// syncPreflight writes the aggregate of the report's checks as the Target's
// single PreflightReady condition and reports whether anything changed. The
// per-check results stay in the report: a snapshot cannot know their
// transitions, so the controller only rolls them up.
func syncPreflight(report *solarv1alpha1.TargetReport, target *solarv1alpha1.Target) bool {
	aggStatus, aggReason, aggMessage := aggregatePreflight(report.Spec.Preflight.Checks)
	return apimeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type:    ConditionTypePreflightReady,
		Status:  aggStatus,
		Reason:  aggReason,
		Message: aggMessage,
	})
}

// aggregatePreflight reduces the current report's checks to one status: a
// definite failure wins (the aggregate is False regardless of unknowns), but
// errored checks are surfaced first in the message because a check that could
// not run is more fundamental than an expected failure. An empty report is
// Unknown, and only all-passing yields True.
func aggregatePreflight(checks []solarv1alpha1.PreflightCheck) (metav1.ConditionStatus, string, string) {
	var failed, errored []string

	for _, check := range checks {
		switch check.Status {
		case metav1.ConditionFalse:
			failed = append(failed, check.Type)
		case metav1.ConditionTrue:
			// passed: the aggregate derives from failed/errored lists
		case metav1.ConditionUnknown:
			errored = append(errored, check.Type)
		}
	}

	switch {
	case len(failed) > 0 && len(errored) > 0:
		return metav1.ConditionFalse, "ChecksErroredAndFailed",
			"checks that could not be evaluated: " + strings.Join(errored, ", ") +
				"; failed checks: " + strings.Join(failed, ", ")
	case len(failed) > 0:
		return metav1.ConditionFalse, "ChecksFailed", "failed checks: " + strings.Join(failed, ", ")
	case len(errored) > 0:
		return metav1.ConditionUnknown, "ChecksErrored", "checks that could not be evaluated: " + strings.Join(errored, ", ")
	case len(checks) == 0:
		return metav1.ConditionUnknown, "NoChecks", "report contains no checks"
	default:
		return metav1.ConditionTrue, "AllChecksPassed", ""
	}
}

// SetupWithManager sets up the controller with the Manager. Report and Target
// share name and namespace, so each report request maps directly onto its Target.
// Targets are watched so a Target that appears after its report re-syncs the
// preflight aggregate: Reconcile bails while the Target is absent, and nothing
// else would re-trigger it. Reconcile still reads the Target via APIReader on
// purpose — the watch adds the Target to the cache, but a cached read would
// mask the race with TargetReconciler's status writes.
func (r *TargetReportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.TargetReport{}).
		Watches(
			&solarv1alpha1.Target{},
			handler.EnqueueRequestsFromMapFunc(mapTargetToReport),
			builder.WithPredicates(targetCreatePredicate()),
		).
		Complete(r)
}

// mapTargetToReport returns the TargetReport request for a Target event.
// Report and Target share name and namespace, so each Target maps 1:1 to its
// report.
func mapTargetToReport(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}}
}
