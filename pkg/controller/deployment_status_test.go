// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"reflect"
	"strings"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func TestClassifyReport(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	staleAfter := 10 * time.Minute

	// report is written by an agent whose clock is off by skew; observed is how
	// long ago the controller saw that write.
	report := func(skew time.Duration, releasesError string) *solarv1alpha1.TargetReport {
		return &solarv1alpha1.TargetReport{Spec: solarv1alpha1.TargetReportSpec{
			LastReportTime: metav1.NewTime(now.Add(skew)),
			ReleasesError:  releasesError,
		}}
	}

	for _, tc := range []struct {
		name     string
		report   *solarv1alpha1.TargetReport
		observed time.Duration
		want     reportState
	}{
		{"missing", nil, 0, reportMissing},
		{"never reported", &solarv1alpha1.TargetReport{}, 0, reportNoReport},
		{"stale", report(0, ""), 11 * time.Minute, reportStale},
		{"stale with error", report(0, "boom"), 11 * time.Minute, reportStale},
		{"fresh with error", report(0, "boom"), time.Minute, reportIncomplete},
		{"fresh", report(0, ""), time.Minute, reportFresh},
		{"exactly at the threshold", report(0, ""), 10 * time.Minute, reportFresh},
		{"agent clock far ahead, silent", report(2*time.Hour, ""), 11 * time.Minute, reportStale},
		{"agent clock far behind, reporting", report(-2*time.Hour, ""), time.Minute, reportFresh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyReport(tc.report, now.Add(-tc.observed), now, staleAfter); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReportObservations(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	key := types.NamespacedName{Namespace: "ns", Name: "t"}
	first := metav1.NewTime(now.Add(3 * time.Hour)) // the agent's clock says nothing
	second := metav1.NewTime(first.Add(30 * time.Second))

	t.Run("counts a write from when it was first seen", func(t *testing.T) {
		var o reportObservations
		if got := o.observe(key, first, now, false); !got.Equal(now) {
			t.Fatalf("first sight: got %s, want %s", got, now)
		}

		if got := o.observe(key, first, now.Add(time.Minute), false); !got.Equal(now) {
			t.Fatalf("same write later: got %s, want %s", got, now)
		}

		if got := o.observe(key, second, now.Add(2*time.Minute), false); !got.Equal(now.Add(2 * time.Minute)) {
			t.Fatalf("new write: got %s, want %s", got, now.Add(2*time.Minute))
		}
	})

	t.Run("keeps a Target that was stale before startup stale until the next write", func(t *testing.T) {
		var o reportObservations
		if got := o.observe(key, first, now, true); !got.IsZero() {
			t.Fatalf("first sight of a stale Target: got %s, want zero", got)
		}

		if got := o.observe(key, first, now.Add(time.Minute), false); !got.IsZero() {
			t.Fatalf("same write later: got %s, want zero", got)
		}

		if got := o.observe(key, second, now.Add(2*time.Minute), true); !got.Equal(now.Add(2 * time.Minute)) {
			t.Fatalf("new write: got %s, want %s", got, now.Add(2*time.Minute))
		}
	})

	t.Run("forgets a deleted report", func(t *testing.T) {
		var o reportObservations
		o.observe(key, first, now, false)
		o.forget(key)

		if got := o.observe(key, first, now.Add(time.Minute), false); !got.Equal(now.Add(time.Minute)) {
			t.Fatalf("after forget: got %s, want %s", got, now.Add(time.Minute))
		}
	})
}

func TestSyncAgentReporting(t *testing.T) {
	for _, tc := range []struct {
		state  reportState
		status metav1.ConditionStatus
		reason string
	}{
		{reportFresh, metav1.ConditionTrue, solarv1alpha1.ReasonReporting},
		{reportIncomplete, metav1.ConditionTrue, solarv1alpha1.ReasonReporting},
		{reportStale, metav1.ConditionFalse, solarv1alpha1.ReasonStale},
		{reportNoReport, metav1.ConditionUnknown, solarv1alpha1.ReasonNoReport},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			target := &solarv1alpha1.Target{}
			if !syncAgentReporting(target, tc.state) {
				t.Fatal("first sync must report a change")
			}

			cond := apimeta.FindStatusCondition(target.Status.Conditions, solarv1alpha1.ConditionTypeAgentReporting)
			if cond == nil || cond.Status != tc.status || cond.Reason != tc.reason {
				t.Fatalf("got %+v, want %s/%s", cond, tc.status, tc.reason)
			}

			if syncAgentReporting(target, tc.state) {
				t.Fatal("an identical second sync must not report a change")
			}
		})
	}
}

func TestSyncNoReportRemovesReportConditions(t *testing.T) {
	target := &solarv1alpha1.Target{}
	syncAgentReporting(target, reportFresh)
	apimeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type: solarv1alpha1.ConditionTypePreflightReady, Status: metav1.ConditionTrue, Reason: solarv1alpha1.ReasonAllChecksPassed,
	})
	apimeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type: ConditionTypeReleasesRendered, Status: metav1.ConditionTrue, Reason: "AllRendered",
	})

	if !syncNoReport(target) {
		t.Fatal("removing conditions must report a change")
	}

	for _, typ := range []string{solarv1alpha1.ConditionTypeAgentReporting, solarv1alpha1.ConditionTypePreflightReady} {
		if apimeta.FindStatusCondition(target.Status.Conditions, typ) != nil {
			t.Fatalf("%s must be removed", typ)
		}
	}

	if apimeta.FindStatusCondition(target.Status.Conditions, ConditionTypeReleasesRendered) == nil {
		t.Fatal("conditions not produced by the report must stay")
	}

	if syncNoReport(target) {
		t.Fatal("a second sync must not report a change")
	}
}

func TestReportRequeueAfter(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	staleAfter := 10 * time.Minute
	observed := now.Add(-4 * time.Minute)

	for _, tc := range []struct {
		name  string
		state reportState
		want  time.Duration
	}{
		{"fresh: just after it goes stale", reportFresh, 6*time.Minute + time.Second},
		{"incomplete: just after it goes stale", reportIncomplete, 6*time.Minute + time.Second},
		{"stale: never", reportStale, 0},
		{"no report: never", reportNoReport, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := reportRequeueAfter(tc.state, observed, now, staleAfter); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestDeriveOutcomeResolution(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entry   solarv1alpha1.ResolvedBinding
		status  metav1.ConditionStatus
		reason  string
		message string
	}{
		{
			name:    "filtered",
			entry:   solarv1alpha1.ResolvedBinding{Name: "b", UniqueName: "app", Filtered: true, Message: "lost to ns/a"},
			status:  metav1.ConditionFalse,
			reason:  solarv1alpha1.ReasonFiltered,
			message: "lost to ns/a",
		},
		{
			name:    "filtered without a uniqueName",
			entry:   solarv1alpha1.ResolvedBinding{Name: "b", Filtered: true, Message: "invalid selector"},
			status:  metav1.ConditionFalse,
			reason:  solarv1alpha1.ReasonFiltered,
			message: "invalid selector",
		},
		{
			name:    "unresolved",
			entry:   solarv1alpha1.ResolvedBinding{Name: "b", Message: "Release app not found"},
			status:  metav1.ConditionUnknown,
			reason:  solarv1alpha1.ReasonUnresolved,
			message: "Release app not found",
		},
		{
			name:  "accepted: nothing to say yet",
			entry: solarv1alpha1.ResolvedBinding{Name: "b", UniqueName: "app", DesiredRevision: "v0.0.1-abc"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveOutcome(tc.entry, nil, reportMissing)
			if got.status != tc.status || got.reason != tc.reason || got.message != tc.message {
				t.Fatalf("got %s/%s %q, want %s/%s %q", got.status, got.reason, got.message, tc.status, tc.reason, tc.message)
			}
		})
	}
}

func TestDeriveOutcomeState(t *testing.T) {
	ready := func(typ string) metav1.Condition {
		return metav1.Condition{Type: typ, Status: metav1.ConditionTrue, Reason: "Succeeded"}
	}
	notReady := func(message string) []metav1.Condition {
		return []metav1.Condition{{Type: "Ready", Status: metav1.ConditionFalse, Reason: "Failed", Message: message}}
	}
	rel := func(phase solarv1alpha1.ReleasePhase, revision string) solarv1alpha1.ReleaseReport {
		return solarv1alpha1.ReleaseReport{
			Name: "app", Namespace: "flux", Phase: phase, Revision: revision,
			SourceConditions: []metav1.Condition{ready("Ready")},
			HelmConditions:   []metav1.Condition{ready("Ready")},
		}
	}
	report := func(releases ...solarv1alpha1.ReleaseReport) *solarv1alpha1.TargetReport {
		return &solarv1alpha1.TargetReport{Spec: solarv1alpha1.TargetReportSpec{
			LastReportTime: metav1.Now(), Releases: releases,
		}}
	}
	entry := func(desired string) solarv1alpha1.ResolvedBinding {
		return solarv1alpha1.ResolvedBinding{Namespace: "ns", Name: "b", UniqueName: "app", DesiredRevision: desired}
	}

	failedHelm := rel(solarv1alpha1.ReleaseFailed, "v2")
	failedHelm.HelmConditions = notReady("boom")
	degradedSource := rel(solarv1alpha1.ReleaseDegraded, "v2")
	degradedSource.SourceConditions = notReady("denied")
	otherNamespace := rel(solarv1alpha1.ReleaseReady, "v2")
	otherNamespace.Namespace = "elsewhere"
	incomplete := report(rel(solarv1alpha1.ReleaseFailed, "v1"))
	incomplete.Spec.ReleasesError = "forbidden"

	for _, tc := range []struct {
		name    string
		entry   solarv1alpha1.ResolvedBinding
		report  *solarv1alpha1.TargetReport
		state   reportState
		status  metav1.ConditionStatus
		reason  string
		message []string
		// phase is the wanted deployment.phase; empty means no deployment, so
		// the stored one is kept.
		phase        solarv1alpha1.ReleasePhase
		flagsUnknown bool
	}{
		{name: "state missing", entry: entry("v2"), state: reportMissing},
		{name: "no report", entry: entry("v2"), report: &solarv1alpha1.TargetReport{}, state: reportNoReport,
			status: metav1.ConditionUnknown, reason: solarv1alpha1.ReasonNoReport, flagsUnknown: true},
		{name: "stale", entry: entry("v2"), report: report(rel(solarv1alpha1.ReleaseFailed, "v1")), state: reportStale,
			status: metav1.ConditionUnknown, reason: solarv1alpha1.ReasonAgentNotReporting, flagsUnknown: true},
		{name: "incomplete", entry: entry("v2"), report: incomplete, state: reportIncomplete,
			status: metav1.ConditionUnknown, reason: solarv1alpha1.ReasonReportIncomplete, message: []string{"forbidden"}, flagsUnknown: true},
		{name: "fresh, no entry", entry: entry("v2"), report: report(), state: reportFresh,
			status: metav1.ConditionFalse, reason: solarv1alpha1.ReasonPending, phase: solarv1alpha1.ReleasePending},
		{name: "two entries share the name", entry: entry("v2"), report: report(rel(solarv1alpha1.ReleaseReady, "v2"), otherNamespace), state: reportFresh,
			status: metav1.ConditionUnknown, reason: solarv1alpha1.ReasonReportIncomplete, flagsUnknown: true},
		{name: "ready with digest", entry: entry("v2"), report: report(rel(solarv1alpha1.ReleaseReady, "v2+0cc9a8446c95")), state: reportFresh,
			status: metav1.ConditionTrue, reason: solarv1alpha1.ReasonReady, phase: solarv1alpha1.ReleaseReady},
		{name: "ready without digest", entry: entry("v2"), report: report(rel(solarv1alpha1.ReleaseReady, "v2")), state: reportFresh,
			status: metav1.ConditionTrue, reason: solarv1alpha1.ReasonReady, phase: solarv1alpha1.ReleaseReady},
		{name: "ready, Flux drops the leading v", entry: entry("v0.0.2-ab12"), report: report(rel(solarv1alpha1.ReleaseReady, "0.0.2-ab12+0cc9a8446c95")), state: reportFresh,
			status: metav1.ConditionTrue, reason: solarv1alpha1.ReasonReady, phase: solarv1alpha1.ReleaseReady},
		{name: "ready on an older chart", entry: entry("v2"), report: report(rel(solarv1alpha1.ReleaseReady, "v1+aaa")), state: reportFresh,
			status: metav1.ConditionFalse, reason: solarv1alpha1.ReasonProgressing, message: []string{"v1", "v2"}, phase: solarv1alpha1.ReleaseReady},
		{name: "ready, desired unknown", entry: entry(""), report: report(rel(solarv1alpha1.ReleaseReady, "v2")), state: reportFresh,
			status: metav1.ConditionFalse, reason: solarv1alpha1.ReasonProgressing, phase: solarv1alpha1.ReleaseReady},
		{name: "failed", entry: entry("v2"), report: report(failedHelm), state: reportFresh,
			status: metav1.ConditionFalse, reason: solarv1alpha1.ReasonFailed, message: []string{"HelmRelease: boom"}, phase: solarv1alpha1.ReleaseFailed},
		{name: "degraded", entry: entry("v2"), report: report(degradedSource), state: reportFresh,
			status: metav1.ConditionFalse, reason: solarv1alpha1.ReasonDegraded, message: []string{"OCIRepository: denied"}, phase: solarv1alpha1.ReleaseDegraded},
		{name: "pending half pair", entry: entry("v2"), report: report(rel(solarv1alpha1.ReleasePending, "")), state: reportFresh,
			status: metav1.ConditionFalse, reason: solarv1alpha1.ReasonPending, phase: solarv1alpha1.ReleasePending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveOutcome(tc.entry, tc.report, tc.state)
			if got.status != tc.status || got.reason != tc.reason {
				t.Fatalf("got %s/%s %q, want %s/%s", got.status, got.reason, got.message, tc.status, tc.reason)
			}

			for _, part := range tc.message {
				if !strings.Contains(got.message, part) {
					t.Fatalf("message %q lacks %q", got.message, part)
				}
			}

			if got.flagsUnknown != tc.flagsUnknown {
				t.Fatalf("flagsUnknown = %v, want %v", got.flagsUnknown, tc.flagsUnknown)
			}

			switch {
			case tc.phase == "" && got.deployment != nil:
				t.Fatalf("want no deployment, got %+v", got.deployment)
			case tc.phase != "" && (got.deployment == nil || got.deployment.Phase != tc.phase ||
				got.deployment.DesiredRevision != tc.entry.DesiredRevision):
				t.Fatalf("got deployment %+v, want phase %s desired %s", got.deployment, tc.phase, tc.entry.DesiredRevision)
			}

			if tc.status == metav1.ConditionUnknown && got.desiredRevision != tc.entry.DesiredRevision {
				t.Fatalf("desiredRevision = %q, want %q", got.desiredRevision, tc.entry.DesiredRevision)
			}
		})
	}
}

func TestDeriveFlags(t *testing.T) {
	cond := func(typ string, status metav1.ConditionStatus, reason string) metav1.Condition {
		return metav1.Condition{Type: typ, Status: status, Reason: reason, Message: typ + " message"}
	}
	helm := func(c metav1.Condition) solarv1alpha1.ReleaseReport {
		return solarv1alpha1.ReleaseReport{HelmConditions: []metav1.Condition{c}}
	}

	for _, tc := range []struct {
		name   string
		rel    solarv1alpha1.ReleaseReport
		flag   string
		status metav1.ConditionStatus // empty means the flag is absent
		reason string
	}{
		{"rolled back", helm(cond("Remediated", metav1.ConditionTrue, "RollbackSucceeded")),
			solarv1alpha1.ConditionTypeRolledBack, metav1.ConditionTrue, "RollbackSucceeded"},
		{"remediated otherwise", helm(cond("Remediated", metav1.ConditionTrue, "UninstallSucceeded")),
			solarv1alpha1.ConditionTypeRolledBack, metav1.ConditionFalse, "UninstallSucceeded"},
		{"drifted", helm(cond("Drifted", metav1.ConditionTrue, "DriftDetected")),
			solarv1alpha1.ConditionTypeDrifted, metav1.ConditionTrue, "DriftDetected"},
		{"test failed", helm(cond("TestSuccess", metav1.ConditionFalse, "TestFailed")),
			solarv1alpha1.ConditionTypeTestFailed, metav1.ConditionTrue, "TestFailed"},
		{"tests passed", helm(cond("TestSuccess", metav1.ConditionTrue, "TestSucceeded")),
			solarv1alpha1.ConditionTypeTestFailed, metav1.ConditionFalse, "TestSucceeded"},
		{"verified", solarv1alpha1.ReleaseReport{SourceConditions: []metav1.Condition{cond("SourceVerified", metav1.ConditionTrue, "Succeeded")}},
			solarv1alpha1.ConditionTypeVerified, metav1.ConditionTrue, "Succeeded"},
		{"no remediation", solarv1alpha1.ReleaseReport{}, solarv1alpha1.ConditionTypeRolledBack, "", ""},
		{"no drift condition", solarv1alpha1.ReleaseReport{}, solarv1alpha1.ConditionTypeDrifted, "", ""},
		{"no test condition", solarv1alpha1.ReleaseReport{}, solarv1alpha1.ConditionTypeTestFailed, "", ""},
		{"no verification", solarv1alpha1.ReleaseReport{}, solarv1alpha1.ConditionTypeVerified, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := apimeta.FindStatusCondition(deriveFlags(&tc.rel), tc.flag)
			if tc.status == "" {
				if got != nil {
					t.Fatalf("want no %s, got %+v", tc.flag, got)
				}

				return
			}

			if got == nil || got.Status != tc.status || got.Reason != tc.reason || got.Message == "" {
				t.Fatalf("got %+v, want %s %s/%s", got, tc.flag, tc.status, tc.reason)
			}
		})
	}

	t.Run("a drifted release stays deployed", func(t *testing.T) {
		rel := solarv1alpha1.ReleaseReport{
			Name: "app", Phase: solarv1alpha1.ReleaseReady, Revision: "v2",
			HelmConditions: []metav1.Condition{cond("Drifted", metav1.ConditionTrue, "DriftDetected")},
		}
		report := &solarv1alpha1.TargetReport{Spec: solarv1alpha1.TargetReportSpec{
			LastReportTime: metav1.Now(), Releases: []solarv1alpha1.ReleaseReport{rel},
		}}

		got := deriveOutcome(solarv1alpha1.ResolvedBinding{UniqueName: "app", DesiredRevision: "v2"}, report, reportFresh)
		if got.status != metav1.ConditionTrue || apimeta.FindStatusCondition(got.flags, solarv1alpha1.ConditionTypeDrifted) == nil {
			t.Fatalf("got %s/%s flags %+v, want True with Drifted", got.status, got.reason, got.flags)
		}
	})
}

func TestNewestChartLive(t *testing.T) {
	for _, tc := range []struct {
		live, desired string
		want          bool
	}{
		{"v2+0cc9a8446c95", "v2", true},
		{"2+0cc9a8446c95", "v2", true},
		{"v2", "v2", true},
		{"v1+aaa", "v2", false},
		{"", "v2", false},
		{"v2", "", false},
	} {
		ok, msg := newestChartLive(tc.live, tc.desired)
		if ok != tc.want || (!ok && msg == "") || (ok && msg != "") {
			t.Errorf("newestChartLive(%q, %q) = %v %q, want %v", tc.live, tc.desired, ok, msg, tc.want)
		}
	}
}

func TestSyncBindingStatus(t *testing.T) {
	filtered := bindingOutcome{status: metav1.ConditionFalse, reason: solarv1alpha1.ReasonFiltered, message: "lost"}

	t.Run("writes the Deployed condition once", func(t *testing.T) {
		binding := &solarv1alpha1.ReleaseBinding{Generation: 3}
		if !syncBindingStatus(binding, filtered) {
			t.Fatal("first sync must report a change")
		}

		cond := apimeta.FindStatusCondition(binding.Status.Conditions, solarv1alpha1.ConditionTypeDeployed)
		if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != solarv1alpha1.ReasonFiltered ||
			cond.Message != "lost" || cond.ObservedGeneration != 3 {
			t.Fatalf("got %+v", cond)
		}

		transition := cond.LastTransitionTime
		if syncBindingStatus(binding, filtered) {
			t.Fatal("an identical second sync must not report a change")
		}
		if got := apimeta.FindStatusCondition(binding.Status.Conditions, solarv1alpha1.ConditionTypeDeployed).LastTransitionTime; !got.Equal(&transition) {
			t.Fatalf("lastTransitionTime moved from %s to %s", transition, got)
		}
	})

	// A binding that was filtered or unresolved and is now accepted must not
	// keep that condition: nothing else would ever replace it.
	for _, stale := range []bindingOutcome{
		filtered,
		{status: metav1.ConditionUnknown, reason: solarv1alpha1.ReasonUnresolved, message: "Release x not found"},
	} {
		t.Run("an empty reason drops a stale "+stale.reason, func(t *testing.T) {
			binding := &solarv1alpha1.ReleaseBinding{}
			syncBindingStatus(binding, stale)

			if !syncBindingStatus(binding, bindingOutcome{}) {
				t.Fatal("dropping the condition must report a change")
			}
			if cond := apimeta.FindStatusCondition(binding.Status.Conditions, solarv1alpha1.ConditionTypeDeployed); cond != nil {
				t.Fatalf("want no Deployed, got %+v", cond)
			}
		})
	}

	drifted := metav1.Condition{Type: solarv1alpha1.ConditionTypeDrifted, Status: metav1.ConditionTrue, Reason: "DriftDetected"}
	rolledBack := metav1.Condition{Type: solarv1alpha1.ConditionTypeRolledBack, Status: metav1.ConditionTrue, Reason: "RollbackSucceeded"}
	ready := bindingOutcome{
		status: metav1.ConditionTrue, reason: solarv1alpha1.ReasonReady,
		deployment: &solarv1alpha1.ReleaseDeployment{Phase: solarv1alpha1.ReleaseReady, LiveRevision: "v1+aaa", DesiredRevision: "v1"},
		flags:      []metav1.Condition{drifted, rolledBack},
	}

	// The report was deleted: what it produced goes, the last deployment stays.
	t.Run("an empty outcome drops Deployed and the flags", func(t *testing.T) {
		binding := &solarv1alpha1.ReleaseBinding{}
		syncBindingStatus(binding, ready)

		if !syncBindingStatus(binding, bindingOutcome{}) {
			t.Fatal("dropping the conditions must report a change")
		}
		if len(binding.Status.Conditions) != 0 {
			t.Fatalf("want no conditions, got %+v", binding.Status.Conditions)
		}
		if !reflect.DeepEqual(binding.Status.Deployment, ready.deployment) {
			t.Fatalf("deployment changed: %+v", binding.Status.Deployment)
		}

		fresh := &solarv1alpha1.ReleaseBinding{}
		if syncBindingStatus(fresh, bindingOutcome{}) || len(fresh.Status.Conditions) != 0 {
			t.Fatalf("an empty outcome must add nothing, got %+v", fresh.Status)
		}
	})

	t.Run("a flag no longer derived is removed", func(t *testing.T) {
		binding := &solarv1alpha1.ReleaseBinding{}
		syncBindingStatus(binding, ready)

		next := ready
		next.flags = []metav1.Condition{drifted}
		if !syncBindingStatus(binding, next) {
			t.Fatal("removing a flag must report a change")
		}
		if apimeta.FindStatusCondition(binding.Status.Conditions, solarv1alpha1.ConditionTypeRolledBack) != nil {
			t.Fatal("RolledBack must be removed")
		}
		if apimeta.FindStatusCondition(binding.Status.Conditions, solarv1alpha1.ConditionTypeDrifted) == nil {
			t.Fatal("Drifted must stay")
		}
		if syncBindingStatus(binding, next) {
			t.Fatal("an identical second sync must not report a change")
		}
	})

	t.Run("a stale report turns the flags Unknown and keeps the deployment", func(t *testing.T) {
		binding := &solarv1alpha1.ReleaseBinding{}
		syncBindingStatus(binding, ready)

		stale := bindingOutcome{
			status: metav1.ConditionUnknown, reason: solarv1alpha1.ReasonAgentNotReporting, message: "silent",
			desiredRevision: "v2", flagsUnknown: true,
		}
		if !syncBindingStatus(binding, stale) {
			t.Fatal("going stale must report a change")
		}

		for _, typ := range []string{solarv1alpha1.ConditionTypeDrifted, solarv1alpha1.ConditionTypeRolledBack} {
			cond := apimeta.FindStatusCondition(binding.Status.Conditions, typ)
			if cond == nil || cond.Status != metav1.ConditionUnknown || cond.Reason != solarv1alpha1.ReasonAgentNotReporting {
				t.Fatalf("%s: got %+v, want Unknown/AgentNotReporting", typ, cond)
			}
		}

		want := solarv1alpha1.ReleaseDeployment{Phase: solarv1alpha1.ReleaseReady, LiveRevision: "v1+aaa", DesiredRevision: "v2"}
		if binding.Status.Deployment == nil || *binding.Status.Deployment != want {
			t.Fatalf("got deployment %+v, want %+v", binding.Status.Deployment, want)
		}
		if syncBindingStatus(binding, stale) {
			t.Fatal("an identical second sync must not report a change")
		}
	})
}

func TestSyncOrphanBinding(t *testing.T) {
	if syncOrphanBinding(&solarv1alpha1.ReleaseBinding{}) {
		t.Fatal("a binding without Deployed must stay untouched")
	}

	binding := &solarv1alpha1.ReleaseBinding{}
	apimeta.SetStatusCondition(&binding.Status.Conditions, metav1.Condition{
		Type: solarv1alpha1.ConditionTypeDeployed, Status: metav1.ConditionTrue, Reason: "Ready",
	})
	if !syncOrphanBinding(binding) {
		t.Fatal("a binding with Deployed must change")
	}

	cond := apimeta.FindStatusCondition(binding.Status.Conditions, solarv1alpha1.ConditionTypeDeployed)
	if cond.Status != metav1.ConditionUnknown || cond.Reason != solarv1alpha1.ReasonUnresolved {
		t.Fatalf("got %+v, want Unknown/Unresolved", cond)
	}
	if syncOrphanBinding(binding) {
		t.Fatal("a second sync must not report a change")
	}
}
