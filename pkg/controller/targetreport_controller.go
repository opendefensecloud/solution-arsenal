// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"cmp"
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const DefaultReportStaleAfter = 10 * time.Minute

// TargetReportReconciler projects a TargetReport onto its Target's status: the
// preflight checks as the PreflightReady aggregate, and the agent's liveness
// as AgentReporting. It also writes the Deployed condition of the bindings in
// the Target's resolvedBindings.
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
	// ReportStaleAfter is how old a report may get before the agent counts as
	// not reporting. Zero means DefaultReportStaleAfter.
	ReportStaleAfter time.Duration

	// observations records when each report was last written, by the
	// controller's clock.
	observations reportObservations
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targetreports,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releasebindings,verbs=get;list;watch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=releasebindings/status,verbs=get;update;patch

// Reconcile syncs one TargetReport onto its Target's status: the preflight
// checks as the PreflightReady aggregate, and whether the agent still reports
// as AgentReporting. A deleted report removes both conditions. It then writes
// each resolved binding's Deployed condition, with or without a report. A
// missing Target awaits ownerRef garbage collection of its report.
func (r *TargetReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}

	target := &solarv1alpha1.Target{}
	if err := r.APIReader.Get(ctx, req.NamespacedName, target); err != nil {
		if apierrors.IsNotFound(err) {
			r.observations.forget(req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get Target: %w", err)
	}

	report := &solarv1alpha1.TargetReport{}
	if err := r.Get(ctx, req.NamespacedName, report); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to get TargetReport: %w", err)
		}

		report = nil
	}

	now := time.Now()
	staleAfter := r.staleAfter()

	var observedAt time.Time
	if report == nil {
		r.observations.forget(req.NamespacedName)
	} else {
		cond := apimeta.FindStatusCondition(target.Status.Conditions, solarv1alpha1.ConditionTypeAgentReporting)
		wasStale := cond != nil && cond.Reason == solarv1alpha1.ReasonStale
		observedAt = r.observations.observe(req.NamespacedName, report.Spec.LastReportTime, now, wasStale)
	}

	state := classifyReport(report, observedAt, now, staleAfter)

	var changed bool
	if report == nil {
		changed = syncNoReport(target)
	} else {
		changed = syncPreflight(report, target)
		changed = syncAgentReporting(target, state) || changed
	}

	if changed {
		if err := r.Status().Update(ctx, target); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update Target report conditions: %w", err)
		}

		log.V(1).Info("Synced report conditions", "target", target.Name, "report", state)
	}

	if err := r.syncBindings(ctx, target); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: reportRequeueAfter(state, observedAt, now, staleAfter)}, nil
}

// syncBindings writes the Deployed condition of each binding in the Target's
// resolvedBindings, across namespaces, and marks bindings that name the Target
// but are no longer listed. A listed binding that is gone, or that now names
// another Target, is skipped: the list survives TargetReconciler's early
// returns, so it can be out of date.
func (r *TargetReportReconciler) syncBindings(ctx context.Context, target *solarv1alpha1.Target) error {
	listed := make(map[types.NamespacedName]bool, len(target.Status.ResolvedBindings))

	for _, entry := range target.Status.ResolvedBindings {
		key := types.NamespacedName{Namespace: entry.Namespace, Name: entry.Name}
		listed[key] = true

		binding := &solarv1alpha1.ReleaseBinding{}
		if err := r.Get(ctx, key, binding); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			return fmt.Errorf("failed to get ReleaseBinding %s: %w", key, err)
		}

		if !bindingNamesTarget(binding, target) {
			continue
		}

		if syncBindingStatus(binding, deriveOutcome(entry)) {
			if err := r.Status().Update(ctx, binding); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to update ReleaseBinding %s status: %w", key, err)
			}
		}
	}

	bindings := &solarv1alpha1.ReleaseBindingList{}
	if err := r.List(ctx, bindings, client.MatchingFields{indexReleaseBindingTargetName: target.Name}); err != nil {
		return fmt.Errorf("failed to list ReleaseBindings: %w", err)
	}

	for i := range bindings.Items {
		binding := &bindings.Items[i]
		if !bindingNamesTarget(binding, target) || listed[client.ObjectKeyFromObject(binding)] {
			continue
		}

		if syncOrphanBinding(binding) {
			if err := r.Status().Update(ctx, binding); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to update ReleaseBinding %s status: %w", client.ObjectKeyFromObject(binding), err)
			}
		}
	}

	return nil
}

// bindingNamesTarget reports whether binding's targetRef resolves to target.
func bindingNamesTarget(binding *solarv1alpha1.ReleaseBinding, target *solarv1alpha1.Target) bool {
	return binding.Spec.TargetRef.Name == target.Name &&
		cmp.Or(binding.Spec.TargetRef.Namespace, binding.Namespace) == target.Namespace
}

func (r *TargetReportReconciler) staleAfter() time.Duration {
	if r.ReportStaleAfter > 0 {
		return r.ReportStaleAfter
	}

	return DefaultReportStaleAfter
}

// syncPreflight writes the aggregate of the report's checks as the Target's
// single PreflightReady condition and reports whether anything changed. The
// per-check results stay in the report: a snapshot cannot know their
// transitions, so the controller only rolls them up.
func syncPreflight(report *solarv1alpha1.TargetReport, target *solarv1alpha1.Target) bool {
	aggStatus, aggReason, aggMessage := aggregatePreflight(report.Spec.Preflight.Checks)
	return apimeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type:    solarv1alpha1.ConditionTypePreflightReady,
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
		return metav1.ConditionFalse, solarv1alpha1.ReasonChecksErroredAndFailed,
			"checks that could not be evaluated: " + strings.Join(errored, ", ") +
				"; failed checks: " + strings.Join(failed, ", ")
	case len(failed) > 0:
		return metav1.ConditionFalse, solarv1alpha1.ReasonChecksFailed, "failed checks: " + strings.Join(failed, ", ")
	case len(errored) > 0:
		return metav1.ConditionUnknown, solarv1alpha1.ReasonChecksErrored, "checks that could not be evaluated: " + strings.Join(errored, ", ")
	case len(checks) == 0:
		return metav1.ConditionUnknown, solarv1alpha1.ReasonNoChecks, "report contains no checks"
	default:
		return metav1.ConditionTrue, solarv1alpha1.ReasonAllChecksPassed, ""
	}
}

// SetupWithManager sets up the controller with the Manager. Report and Target
// share name and namespace, so each report request maps directly onto its Target.
// Targets are watched for two events: a Target appearing after its report
// (Reconcile bails while the Target is absent, and nothing else would
// re-trigger it), and a change to its resolvedBindings, which decides the
// bindings' status even without a report. Reconcile still reads the Target via APIReader on
// purpose — the watch adds the Target to the cache, but a cached read would
// mask the race with TargetReconciler's status writes.
func (r *TargetReportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.TargetReport{}).
		Watches(
			&solarv1alpha1.Target{},
			handler.EnqueueRequestsFromMapFunc(mapTargetToReport),
			builder.WithPredicates(targetResolvedBindingsChangedPredicate()),
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
