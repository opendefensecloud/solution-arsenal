// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
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
