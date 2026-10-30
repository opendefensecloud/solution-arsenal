// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AgentCredentialReconciler", func() {
	newTarget := func(name string) *solarv1alpha1.Target {
		return &solarv1alpha1.Target{Name: name, Namespace: ns.Name,
			Spec: solarv1alpha1.TargetSpec{RenderRegistryRef: solarv1alpha1.ObjectReference{Name: "agent-cred-registry"}}}
	}
	key := func(name string) client.ObjectKey { return client.ObjectKey{Namespace: ns.Name, Name: name} }

	It("creates ServiceAccount, Role, RoleBinding, empty TargetReport and a token Secret", func() {
		t := newTarget("ac-create")
		Expect(k8sClient.Create(ctx, t)).To(Succeed())

		Eventually(func(g Gomega) {
			sa := &corev1.ServiceAccount{}
			g.Expect(k8sClient.Get(ctx, key("solar-agent-ac-create"), sa)).To(Succeed())

			role := &rbacv1.Role{}
			g.Expect(k8sClient.Get(ctx, key("solar-agent-ac-create"), role)).To(Succeed())
			g.Expect(role.Rules).To(Equal(agentRules("ac-create")))

			rb := &rbacv1.RoleBinding{}
			g.Expect(k8sClient.Get(ctx, key("solar-agent-ac-create"), rb)).To(Succeed())
			g.Expect(rb.Subjects).To(ConsistOf(rbacv1.Subject{Kind: "ServiceAccount", Name: "solar-agent-ac-create", Namespace: ns.Name}))

			report := &solarv1alpha1.TargetReport{}
			g.Expect(k8sClient.Get(ctx, key("ac-create"), report)).To(Succeed())
			g.Expect(report.Spec.LastReportTime.IsZero()).To(BeTrue())

			s := &corev1.Secret{}
			g.Expect(k8sClient.Get(ctx, key("solar-agent-ac-create-1"), s)).To(Succeed())
			g.Expect(s.Labels).To(HaveKeyWithValue(LabelAgentCredential, "ac-create"))
			g.Expect(s.Data[AgentCredentialTokenKey]).NotTo(BeEmpty())
			g.Expect(s.Annotations).To(HaveKey(AnnotationExpiresAt))

			got := &solarv1alpha1.Target{}
			g.Expect(k8sClient.Get(ctx, key("ac-create"), got)).To(Succeed())
			for _, o := range []client.Object{sa, role, rb, report, s} {
				g.Expect(metav1.IsControlledBy(o, got)).To(BeTrue(), "%T %s not owned by Target", o, o.GetName())
			}
			g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, ConditionTypeAgentCredentialReady)).To(BeTrue())
		}).Should(Succeed())
	})

	It("requests the configured token lifetime, not the one-hour default", func() {
		t := newTarget("ac-ttl")
		Expect(k8sClient.Create(ctx, t)).To(Succeed())
		Eventually(func(g Gomega) {
			s := &corev1.Secret{}
			g.Expect(k8sClient.Get(ctx, key("solar-agent-ac-ttl-1"), s)).To(Succeed())
			exp, err := time.Parse(time.RFC3339, s.Annotations[AnnotationExpiresAt])
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exp).To(BeTemporally(">", time.Now().Add(2*time.Hour)))
		}).Should(Succeed())
	})

	It("removes everything when the Target is deleted", func() {
		t := newTarget("ac-delete")
		Expect(k8sClient.Create(ctx, t)).To(Succeed())
		Eventually(func() error { return k8sClient.Get(ctx, key("solar-agent-ac-delete-1"), &corev1.Secret{}) }).Should(Succeed())

		Expect(k8sClient.Delete(ctx, t)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("ac-delete"), &solarv1alpha1.Target{}))).To(BeTrue())
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("solar-agent-ac-delete"), &corev1.ServiceAccount{}))).To(BeTrue())
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("solar-agent-ac-delete"), &rbacv1.Role{}))).To(BeTrue())
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("solar-agent-ac-delete"), &rbacv1.RoleBinding{}))).To(BeTrue())
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("ac-delete"), &solarv1alpha1.TargetReport{}))).To(BeTrue())
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("solar-agent-ac-delete-1"), &corev1.Secret{}))).To(BeTrue())
		}).Should(Succeed())
	})

	It("re-mints when the current credential Secret is deleted by hand", func() {
		t := newTarget("ac-redo")
		Expect(k8sClient.Create(ctx, t)).To(Succeed())
		s := &corev1.Secret{}
		Eventually(func() error { return k8sClient.Get(ctx, key("solar-agent-ac-redo-1"), s) }).Should(Succeed())
		Expect(k8sClient.Delete(ctx, s)).To(Succeed())
		Eventually(func(g Gomega) {
			cur, err := currentAgentCredential(ctx, k8sClient, t)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cur).NotTo(BeNil())
			g.Expect(cur.UID).NotTo(Equal(s.UID))
		}).Should(Succeed())
	})

	It("rotates on annotation and keeps the old Secret until the report shows the new revision", func() {
		t := newTarget("ac-rotate")
		Expect(k8sClient.Create(ctx, t)).To(Succeed())
		Eventually(func() error { return k8sClient.Get(ctx, key("solar-agent-ac-rotate-1"), &corev1.Secret{}) }).Should(Succeed())

		Expect(k8sClient.Get(ctx, key("ac-rotate"), t)).To(Succeed())
		t.Annotations = map[string]string{AnnotationRotateAgentCredential: "1"}
		Expect(k8sClient.Update(ctx, t)).To(Succeed())

		second := &corev1.Secret{}
		Eventually(func() error { return k8sClient.Get(ctx, key("solar-agent-ac-rotate-2"), second) }).Should(Succeed())
		// Offline target: no report yet, so the old token must stay valid.
		Consistently(func() error { return k8sClient.Get(ctx, key("solar-agent-ac-rotate-1"), &corev1.Secret{}) }).Should(Succeed())

		report := &solarv1alpha1.TargetReport{}
		Expect(k8sClient.Get(ctx, key("ac-rotate"), report)).To(Succeed())
		report.Spec.LastReportTime = metav1.Now()
		report.Spec.Releases = []solarv1alpha1.ReleaseReport{{Name: "solar-agent", Namespace: ns.Name,
			Phase: solarv1alpha1.ReleaseReady, Revision: "v0.0.1-0011aabb-" + credentialTag(second)}}
		Expect(k8sClient.Update(ctx, report)).To(Succeed())

		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, key("solar-agent-ac-rotate-1"), &corev1.Secret{}))
		}).Should(BeTrue())
		Expect(k8sClient.Get(ctx, key("solar-agent-ac-rotate-2"), &corev1.Secret{})).To(Succeed())
	})

	It("revokes: deletes all credentials, mints none, and says so", func() {
		t := newTarget("ac-revoke")
		Expect(k8sClient.Create(ctx, t)).To(Succeed())
		Eventually(func() error { return k8sClient.Get(ctx, key("solar-agent-ac-revoke-1"), &corev1.Secret{}) }).Should(Succeed())

		Expect(k8sClient.Get(ctx, key("ac-revoke"), t)).To(Succeed())
		t.Annotations = map[string]string{AnnotationAgentCredential: AgentCredentialRevoked}
		Expect(k8sClient.Update(ctx, t)).To(Succeed())

		Eventually(func(g Gomega) {
			list := &corev1.SecretList{}
			g.Expect(k8sClient.List(ctx, list, client.InNamespace(ns.Name), client.MatchingLabels{LabelAgentCredential: "ac-revoke"})).To(Succeed())
			g.Expect(list.Items).To(BeEmpty())
			got := &solarv1alpha1.Target{}
			g.Expect(k8sClient.Get(ctx, key("ac-revoke"), got)).To(Succeed())
			c := apimeta.FindStatusCondition(got.Status.Conditions, ConditionTypeAgentCredentialReady)
			g.Expect(c).NotTo(BeNil())
			g.Expect(c.Reason).To(Equal(ReasonAgentCredentialRevoked))
		}).Should(Succeed())
		Consistently(func() []corev1.Secret {
			list := &corev1.SecretList{}
			_ = k8sClient.List(ctx, list, client.InNamespace(ns.Name), client.MatchingLabels{LabelAgentCredential: "ac-revoke"})

			return list.Items
		}).Should(BeEmpty())
	})

	It("mints a new credential after the revoke is lifted", func() {
		t := newTarget("ac-unrevoke")
		t.Annotations = map[string]string{AnnotationAgentCredential: AgentCredentialRevoked}
		Expect(k8sClient.Create(ctx, t)).To(Succeed())
		Consistently(func() error { return k8sClient.Get(ctx, key("solar-agent-ac-unrevoke-1"), &corev1.Secret{}) }).ShouldNot(Succeed())

		Expect(k8sClient.Get(ctx, key("ac-unrevoke"), t)).To(Succeed())
		delete(t.Annotations, AnnotationAgentCredential)
		Expect(k8sClient.Update(ctx, t)).To(Succeed())
		Eventually(func(g Gomega) {
			cur, err := currentAgentCredential(ctx, k8sClient, t)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cur).NotTo(BeNil())
		}).Should(Succeed())
	})
})
