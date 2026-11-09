// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

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

	Describe("tick", func() {
		ctx := context.Background()

		// failingCollector reads a cluster whose Flux list calls fail.
		failingCollector := func() *Collector {
			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
				ociRepositoryGVR: "OCIRepositoryList",
				helmReleaseGVR:   "HelmReleaseList",
			})
			dyn.PrependReactor("list", "*", func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("connection refused")
			})

			return &Collector{Dynamic: dyn, Namespace: "tenant-a"}
		}

		check := staticCheck{typ: "FluxCrds", status: metav1.ConditionTrue, reason: solarv1alpha1.CheckReasonPresent}

		It("reports the collection error and its checks on the first tick", func() {
			pub := &recordingPublisher{}
			a := &Agent{Collector: failingCollector(), Checks: []Check{check}, Publisher: pub, MaxReportAge: time.Hour}

			a.tick(ctx)

			Expect(pub.reports).To(HaveLen(1))
			Expect(pub.reports[0].Spec.ReleasesError).To(ContainSubstring("connection refused"))
			Expect(pub.reports[0].Spec.Preflight.Checks).NotTo(BeEmpty())
			Expect(pub.reports[0].Spec.Releases).To(BeEmpty())
		})

		// Resending the last good releases would make a broken read look healthy.
		It("sends the error rather than the old releases on a later tick", func() {
			pub := &recordingPublisher{}
			good := solarv1alpha1.TargetReport{Spec: solarv1alpha1.TargetReportSpec{
				Releases: []solarv1alpha1.ReleaseReport{{Name: "app", Phase: solarv1alpha1.ReleaseReady}},
			}}
			a := &Agent{
				Collector: failingCollector(), Checks: []Check{check}, Publisher: pub, MaxReportAge: time.Hour,
				last: &good, lastPublished: time.Now(),
			}

			a.tick(ctx)

			Expect(pub.reports).To(HaveLen(1))
			Expect(pub.reports[0].Spec.ReleasesError).NotTo(BeEmpty())
			Expect(pub.reports[0].Spec.Releases).To(BeEmpty())
		})
	})
})

type recordingPublisher struct {
	reports []solarv1alpha1.TargetReport
}

func (p *recordingPublisher) Publish(_ context.Context, r solarv1alpha1.TargetReport) error {
	p.reports = append(p.reports, r)

	return nil
}

type staticCheck struct {
	typ    string
	status metav1.ConditionStatus
	reason string
}

func (c staticCheck) Type() string { return c.typ }

func (c staticCheck) Run(context.Context) (metav1.ConditionStatus, string, string) {
	return c.status, c.reason, ""
}
