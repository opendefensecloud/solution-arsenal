// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These tests run as plain Go tests (not Ginkgo specs), so they don't trigger
// the envtest BeforeSuite in suite_test.go and stay runnable in environments
// where the kubebuilder etcd binary is not available.

// initialBucket is the [low, high] range a fresh-wait return can fall into:
// dependencyWaitInitialInterval ± dependencyWaitRandomizationFactor.
var (
	initialLow  = time.Duration(float64(dependencyWaitInitialInterval) * (1 - dependencyWaitRandomizationFactor))
	initialHigh = time.Duration(float64(dependencyWaitInitialInterval) * (1 + dependencyWaitRandomizationFactor))
	maxLow      = time.Duration(float64(dependencyWaitMaxInterval) * (1 - dependencyWaitRandomizationFactor))
	maxHigh     = time.Duration(float64(dependencyWaitMaxInterval) * (1 + dependencyWaitRandomizationFactor))
)

func TestRequeueAfterForCondition(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		cond    *metav1.Condition
		minWant time.Duration
		maxWant time.Duration
	}{
		{
			name:    "nil condition treated as fresh wait",
			cond:    nil,
			minWant: initialLow,
			maxWant: initialHigh,
		},
		{
			name:    "zero LastTransitionTime treated as fresh wait",
			cond:    &metav1.Condition{},
			minWant: initialLow,
			maxWant: initialHigh,
		},
		{
			name:    "negative age (clock skew) treated as fresh wait",
			cond:    &metav1.Condition{LastTransitionTime: metav1.NewTime(now.Add(time.Minute))},
			minWant: initialLow,
			maxWant: initialHigh,
		},
		{
			name:    "age well past saturation returns saturated interval",
			cond:    &metav1.Condition{LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
			minWant: maxLow,
			maxWant: maxHigh,
		},
		{
			name:    "age at one day stays saturated, doesn't blow up the loop",
			cond:    &metav1.Condition{LastTransitionTime: metav1.NewTime(now.Add(-24 * time.Hour))},
			minWant: maxLow,
			maxWant: maxHigh,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := requeueAfterForCondition(tc.cond, now)
			if got < tc.minWant || got > tc.maxWant {
				t.Errorf("got %v, want in [%v, %v]", got, tc.minWant, tc.maxWant)
			}
		})
	}
}

// TestRequeueAfterForConditionGrows asserts the curve grows monotonically (in
// expectation) from a fresh wait to a saturated wait. Run as a single sample
// per age — jitter can put any single comparison in the wrong order, so the
// assertion is just that the saturated bucket and the fresh bucket are
// disjoint, which they are by construction (7.5s < 150s).
func TestRequeueAfterForConditionGrows(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC)

	fresh := requeueAfterForCondition(
		&metav1.Condition{LastTransitionTime: metav1.NewTime(now)},
		now,
	)
	old := requeueAfterForCondition(
		&metav1.Condition{LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))},
		now,
	)

	if fresh > initialHigh {
		t.Errorf("fresh wait returned %v, should be ≤ %v", fresh, initialHigh)
	}
	if old < maxLow {
		t.Errorf("hour-old wait returned %v, should be ≥ %v (saturated)", old, maxLow)
	}
	if fresh >= old {
		t.Errorf("fresh wait %v should be smaller than hour-old wait %v", fresh, old)
	}
}

func TestRequeueAfterForConditions(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC)
	fresh := &metav1.Condition{LastTransitionTime: metav1.NewTime(now)}
	old := &metav1.Condition{LastTransitionTime: metav1.NewTime(now.Add(-time.Hour))}

	t.Run("returns zero when given no non-nil conditions", func(t *testing.T) {
		t.Parallel()
		if got := requeueAfterForConditions(now); got != 0 {
			t.Errorf("got %v, want 0", got)
		}
		if got := requeueAfterForConditions(now, nil, nil); got != 0 {
			t.Errorf("got %v, want 0", got)
		}
	})

	t.Run("picks the soonest across multiple conditions", func(t *testing.T) {
		t.Parallel()
		// fresh bucket [2.5s, 7.5s] is disjoint from saturated bucket
		// [2.5m, 7.5m], so the min must come from the fresh condition.
		got := requeueAfterForConditions(now, old, fresh, nil)
		if got > initialHigh {
			t.Errorf("got %v, want ≤ %v (the fresh wait's upper bound)", got, initialHigh)
		}
	})
}
