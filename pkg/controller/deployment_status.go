// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	// reportClockAhead means the report time is ahead of the controller's clock
	// by more than reportClockSkewTolerance, so its age says nothing.
	reportClockAhead reportState = "ClockAhead"
	// reportIncomplete means the agent reports, but could not read its releases.
	reportIncomplete reportState = "Incomplete"
	// reportFresh means the report is current and complete.
	reportFresh reportState = "Fresh"
)

// reportClockSkewTolerance is how far an agent's clock may run ahead of the
// controller's. The agent stamps LastReportTime itself, so without a bound a
// clock hours ahead would keep a silent agent "Reporting" for hours.
const reportClockSkewTolerance = time.Minute

// classifyReport decides whether report is usable at now. report may be nil.
// The order matters: a stale report is stale whether or not it carries an
// error.
func classifyReport(report *solarv1alpha1.TargetReport, now time.Time, staleAfter time.Duration) reportState {
	switch {
	case report == nil:
		return reportMissing
	case report.Spec.LastReportTime.IsZero():
		return reportNoReport
	case report.Spec.LastReportTime.Sub(now) > reportClockSkewTolerance:
		return reportClockAhead
	case now.Sub(report.Spec.LastReportTime.Time) > staleAfter:
		return reportStale
	case report.Spec.ReleasesError != "":
		return reportIncomplete
	default:
		return reportFresh
	}
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
	case reportClockAhead:
		cond.Status, cond.Reason, cond.Message = metav1.ConditionUnknown, solarv1alpha1.ReasonClockSkew,
			"The agent's report time is ahead of the controller's clock; check the target cluster's time sync"
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

	for _, typ := range []string{solarv1alpha1.ConditionTypeAgentReporting, ConditionTypePreflightReady} {
		if apimeta.RemoveStatusCondition(&target.Status.Conditions, typ) {
			changed = true
		}
	}

	return changed
}

// reportRequeueAfter is when report's state next changes without an update:
// a current report goes stale, and a report from a clock running ahead comes
// within reportClockSkewTolerance. Nothing else triggers a reconcile when an
// agent falls silent. 0 means no requeue.
func reportRequeueAfter(report *solarv1alpha1.TargetReport, state reportState, now time.Time, staleAfter time.Duration) time.Duration {
	switch state {
	case reportFresh, reportIncomplete:
		return report.Spec.LastReportTime.Add(staleAfter).Sub(now) + time.Second
	case reportClockAhead:
		return report.Spec.LastReportTime.Add(-reportClockSkewTolerance).Sub(now) + time.Second
	default:
		return 0
	}
}
