// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"time"

	"github.com/cenkalti/backoff/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Dependency-wait backoff curve. 5s initial catches race conditions where the
// dependency lands seconds later; the 5m cap keeps steady-state polling well
// below controller-runtime's 1000s rate-limiter cap. Jitter 0.5 avoids
// thundering herd if many Targets are waiting on the same dependency. No
// overall cap — we never give up; the dependency may appear hours later when
// an admin creates it.
const (
	dependencyWaitInitialInterval     = 5 * time.Second
	dependencyWaitMaxInterval         = 5 * time.Minute
	dependencyWaitMultiplier          = 2.0
	dependencyWaitRandomizationFactor = 0.5
)

// requeueAfterForCondition returns the RequeueAfter delay for a dependency-wait
// condition based on how long it has been in its current Status. A nil or
// zero-time condition is treated as a fresh wait, returning the initial
// (jittered) interval.
//
// The curve is derived from the condition's LastTransitionTime, which is
// already managed for us by apimeta.SetStatusCondition (it preserves the
// timestamp while Status is unchanged and resets it when Status flips). No
// new persistent state is needed.
func requeueAfterForCondition(cond *metav1.Condition, now time.Time) time.Duration {
	age := time.Duration(0)
	if cond != nil && !cond.LastTransitionTime.IsZero() {
		if delta := now.Sub(cond.LastTransitionTime.Time); delta > 0 {
			age = delta
		}
	}

	// Walk the backoff curve until the cumulative-time-elapsed-after-this-step
	// would cross `age`. That step's interval is the right next-RequeueAfter:
	//   age ∈ [0, I_0)         → return I_0
	//   age ∈ [I_0, I_0+I_1)   → return I_1
	//   age ∈ [I_0+I_1, ...)   → return I_2
	//   ...
	b := newDependencyWaitBackoff()
	elapsed := time.Duration(0)
	// Loop cap is a safety net for nonsensical ages (clock skew, manual
	// timestamp edits). Even at age=24h with a 5m cap we expect ~290 steps,
	// well under this bound.
	for range 1000 {
		next := b.NextBackOff()
		if age < elapsed+next {
			return next
		}
		elapsed += next
	}

	return dependencyWaitMaxInterval
}

// requeueAfterForConditions returns the soonest RequeueAfter across the given
// dependency-wait conditions (min of per-condition values). Nil entries are
// skipped. Returns 0 if every condition is nil — callers should only invoke
// this when at least one condition is actually waiting.
func requeueAfterForConditions(now time.Time, conds ...*metav1.Condition) time.Duration {
	var soonest time.Duration
	for _, c := range conds {
		if c == nil {
			continue
		}
		d := requeueAfterForCondition(c, now)
		if soonest == 0 || d < soonest {
			soonest = d
		}
	}

	return soonest
}

func newDependencyWaitBackoff() *backoff.ExponentialBackOff {
	return &backoff.ExponentialBackOff{
		InitialInterval:     dependencyWaitInitialInterval,
		Multiplier:          dependencyWaitMultiplier,
		MaxInterval:         dependencyWaitMaxInterval,
		RandomizationFactor: dependencyWaitRandomizationFactor,
	}
}
