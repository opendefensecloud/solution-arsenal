// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TargetReportReconciler", Ordered, func() {
	var (
		newTarget = func(name string) *solarv1alpha1.Target {
			return &solarv1alpha1.Target{
				Name:      name,
				Namespace: ns.Name,
				Spec: solarv1alpha1.TargetSpec{
					RenderRegistryRef: solarv1alpha1.ObjectReference{Name: "test-registry"},
				},
			}
		}

		newReport = func(name string, checks ...solarv1alpha1.PreflightCheck) *solarv1alpha1.TargetReport {
			return &solarv1alpha1.TargetReport{
				Name:      name,
				Namespace: ns.Name,
				Spec: solarv1alpha1.TargetReportSpec{
					LastReportTime: metav1.Now(),
					Preflight:      solarv1alpha1.PreflightReport{Checks: checks},
				},
			}
		}

		getTarget = func(name string) *solarv1alpha1.Target {
			latest := &solarv1alpha1.Target{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns.Name}, latest)).To(Succeed())

			return latest
		}

		// cleanup force-removes finalizers so namespace teardown never hangs.
		cleanup = func(obj client.Object) {
			patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
			_ = client.IgnoreNotFound(k8sClient.Patch(ctx, obj, patch))
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, obj))
		}
	)

	It("aggregates the report's checks onto the Target's status", func() {
		target := newTarget("tr-sync")
		Expect(k8sClient.Create(ctx, target)).To(Succeed())
		DeferCleanup(cleanup, target)

		report := newReport("tr-sync", solarv1alpha1.PreflightCheck{
			Type:   "FluxCrds",
			Status: metav1.ConditionTrue,
			Reason: "Present",
		})
		Expect(k8sClient.Create(ctx, report)).To(Succeed())
		DeferCleanup(cleanup, report)

		Eventually(func(g Gomega) {
			agg := apimeta.FindStatusCondition(getTarget("tr-sync").Status.Conditions, ConditionTypePreflightReady)
			g.Expect(agg).NotTo(BeNil())
			g.Expect(agg.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(agg.Reason).To(Equal("AllChecksPassed"))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("reconciles a report created before its Target", func() {
		report := newReport("tr-before-target", solarv1alpha1.PreflightCheck{
			Type:   "FluxCrds",
			Status: metav1.ConditionTrue,
			Reason: "Present",
		})
		Expect(k8sClient.Create(ctx, report)).To(Succeed())
		DeferCleanup(cleanup, report)

		target := newTarget("tr-before-target")
		Expect(k8sClient.Create(ctx, target)).To(Succeed())
		DeferCleanup(cleanup, target)

		Eventually(func(g Gomega) {
			agg := apimeta.FindStatusCondition(getTarget("tr-before-target").Status.Conditions, ConditionTypePreflightReady)
			g.Expect(agg).NotTo(BeNil())
			g.Expect(agg.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(agg.Reason).To(Equal("AllChecksPassed"))
		}, eventuallyTimeout).Should(Succeed())
	})
})
