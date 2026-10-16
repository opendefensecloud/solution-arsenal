// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"time"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Agent", func() {
	Describe("changed", func() {
		report := solarv1alpha1.TargetReport{
			Spec: solarv1alpha1.TargetReportSpec{
				Releases: []solarv1alpha1.ReleaseReport{{Name: "app", Phase: solarv1alpha1.ReleaseReady}},
			},
		}

		It("pushes the first report", func() {
			a := &Agent{MaxReportAge: time.Hour}
			Expect(a.changed(report)).To(BeTrue())
		})

		It("stays quiet while nothing changes and the report is still fresh", func() {
			a := &Agent{MaxReportAge: time.Hour, last: &report, lastPublished: time.Now()}
			Expect(a.changed(report)).To(BeFalse())
		})

		It("pushes when a release changes phase", func() {
			a := &Agent{MaxReportAge: time.Hour, last: &report, lastPublished: time.Now()}

			degraded := solarv1alpha1.TargetReport{
				Spec: solarv1alpha1.TargetReportSpec{
					Releases: []solarv1alpha1.ReleaseReport{{Name: "app", Phase: solarv1alpha1.ReleaseDegraded}},
				},
			}
			Expect(a.changed(degraded)).To(BeTrue())
		})

		// Without this the heartbeat never fires on a stable cluster and every
		// quiet Target looks dead.
		It("pushes an unchanged report once it is older than MaxReportAge", func() {
			a := &Agent{
				MaxReportAge:  time.Minute,
				last:          &report,
				lastPublished: time.Now().Add(-2 * time.Minute),
			}
			Expect(a.changed(report)).To(BeTrue())
		})
	})
})
