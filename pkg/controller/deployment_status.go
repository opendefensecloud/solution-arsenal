// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"sync"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// reportState is how far a TargetReport can be trusted.
type reportState string

const (
	// reportMissing means the Target has no report: no agent is expected.
	reportMissing reportState = "Missing"
	// reportNoReport means the report was pre-created but never written.
	reportNoReport reportState = "NoReport"
	// reportStale means the agent stopped reporting.
	reportStale reportState = "Stale"
	// reportIncomplete means the agent reports, but could not read its releases.
	reportIncomplete reportState = "Incomplete"
	// reportFresh means the report is current and complete.
	reportFresh reportState = "Fresh"
)

// classifyReport decides whether report is usable at now. report may be nil.
// observedAt is when the controller saw the report's current LastReportTime
// first, by its own clock: the agent stamps LastReportTime itself, so its
// value only says that the agent wrote again, not when. The order matters: a
// stale report is stale whether or not it carries an error.
func classifyReport(report *solarv1alpha1.TargetReport, observedAt, now time.Time, staleAfter time.Duration) reportState {
	switch {
	case report == nil:
		return reportMissing
	case report.Spec.LastReportTime.IsZero():
		return reportNoReport
	case now.Sub(observedAt) > staleAfter:
		return reportStale
	case report.Spec.ReleasesError != "":
		return reportIncomplete
	default:
		return reportFresh
	}
}

// reportObservation is when the controller first saw a report's
// LastReportTime.
type reportObservation struct {
	lastReportTime metav1.Time
	observedAt     time.Time
}

// reportObservations remembers, per report, when the controller first saw its
// current LastReportTime, so staleness depends on the controller's clock only.
// It lives in memory; the zero value is ready to use.
type reportObservations struct {
	mu   sync.Mutex
	seen map[types.NamespacedName]reportObservation
}

// observe returns when lastReportTime was first seen for key. A new value
// counts from now. On the first sight since startup of a Target that was
// already stale, the value counts as never seen (zero time), so a restart
// doesn't show silent agents as reporting until the next write.
func (o *reportObservations) observe(key types.NamespacedName, lastReportTime metav1.Time, now time.Time, wasStale bool) time.Time {
	o.mu.Lock()
	defer o.mu.Unlock()

	seen, known := o.seen[key]
	switch {
	case known && seen.lastReportTime.Equal(&lastReportTime):
		return seen.observedAt
	case !known && wasStale:
		seen = reportObservation{lastReportTime: lastReportTime}
	default:
		seen = reportObservation{lastReportTime: lastReportTime, observedAt: now}
	}

	if o.seen == nil {
		o.seen = map[types.NamespacedName]reportObservation{}
	}

	o.seen[key] = seen

	return seen.observedAt
}

// forget drops what was observed for key, for a deleted report.
func (o *reportObservations) forget(key types.NamespacedName) {
	o.mu.Lock()
	defer o.mu.Unlock()

	delete(o.seen, key)
}

// syncAgentReporting sets the Target's AgentReporting condition for state and
// reports whether it changed. The message carries no timestamps, so an
// unchanged heartbeat leaves the Target untouched.
func syncAgentReporting(target *solarv1alpha1.Target, state reportState) bool {
	cond := metav1.Condition{Type: solarv1alpha1.ConditionTypeAgentReporting}

	switch state {
	case reportFresh, reportIncomplete:
		cond.Status, cond.Reason, cond.Message = metav1.ConditionTrue, solarv1alpha1.ReasonReporting,
			"The agent updates its TargetReport"
	case reportStale:
		cond.Status, cond.Reason, cond.Message = metav1.ConditionFalse, solarv1alpha1.ReasonStale,
			"The agent has not updated its TargetReport within the stale threshold"
	case reportNoReport, reportMissing:
		cond.Status, cond.Reason, cond.Message = metav1.ConditionUnknown, solarv1alpha1.ReasonNoReport,
			"The agent has not reported yet"
	}

	return apimeta.SetStatusCondition(&target.Status.Conditions, cond)
}

// syncNoReport removes the Target conditions a TargetReport produces, for a
// Target that has none, and reports whether anything changed.
func syncNoReport(target *solarv1alpha1.Target) bool {
	changed := false

	for _, typ := range []string{solarv1alpha1.ConditionTypeAgentReporting, solarv1alpha1.ConditionTypePreflightReady} {
		if apimeta.RemoveStatusCondition(&target.Status.Conditions, typ) {
			changed = true
		}
	}

	return changed
}

// reportRequeueAfter is when a current report goes stale without an update:
// nothing else triggers a reconcile when an agent falls silent. 0 means no
// requeue.
func reportRequeueAfter(state reportState, observedAt, now time.Time, staleAfter time.Duration) time.Duration {
	switch state {
	case reportFresh, reportIncomplete:
		return observedAt.Add(staleAfter).Sub(now) + time.Second
	default:
		return 0
	}
}

// bindingOutcome is what a binding's status should say. An empty reason means
// no Deployed condition is written yet.
type bindingOutcome struct {
	status     metav1.ConditionStatus
	reason     string
	message    string
	deployment *solarv1alpha1.ReleaseDeployment
}

func deriveOutcome(entry solarv1alpha1.ResolvedBinding) bindingOutcome {
	switch {
	case entry.Filtered:
		return bindingOutcome{status: metav1.ConditionFalse, reason: solarv1alpha1.ReasonFiltered, message: entry.Message}
	case entry.UniqueName == "":
		return bindingOutcome{status: metav1.ConditionUnknown, reason: solarv1alpha1.ReasonUnresolved, message: entry.Message}
	default:
		return bindingOutcome{}
	}
}

// syncBindingStatus writes outcome onto binding's status and reports whether
// anything changed. A nil deployment keeps the stored one, so the last state
// the agent reported survives. An empty reason only drops a Filtered or
// Unresolved condition: the binding is accepted now, and nothing else would
// replace it.
func syncBindingStatus(binding *solarv1alpha1.ReleaseBinding, outcome bindingOutcome) bool {
	if outcome.reason == "" {
		cond := apimeta.FindStatusCondition(binding.Status.Conditions, solarv1alpha1.ConditionTypeDeployed)
		if cond == nil || (cond.Reason != solarv1alpha1.ReasonFiltered && cond.Reason != solarv1alpha1.ReasonUnresolved) {
			return false
		}

		return apimeta.RemoveStatusCondition(&binding.Status.Conditions, solarv1alpha1.ConditionTypeDeployed)
	}

	changed := apimeta.SetStatusCondition(&binding.Status.Conditions, metav1.Condition{
		Type:               solarv1alpha1.ConditionTypeDeployed,
		Status:             outcome.status,
		Reason:             outcome.reason,
		Message:            outcome.message,
		ObservedGeneration: binding.Generation,
	})

	if outcome.deployment != nil && !apiequality.Semantic.DeepEqual(binding.Status.Deployment, outcome.deployment) {
		binding.Status.Deployment = outcome.deployment
		changed = true
	}

	return changed
}

// syncOrphanBinding marks a binding that names the Target but is no longer in
// its resolvedBindings, for example after its ReferenceGrant was revoked, so it
// doesn't keep an old Deployed=True. A binding without Deployed is left alone.
func syncOrphanBinding(binding *solarv1alpha1.ReleaseBinding) bool {
	if apimeta.FindStatusCondition(binding.Status.Conditions, solarv1alpha1.ConditionTypeDeployed) == nil {
		return false
	}

	return syncBindingStatus(binding, bindingOutcome{
		status:  metav1.ConditionUnknown,
		reason:  solarv1alpha1.ReasonUnresolved,
		message: "The binding is no longer resolved for its Target",
	})
}
