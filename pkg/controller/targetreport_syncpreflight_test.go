// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"strings"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func TestSyncPreflight(t *testing.T) {
	t.Parallel()

	check := func(checkType string, status metav1.ConditionStatus, reason string) solarv1alpha1.PreflightCheck {
		return solarv1alpha1.PreflightCheck{Type: checkType, Status: status, Reason: reason}
	}

	t.Run("projects only the aggregate, keeping checks in the report", func(t *testing.T) {
		t.Parallel()

		target := &solarv1alpha1.Target{}
		report := &solarv1alpha1.TargetReport{Spec: solarv1alpha1.TargetReportSpec{
			Preflight: solarv1alpha1.PreflightReport{Checks: []solarv1alpha1.PreflightCheck{
				check("FluxCrds", metav1.ConditionTrue, "Present"),
				check("Quota", metav1.ConditionFalse, "Insufficient"),
				check("Dns", metav1.ConditionUnknown, "Timeout"),
			}},
		}}

		if !syncPreflight(report, target) {
			t.Fatal("syncPreflight reported no change on the first sync")
		}

		if len(target.Status.Conditions) != 1 {
			t.Fatalf("got %d conditions, want exactly the aggregate", len(target.Status.Conditions))
		}

		agg := target.Status.Conditions[0]
		if agg.Type != ConditionTypePreflightReady || agg.Status != metav1.ConditionFalse || agg.Reason != "ChecksErroredAndFailed" {
			t.Fatalf("got aggregate condition %+v, want status False reason ChecksErroredAndFailed", agg)
		}
		errored := strings.Index(agg.Message, "Dns")
		failed := strings.Index(agg.Message, "Quota")
		if errored < 0 || failed < 0 || errored > failed {
			t.Errorf("got aggregate message %q, want Dns listed before Quota", agg.Message)
		}
	})

	t.Run("passes an all-passing report", func(t *testing.T) {
		t.Parallel()

		target := &solarv1alpha1.Target{}
		report := &solarv1alpha1.TargetReport{Spec: solarv1alpha1.TargetReportSpec{
			Preflight: solarv1alpha1.PreflightReport{Checks: []solarv1alpha1.PreflightCheck{
				check("FluxCrds", metav1.ConditionTrue, "Present"),
			}},
		}}

		if !syncPreflight(report, target) {
			t.Fatal("syncPreflight reported no change")
		}

		agg := apimeta.FindStatusCondition(target.Status.Conditions, ConditionTypePreflightReady)
		if agg == nil || agg.Status != metav1.ConditionTrue || agg.Reason != "AllChecksPassed" {
			t.Errorf("got aggregate condition %+v, want status True reason AllChecksPassed", agg)
		}
	})

	t.Run("reports NoChecks for an empty report", func(t *testing.T) {
		t.Parallel()

		target := &solarv1alpha1.Target{}
		report := &solarv1alpha1.TargetReport{}

		if !syncPreflight(report, target) {
			t.Fatal("syncPreflight reported no change")
		}

		if len(target.Status.Conditions) != 1 {
			t.Fatalf("got %d conditions, want exactly the aggregate", len(target.Status.Conditions))
		}

		agg := target.Status.Conditions[0]
		if agg.Status != metav1.ConditionUnknown || agg.Reason != "NoChecks" {
			t.Errorf("got aggregate condition %+v, want status Unknown reason NoChecks", agg)
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		t.Parallel()

		target := &solarv1alpha1.Target{}
		report := &solarv1alpha1.TargetReport{Spec: solarv1alpha1.TargetReportSpec{
			Preflight: solarv1alpha1.PreflightReport{Checks: []solarv1alpha1.PreflightCheck{
				check("FluxCrds", metav1.ConditionTrue, "Present"),
			}},
		}}

		if !syncPreflight(report, target) {
			t.Fatal("first syncPreflight reported no change")
		}
		if syncPreflight(report, target) {
			t.Error("second syncPreflight reported a change on unchanged input")
		}
	})
}
