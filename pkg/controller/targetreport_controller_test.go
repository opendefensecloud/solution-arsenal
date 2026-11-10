// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"time"

	corev1 "k8s.io/api/core/v1"
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
			agg := apimeta.FindStatusCondition(getTarget("tr-sync").Status.Conditions, solarv1alpha1.ConditionTypePreflightReady)
			g.Expect(agg).NotTo(BeNil())
			g.Expect(agg.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(agg.Reason).To(Equal(solarv1alpha1.ReasonAllChecksPassed))
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
			agg := apimeta.FindStatusCondition(getTarget("tr-before-target").Status.Conditions, solarv1alpha1.ConditionTypePreflightReady)
			g.Expect(agg).NotTo(BeNil())
			g.Expect(agg.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(agg.Reason).To(Equal(solarv1alpha1.ReasonAllChecksPassed))
		}, eventuallyTimeout).Should(Succeed())
	})

	Describe("agent liveness", func() {
		agentReporting := func(name string) *metav1.Condition {
			return apimeta.FindStatusCondition(getTarget(name).Status.Conditions, solarv1alpha1.ConditionTypeAgentReporting)
		}

		expectAgentReporting := func(name string, status metav1.ConditionStatus, reason string) {
			GinkgoHelper()
			Eventually(func(g Gomega) {
				cond := agentReporting(name)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(status))
				g.Expect(cond.Reason).To(Equal(reason))
			}, eventuallyTimeout).Should(Succeed())
		}

		// heartbeat moves only lastReportTime forward, as the agent does when
		// nothing changed.
		heartbeat := func(name string) {
			GinkgoHelper()
			Eventually(func() error {
				latest := &solarv1alpha1.TargetReport{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns.Name}, latest); err != nil {
					return err
				}
				latest.Spec.LastReportTime = metav1.Now()

				return k8sClient.Update(ctx, latest)
			}, eventuallyTimeout).Should(Succeed())
		}

		setup := func(name string) {
			GinkgoHelper()
			target := newTarget(name)
			Expect(k8sClient.Create(ctx, target)).To(Succeed())
			DeferCleanup(cleanup, target)

			report := newReport(name, solarv1alpha1.PreflightCheck{
				Type: "FluxCrds", Status: metav1.ConditionTrue, Reason: solarv1alpha1.CheckReasonPresent,
			})
			Expect(k8sClient.Create(ctx, report)).To(Succeed())
			DeferCleanup(cleanup, report)
		}

		It("reports the agent as reporting", func() {
			setup("tr-live")
			expectAgentReporting("tr-live", metav1.ConditionTrue, solarv1alpha1.ReasonReporting)
		})

		It("goes stale without a new report", func() {
			setup("tr-stale")
			expectAgentReporting("tr-stale", metav1.ConditionTrue, solarv1alpha1.ReasonReporting)
			expectAgentReporting("tr-stale", metav1.ConditionFalse, solarv1alpha1.ReasonStale)
		})

		It("recovers when a fresh report arrives", func() {
			setup("tr-recover")
			expectAgentReporting("tr-recover", metav1.ConditionFalse, solarv1alpha1.ReasonStale)

			heartbeat("tr-recover")
			expectAgentReporting("tr-recover", metav1.ConditionTrue, solarv1alpha1.ReasonReporting)
		})

		// skewed creates a report stamped by an agent whose clock is off by skew.
		skewed := func(name string, skew time.Duration) {
			GinkgoHelper()
			target := newTarget(name)
			Expect(k8sClient.Create(ctx, target)).To(Succeed())
			DeferCleanup(cleanup, target)

			report := newReport(name)
			report.Spec.LastReportTime = metav1.NewTime(time.Now().Add(skew))
			Expect(k8sClient.Create(ctx, report)).To(Succeed())
			DeferCleanup(cleanup, report)
		}

		It("goes stale for a silent agent whose clock is far ahead", func() {
			skewed("tr-clock-ahead", 2*time.Hour)
			expectAgentReporting("tr-clock-ahead", metav1.ConditionTrue, solarv1alpha1.ReasonReporting)
			expectAgentReporting("tr-clock-ahead", metav1.ConditionFalse, solarv1alpha1.ReasonStale)
		})

		It("reports an agent whose clock is far behind", func() {
			skewed("tr-clock-behind", -2*time.Hour)
			expectAgentReporting("tr-clock-behind", metav1.ConditionTrue, solarv1alpha1.ReasonReporting)
		})

		It("does not write on an unchanged heartbeat", func() {
			setup("tr-heartbeat")
			expectAgentReporting("tr-heartbeat", metav1.ConditionTrue, solarv1alpha1.ReasonReporting)
			// Let the reconcile of the create settle before taking the version.
			Consistently(func() string { return getTarget("tr-heartbeat").ResourceVersion }, consistentlyDuration/2).
				Should(Equal(getTarget("tr-heartbeat").ResourceVersion))
			rv := getTarget("tr-heartbeat").ResourceVersion

			heartbeat("tr-heartbeat")

			Consistently(func() string { return getTarget("tr-heartbeat").ResourceVersion }, consistentlyDuration).
				Should(Equal(rv))
		})

		// With credentials enabled the controller re-creates a deleted report, so
		// this Target's credentials are revoked, as when removing an agent on
		// purpose.
		It("removes the conditions when the report is deleted", func() {
			target := newTarget("tr-deleted")
			target.Annotations = map[string]string{AnnotationAgentCredential: AgentCredentialRevoked}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())
			DeferCleanup(cleanup, target)

			report := newReport("tr-deleted", solarv1alpha1.PreflightCheck{
				Type: "FluxCrds", Status: metav1.ConditionTrue, Reason: solarv1alpha1.CheckReasonPresent,
			})
			Expect(k8sClient.Create(ctx, report)).To(Succeed())
			DeferCleanup(cleanup, report)
			expectAgentReporting("tr-deleted", metav1.ConditionTrue, solarv1alpha1.ReasonReporting)

			cleanup(&solarv1alpha1.TargetReport{Name: "tr-deleted", Namespace: ns.Name})

			Eventually(func(g Gomega) {
				conds := getTarget("tr-deleted").Status.Conditions
				g.Expect(apimeta.FindStatusCondition(conds, solarv1alpha1.ConditionTypeAgentReporting)).To(BeNil())
				g.Expect(apimeta.FindStatusCondition(conds, solarv1alpha1.ConditionTypePreflightReady)).To(BeNil())
			}, eventuallyTimeout).Should(Succeed())
		})
	})

	// These Targets name a registry that doesn't exist, so TargetReconciler
	// stops before resolving bindings and leaves resolvedBindings as written here.
	Describe("binding resolution", func() {
		newUnrenderedTarget := func(name string) *solarv1alpha1.Target {
			target := newTarget(name)
			target.Spec.RenderRegistryRef.Name = "tr-no-such-registry"

			return target
		}

		newBinding := func(namespace, name, targetName string) *solarv1alpha1.ReleaseBinding {
			return &solarv1alpha1.ReleaseBinding{
				Name:      name,
				Namespace: namespace,
				Spec: solarv1alpha1.ReleaseBindingSpec{
					TargetRef:  solarv1alpha1.ObjectReference{Name: targetName, Namespace: ns.Name},
					ReleaseRef: corev1.LocalObjectReference{Name: "tr-no-such-release"},
				},
			}
		}

		setResolved := func(targetName string, entries ...solarv1alpha1.ResolvedBinding) {
			GinkgoHelper()
			Eventually(func() error {
				latest := &solarv1alpha1.Target{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: targetName, Namespace: ns.Name}, latest); err != nil {
					return err
				}
				latest.Status.ResolvedBindings = entries

				return k8sClient.Status().Update(ctx, latest)
			}, eventuallyTimeout).Should(Succeed())
		}

		deployed := func(g Gomega, namespace, name string) *metav1.Condition {
			binding := &solarv1alpha1.ReleaseBinding{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, binding)).To(Succeed())

			return apimeta.FindStatusCondition(binding.Status.Conditions, solarv1alpha1.ConditionTypeDeployed)
		}

		expectDeployed := func(namespace, name string, status metav1.ConditionStatus, reason string) {
			GinkgoHelper()
			Eventually(func(g Gomega) {
				cond := deployed(g, namespace, name)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(status))
				g.Expect(cond.Reason).To(Equal(reason))
			}, eventuallyTimeout).Should(Succeed())
		}

		create := func(obj client.Object) {
			GinkgoHelper()
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(cleanup, obj)
		}

		It("marks filtered bindings with no report at all", func() {
			// Revoked: with credentials enabled the controller would create a report.
			target := newUnrenderedTarget("tr-filtered")
			target.Annotations = map[string]string{AnnotationAgentCredential: AgentCredentialRevoked}
			create(target)
			create(newBinding(ns.Name, "tr-filtered-rb", "tr-filtered"))

			setResolved("tr-filtered", solarv1alpha1.ResolvedBinding{
				Namespace: ns.Name, Name: "tr-filtered-rb", UniqueName: "app", Filtered: true, Message: "lost to another binding",
			})

			expectDeployed(ns.Name, "tr-filtered-rb", metav1.ConditionFalse, solarv1alpha1.ReasonFiltered)
			Consistently(func() *metav1.Condition {
				return apimeta.FindStatusCondition(getTarget("tr-filtered").Status.Conditions, solarv1alpha1.ConditionTypeAgentReporting)
			}, consistentlyDuration).Should(BeNil())
		})

		It("marks a filtered binding in another namespace", func() {
			other := &corev1.Namespace{GenerateName: "tr-other-"}
			Expect(k8sClient.Create(ctx, other)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, other)

			create(newUnrenderedTarget("tr-cross"))
			create(newBinding(other.Name, "tr-cross-rb", "tr-cross"))

			setResolved("tr-cross", solarv1alpha1.ResolvedBinding{
				Namespace: other.Name, Name: "tr-cross-rb", UniqueName: "app", Filtered: true, Message: "lost",
			})

			expectDeployed(other.Name, "tr-cross-rb", metav1.ConditionFalse, solarv1alpha1.ReasonFiltered)
		})

		It("marks a binding dropped from the list as unresolved", func() {
			create(newUnrenderedTarget("tr-dropped"))
			binding := newBinding(ns.Name, "tr-dropped-rb", "tr-dropped")
			create(binding)

			// As if a report had once shown it deployed.
			Eventually(func() error {
				latest := &solarv1alpha1.ReleaseBinding{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), latest); err != nil {
					return err
				}
				apimeta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
					Type: solarv1alpha1.ConditionTypeDeployed, Status: metav1.ConditionTrue, Reason: "Ready",
				})

				return k8sClient.Status().Update(ctx, latest)
			}, eventuallyTimeout).Should(Succeed())

			// Any list change triggers a reconcile; the binding isn't in it.
			setResolved("tr-dropped", solarv1alpha1.ResolvedBinding{Namespace: ns.Name, Name: "tr-dropped-other", Message: "x"})

			expectDeployed(ns.Name, "tr-dropped-rb", metav1.ConditionUnknown, solarv1alpha1.ReasonUnresolved)
		})

		It("skips a listed binding that no longer exists", func() {
			create(newUnrenderedTarget("tr-gone"))
			create(newBinding(ns.Name, "tr-gone-live", "tr-gone"))

			setResolved("tr-gone",
				solarv1alpha1.ResolvedBinding{Namespace: ns.Name, Name: "tr-gone-deleted", UniqueName: "a", Filtered: true, Message: "lost"},
				solarv1alpha1.ResolvedBinding{Namespace: ns.Name, Name: "tr-gone-live", UniqueName: "b", Filtered: true, Message: "lost"},
			)

			expectDeployed(ns.Name, "tr-gone-live", metav1.ConditionFalse, solarv1alpha1.ReasonFiltered)
		})

		// For example a binding created before its Release.
		It("drops Unresolved once the binding is accepted", func() {
			create(newUnrenderedTarget("tr-accepted"))
			create(newBinding(ns.Name, "tr-accepted-rb", "tr-accepted"))

			setResolved("tr-accepted", solarv1alpha1.ResolvedBinding{
				Namespace: ns.Name, Name: "tr-accepted-rb", Message: "Release tr-no-such-release not found",
			})
			expectDeployed(ns.Name, "tr-accepted-rb", metav1.ConditionUnknown, solarv1alpha1.ReasonUnresolved)

			setResolved("tr-accepted", solarv1alpha1.ResolvedBinding{
				Namespace: ns.Name, Name: "tr-accepted-rb", UniqueName: "app", DesiredRevision: "v0.0.1-abc",
			})
			Eventually(func(g Gomega) {
				g.Expect(deployed(g, ns.Name, "tr-accepted-rb")).To(BeNil())
			}, eventuallyTimeout).Should(Succeed())
		})

		// The list survives early returns, so it can still name a binding that
		// was moved to another Target.
		It("ignores a listed binding that now names another Target", func() {
			create(newUnrenderedTarget("tr-moved"))
			create(newBinding(ns.Name, "tr-moved-rb", "tr-moved-elsewhere"))

			setResolved("tr-moved", solarv1alpha1.ResolvedBinding{
				Namespace: ns.Name, Name: "tr-moved-rb", UniqueName: "app", Filtered: true, Message: "lost",
			})

			Consistently(func(g Gomega) {
				g.Expect(deployed(g, ns.Name, "tr-moved-rb")).To(BeNil())
			}, consistentlyDuration).Should(Succeed())
		})

		// An existing cluster right after the upgrade: a report, no list yet.
		It("does not change anything else without a resolved list", func() {
			create(newUnrenderedTarget("tr-unlisted"))
			create(newBinding(ns.Name, "tr-unlisted-rb", "tr-unlisted"))
			create(newReport("tr-unlisted", solarv1alpha1.PreflightCheck{
				Type: "FluxCrds", Status: metav1.ConditionTrue, Reason: solarv1alpha1.CheckReasonPresent,
			}))

			Eventually(func(g Gomega) {
				cond := apimeta.FindStatusCondition(getTarget("tr-unlisted").Status.Conditions, solarv1alpha1.ConditionTypeAgentReporting)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}, eventuallyTimeout).Should(Succeed())
			Consistently(func(g Gomega) {
				g.Expect(deployed(g, ns.Name, "tr-unlisted-rb")).To(BeNil())
			}, consistentlyDuration).Should(Succeed())
		})
	})
})
