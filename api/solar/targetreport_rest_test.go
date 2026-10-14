// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar_test

import (
	"context"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.opendefense.cloud/solar/api/solar"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// validTargetReport returns a report that passes Validate.
func validTargetReport() *solar.TargetReport {
	return &solar.TargetReport{
		Spec: solar.TargetReportSpec{
			LastReportTime: metav1.Now(),
			Preflight: solar.PreflightReport{
				Checks: []solar.PreflightCheck{
					{Type: "Crds", Status: metav1.ConditionTrue, Reason: "Present"},
					{Type: "Capacity", Status: metav1.ConditionFalse, Reason: "Insufficient", Message: "cpu allocatable below required"},
				},
			},
			Releases: []solar.ReleaseReport{
				{Name: "guestbook", Namespace: "workloads", Phase: solar.ReleaseReady},
			},
		},
	}
}

var _ = Describe("TargetReport REST", func() {
	Describe("Validate (create path)", func() {
		It("accepts a valid report", func() {
			Expect(validTargetReport().Validate(context.Background())).To(BeEmpty())
		})

		It("rejects a missing lastReportTime", func() {
			tr := validTargetReport()
			tr.Spec.LastReportTime = metav1.Time{}
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.lastReportTime"))
		})

		It("rejects a non-PascalCase check type", func() {
			tr := validTargetReport()
			tr.Spec.Preflight.Checks[0].Type = "crds"
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.preflight.checks[0].type"))
		})

		It("rejects duplicate check types", func() {
			tr := validTargetReport()
			tr.Spec.Preflight.Checks[1].Type = "Crds"
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.preflight.checks[1].type"))
		})

		It("rejects duplicate release name/namespace pairs", func() {
			tr := validTargetReport()
			tr.Spec.Releases = append(tr.Spec.Releases, solar.ReleaseReport{Name: "guestbook", Namespace: "workloads", Phase: solar.ReleaseReady})
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.releases[1].name"))
		})

		It("rejects a check status outside True/False/Unknown", func() {
			tr := validTargetReport()
			tr.Spec.Preflight.Checks[0].Status = metav1.ConditionStatus("Sometimes")
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.preflight.checks[0].status"))
		})

		It("rejects a check reason that is not PascalCase", func() {
			tr := validTargetReport()
			tr.Spec.Preflight.Checks[0].Reason = "not valid!"
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.preflight.checks[0].reason"))
		})

		It("rejects a missing check reason", func() {
			tr := validTargetReport()
			tr.Spec.Preflight.Checks[0].Reason = ""
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.preflight.checks[0].reason"))
		})

		It("rejects a check message longer than 4096 bytes", func() {
			tr := validTargetReport()
			tr.Spec.Preflight.Checks[0].Message = strings.Repeat("m", 4097)
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.preflight.checks[0].message"))
		})

		It("rejects a release with an empty name", func() {
			tr := validTargetReport()
			tr.Spec.Releases[0].Name = ""
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.releases[0].name"))
		})

		It("rejects a release with an empty namespace", func() {
			tr := validTargetReport()
			tr.Spec.Releases[0].Namespace = ""
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.releases[0].namespace"))
		})

		It("rejects an unsupported release phase", func() {
			tr := validTargetReport()
			tr.Spec.Releases[0].Phase = solar.ReleasePhase("Exploded")
			errs := tr.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.releases[0].phase"))
		})

		It("accepts an empty report with no checks", func() {
			tr := validTargetReport()
			tr.Spec.Preflight = solar.PreflightReport{}
			tr.Spec.Releases = nil
			Expect(tr.Validate(context.Background())).To(BeEmpty())
		})
	})

	Describe("ValidateUpdate (update path)", func() {
		It("rejects the same invalid state as Validate", func() {
			old := validTargetReport()
			updated := old.DeepCopy()
			updated.Spec.Preflight.Checks[0].Type = "crds"

			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.preflight.checks[0].type"))
		})

		It("rejects a missing lastReportTime", func() {
			old := validTargetReport()
			updated := old.DeepCopy()
			updated.Spec.LastReportTime = metav1.Time{}

			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.lastReportTime"))
		})

		It("rejects a lastReportTime that does not advance", func() {
			old := validTargetReport()
			old.Spec.LastReportTime = metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
			updated := old.DeepCopy()
			updated.Spec.LastReportTime = old.Spec.LastReportTime

			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.lastReportTime"))
		})

		It("rejects a lastReportTime older than the previous one", func() {
			old := validTargetReport()
			old.Spec.LastReportTime = metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
			updated := old.DeepCopy()
			updated.Spec.LastReportTime = metav1.NewTime(old.Spec.LastReportTime.Add(-time.Second))

			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.lastReportTime"))
		})

		It("accepts a valid update", func() {
			old := validTargetReport()
			old.Spec.LastReportTime = metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
			updated := old.DeepCopy()
			updated.Spec.LastReportTime = metav1.NewTime(old.Spec.LastReportTime.Add(time.Second))

			Expect(updated.ValidateUpdate(context.Background(), old)).To(BeEmpty())
		})
	})
})
