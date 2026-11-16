// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"strings"
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

// flagConditionTypes are the binding conditions that can be true beside any
// Deployed state.
var flagConditionTypes = []string{
	solarv1alpha1.ConditionTypeRolledBack,
	solarv1alpha1.ConditionTypeDrifted,
	solarv1alpha1.ConditionTypeTestFailed,
	solarv1alpha1.ConditionTypeVerified,
}

// bindingOutcome is what a binding's status should say. An empty reason means
// there is no Deployed condition: the binding is accepted, but no report
// covers it.
type bindingOutcome struct {
	status  metav1.ConditionStatus
	reason  string
	message string
	// deployment replaces the stored one. When nil, the stored one is kept, so
	// the last state the agent reported survives, and only its DesiredRevision
	// follows desiredRevision.
	deployment      *solarv1alpha1.ReleaseDeployment
	desiredRevision string
	// flags are the flag conditions to write; the others are removed. With
	// flagsUnknown, the stored flags turn Unknown with this outcome's reason
	// instead.
	flags        []metav1.Condition
	flagsUnknown bool
}

// deriveOutcome decides a binding's status from its resolvedBindings entry and
// the Target's report, classified as state. A filtered or unresolved binding
// doesn't depend on the agent. report is nil only for reportMissing.
func deriveOutcome(entry solarv1alpha1.ResolvedBinding, report *solarv1alpha1.TargetReport, state reportState) bindingOutcome {
	switch {
	case entry.Filtered:
		return bindingOutcome{status: metav1.ConditionFalse, reason: solarv1alpha1.ReasonFiltered, message: entry.Message}
	case entry.UniqueName == "":
		return bindingOutcome{status: metav1.ConditionUnknown, reason: solarv1alpha1.ReasonUnresolved, message: entry.Message}
	}

	unknown := func(reason, message string) bindingOutcome {
		return bindingOutcome{
			status: metav1.ConditionUnknown, reason: reason, message: message,
			desiredRevision: entry.DesiredRevision, flagsUnknown: true,
		}
	}

	switch state {
	case reportMissing:
		return bindingOutcome{}
	case reportNoReport:
		return unknown(solarv1alpha1.ReasonNoReport, "The agent has not reported yet")
	case reportStale:
		return unknown(solarv1alpha1.ReasonAgentNotReporting,
			"The agent has not updated its TargetReport within the stale threshold")
	case reportIncomplete:
		return unknown(solarv1alpha1.ReasonReportIncomplete,
			"The agent could not read its Flux objects: "+report.Spec.ReleasesError)
	case reportFresh:
	}

	var rel *solarv1alpha1.ReleaseReport

	matches := 0
	for i := range report.Spec.Releases {
		if report.Spec.Releases[i].Name == entry.UniqueName {
			rel = &report.Spec.Releases[i]
			matches++
		}
	}

	switch matches {
	case 0:
		return bindingOutcome{
			status: metav1.ConditionFalse, reason: solarv1alpha1.ReasonPending,
			message:    "The agent reports no Flux objects for " + entry.UniqueName,
			deployment: &solarv1alpha1.ReleaseDeployment{Phase: solarv1alpha1.ReleasePending, DesiredRevision: entry.DesiredRevision},
		}
	case 1:
	default:
		return unknown(solarv1alpha1.ReasonReportIncomplete,
			fmt.Sprintf("The report has %d entries named %s", matches, entry.UniqueName))
	}

	outcome := bindingOutcome{
		status: metav1.ConditionFalse,
		deployment: &solarv1alpha1.ReleaseDeployment{
			Phase: rel.Phase, LiveRevision: rel.Revision, DesiredRevision: entry.DesiredRevision,
		},
		flags: deriveFlags(rel),
	}

	switch rel.Phase {
	case solarv1alpha1.ReleaseReady:
		if live, message := newestChartLive(rel.Revision, entry.DesiredRevision); live {
			outcome.status, outcome.reason = metav1.ConditionTrue, solarv1alpha1.ReasonReady
		} else {
			outcome.reason, outcome.message = solarv1alpha1.ReasonProgressing, message
		}
	case solarv1alpha1.ReleasePending:
		outcome.reason, outcome.message = solarv1alpha1.ReasonPending, "Only one half of the Flux pair exists"
	case solarv1alpha1.ReleaseProgressing, solarv1alpha1.ReleaseDegraded, solarv1alpha1.ReleaseFailed:
		outcome.reason, outcome.message = string(rel.Phase), notReadyMessage(rel)
	default:
		return unknown(solarv1alpha1.ReasonReportIncomplete, fmt.Sprintf("The report has an unknown phase %q", rel.Phase))
	}

	return outcome
}

// newestChartLive is the single place for whether the newest rendered chart
// is live, and says why not otherwise. Flux reports the live chart version
// without a leading "v" and may append the chart digest after a "+", so the
// versions are compared without either.
func newestChartLive(liveRevision, desiredRevision string) (bool, string) {
	liveVersion, _ := solarv1alpha1.SplitRevision(liveRevision)

	switch {
	case desiredRevision == "":
		return false, "The newest rendered chart isn't known yet"
	case liveRevision == "":
		return false, fmt.Sprintf("No chart is live yet, want %s", desiredRevision)
	case strings.TrimPrefix(liveVersion, "v") != strings.TrimPrefix(desiredRevision, "v"):
		return false, fmt.Sprintf("Chart %s is live, want %s", liveRevision, desiredRevision)
	default:
		return true, ""
	}
}

// notReadyMessage is the Ready message of the first half of the Flux pair that
// isn't ready, prefixed with its kind. The source comes first: a failed fetch
// also fails the HelmRelease.
func notReadyMessage(rel *solarv1alpha1.ReleaseReport) string {
	for _, half := range []struct {
		kind       string
		conditions []metav1.Condition
	}{
		{"OCIRepository", rel.SourceConditions},
		{"HelmRelease", rel.HelmConditions},
	} {
		if cond := apimeta.FindStatusCondition(half.conditions, "Ready"); cond != nil && cond.Status != metav1.ConditionTrue {
			return half.kind + ": " + cond.Message
		}
	}

	return ""
}

// deriveFlags reads the flag conditions from the release's verbatim Flux
// conditions. A flag whose source condition is absent is absent too. Reasons
// and messages are Flux's.
func deriveFlags(rel *solarv1alpha1.ReleaseReport) []metav1.Condition {
	var flags []metav1.Condition

	flag := func(typ string, status metav1.ConditionStatus, source *metav1.Condition) {
		flags = append(flags, metav1.Condition{Type: typ, Status: status, Reason: source.Reason, Message: source.Message})
	}

	if c := apimeta.FindStatusCondition(rel.HelmConditions, "Remediated"); c != nil {
		status := metav1.ConditionFalse
		if c.Status == metav1.ConditionTrue && c.Reason == "RollbackSucceeded" {
			status = metav1.ConditionTrue
		}

		flag(solarv1alpha1.ConditionTypeRolledBack, status, c)
	}

	if c := apimeta.FindStatusCondition(rel.HelmConditions, "Drifted"); c != nil {
		flag(solarv1alpha1.ConditionTypeDrifted, c.Status, c)
	}

	if c := apimeta.FindStatusCondition(rel.HelmConditions, "TestSuccess"); c != nil {
		status := metav1.ConditionUnknown
		switch c.Status {
		case metav1.ConditionTrue:
			status = metav1.ConditionFalse
		case metav1.ConditionFalse:
			status = metav1.ConditionTrue
		case metav1.ConditionUnknown:
		}

		flag(solarv1alpha1.ConditionTypeTestFailed, status, c)
	}

	if c := apimeta.FindStatusCondition(rel.SourceConditions, "SourceVerified"); c != nil {
		flag(solarv1alpha1.ConditionTypeVerified, c.Status, c)
	}

	return flags
}

// syncBindingStatus writes outcome onto binding's status and reports whether
// anything changed. An empty reason removes Deployed and the flags, for
// example after the report was deleted, but keeps the last deployment.
func syncBindingStatus(binding *solarv1alpha1.ReleaseBinding, outcome bindingOutcome) bool {
	conditions := &binding.Status.Conditions

	var changed bool
	if outcome.reason == "" {
		changed = apimeta.RemoveStatusCondition(conditions, solarv1alpha1.ConditionTypeDeployed)
	} else {
		changed = apimeta.SetStatusCondition(conditions, metav1.Condition{
			Type:               solarv1alpha1.ConditionTypeDeployed,
			Status:             outcome.status,
			Reason:             outcome.reason,
			Message:            outcome.message,
			ObservedGeneration: binding.Generation,
		})
	}

	if outcome.flagsUnknown {
		for _, typ := range flagConditionTypes {
			if apimeta.FindStatusCondition(*conditions, typ) == nil {
				continue
			}

			changed = apimeta.SetStatusCondition(conditions, metav1.Condition{
				Type: typ, Status: metav1.ConditionUnknown, Reason: outcome.reason, Message: outcome.message,
				ObservedGeneration: binding.Generation,
			}) || changed
		}
	} else {
		for _, typ := range flagConditionTypes {
			if apimeta.FindStatusCondition(outcome.flags, typ) == nil {
				changed = apimeta.RemoveStatusCondition(conditions, typ) || changed
			}
		}

		for _, flag := range outcome.flags {
			flag.ObservedGeneration = binding.Generation
			changed = apimeta.SetStatusCondition(conditions, flag) || changed
		}
	}

	switch {
	case outcome.deployment != nil:
		if !apiequality.Semantic.DeepEqual(binding.Status.Deployment, outcome.deployment) {
			binding.Status.Deployment = outcome.deployment
			changed = true
		}
	case outcome.desiredRevision != "", outcome.flagsUnknown && binding.Status.Deployment != nil:
		if binding.Status.Deployment == nil {
			binding.Status.Deployment = &solarv1alpha1.ReleaseDeployment{}
		}

		if binding.Status.Deployment.DesiredRevision != outcome.desiredRevision {
			binding.Status.Deployment.DesiredRevision = outcome.desiredRevision
			changed = true
		}
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
