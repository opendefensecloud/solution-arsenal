// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// Check reasons, all PascalCase as the report API requires.
const (
	reasonPresent     = "Present"
	reasonMissing     = "Missing"
	reasonUnavailable = "Unavailable"
)

// Check is one preflight gate. Every check in the report feeds the Target's
// PreflightReady aggregate, so a check that cannot be evaluated reports Unknown
// rather than failing the gate.
type Check interface {
	Type() string
	Run(ctx context.Context) (status metav1.ConditionStatus, reason, message string)
}

// runChecks evaluates every check into the report, sorted by Type so equal
// reports compare equal.
func runChecks(ctx context.Context, checks []Check) []solarv1alpha1.PreflightCheck {
	out := make([]solarv1alpha1.PreflightCheck, 0, len(checks))

	for _, check := range checks {
		status, reason, message := check.Run(ctx)
		out = append(out, solarv1alpha1.PreflightCheck{
			Type:    check.Type(),
			Status:  status,
			Reason:  reason,
			Message: message,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })

	return out
}
