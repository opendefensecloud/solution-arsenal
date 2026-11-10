// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
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
