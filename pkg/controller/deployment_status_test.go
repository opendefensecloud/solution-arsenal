// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"reflect"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func TestClassifyReport(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	staleAfter := 10 * time.Minute

	report := func(age time.Duration, releasesError string) *solarv1alpha1.TargetReport {
		return &solarv1alpha1.TargetReport{Spec: solarv1alpha1.TargetReportSpec{
			LastReportTime: metav1.NewTime(now.Add(-age)),
			ReleasesError:  releasesError,
		}}
	}

	for _, tc := range []struct {
		name   string
		report *solarv1alpha1.TargetReport
		want   reportState
	}{
		{"missing", nil, reportMissing},
		{"never reported", &solarv1alpha1.TargetReport{}, reportNoReport},
		{"stale", report(11*time.Minute, ""), reportStale},
		{"stale with error", report(11*time.Minute, "boom"), reportStale},
		{"fresh with error", report(time.Minute, "boom"), reportIncomplete},
		{"fresh", report(time.Minute, ""), reportFresh},
		{"exactly at the threshold", report(10*time.Minute, ""), reportFresh},
		{"slightly ahead of the controller clock", report(-30*time.Second, ""), reportFresh},
		{"far ahead of the controller clock", report(-2*time.Hour, ""), reportClockAhead},
		{"far ahead with error", report(-2*time.Hour, "boom"), reportClockAhead},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyReport(tc.report, now, staleAfter); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
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
		{reportClockAhead, metav1.ConditionUnknown, solarv1alpha1.ReasonClockSkew},
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
		Type: ConditionTypePreflightReady, Status: metav1.ConditionTrue, Reason: ReasonAllChecksPassed,
	})
	apimeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type: ConditionTypeReleasesRendered, Status: metav1.ConditionTrue, Reason: "AllRendered",
	})

	if !syncNoReport(target) {
		t.Fatal("removing conditions must report a change")
	}

	for _, typ := range []string{solarv1alpha1.ConditionTypeAgentReporting, ConditionTypePreflightReady} {
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
	at := func(offset time.Duration) *solarv1alpha1.TargetReport {
		return &solarv1alpha1.TargetReport{Spec: solarv1alpha1.TargetReportSpec{LastReportTime: metav1.NewTime(now.Add(offset))}}
	}

	for _, tc := range []struct {
		name   string
		report *solarv1alpha1.TargetReport
		state  reportState
		want   time.Duration
	}{
		{"fresh: just after it goes stale", at(-4 * time.Minute), reportFresh, 6*time.Minute + time.Second},
		{"incomplete: just after it goes stale", at(-4 * time.Minute), reportIncomplete, 6*time.Minute + time.Second},
		{"clock ahead: once the time is within the tolerance", at(2 * time.Hour), reportClockAhead, 2*time.Hour - reportClockSkewTolerance + time.Second},
		{"stale: never", at(-time.Hour), reportStale, 0},
		{"no report: never", &solarv1alpha1.TargetReport{}, reportNoReport, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := reportRequeueAfter(tc.report, tc.state, now, staleAfter); got != tc.want {
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
			got := deriveOutcome(tc.entry)
			if got.status != tc.status || got.reason != tc.reason || got.message != tc.message {
				t.Fatalf("got %s/%s %q, want %s/%s %q", got.status, got.reason, got.message, tc.status, tc.reason, tc.message)
			}
		})
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

	t.Run("an empty reason changes nothing else", func(t *testing.T) {
		binding := &solarv1alpha1.ReleaseBinding{}
		syncBindingStatus(binding, bindingOutcome{status: metav1.ConditionTrue, reason: "Ready"})
		before := binding.Status.DeepCopy()

		if syncBindingStatus(binding, bindingOutcome{}) {
			t.Fatal("an empty outcome must not report a change")
		}
		if !reflect.DeepEqual(before, &binding.Status) {
			t.Fatalf("status changed: %+v", binding.Status)
		}

		fresh := &solarv1alpha1.ReleaseBinding{}
		if syncBindingStatus(fresh, bindingOutcome{}) || len(fresh.Status.Conditions) != 0 {
			t.Fatalf("an empty outcome must add nothing, got %+v", fresh.Status)
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
