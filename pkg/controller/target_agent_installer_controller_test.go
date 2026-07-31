// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// stubAgentInstaller is a thread-safe fake whose behaviour is controlled by
// tests. The zero value succeeds silently and records every call.
type stubAgentInstaller struct {
	mu      sync.Mutex
	failErr error
	calls   []string // "<namespace>/<name>" of each Target passed to Install
}

func (s *stubAgentInstaller) Install(_ context.Context, _ *rest.Config, target *solarv1alpha1.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, target.Namespace+"/"+target.Name)

	return s.failErr
}

func (s *stubAgentInstaller) failWith(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failErr = err
}

func (s *stubAgentInstaller) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.calls)
}

const validKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.invalid:6443
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: test
current-context: test
users:
- name: test
  user:
    token: fake-token
`

var _ = Describe("TargetAgentInstallerReconciler", Ordered, func() {
	BeforeEach(func() {
		fakeAgentInstaller.mu.Lock()
		fakeAgentInstaller.failErr = nil
		fakeAgentInstaller.calls = nil
		fakeAgentInstaller.mu.Unlock()
	})

	It("does nothing for a target without AgentAccessSecretRef", func() {
		target := &solarv1alpha1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "no-remote-access", Namespace: ns.Name},
			Spec:       solarv1alpha1.TargetSpec{RenderRegistryRef: corev1.LocalObjectReference{Name: "reg"}},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())

		Consistently(func() int { return fakeAgentInstaller.callCount() }).Should(Equal(0))
	})

	It("installs the agent and sets AgentInstalled=True once a valid kubeconfig secret exists", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "remote-kubeconfig", Namespace: ns.Name},
			Data:       map[string][]byte{"kubeconfig": []byte(validKubeconfig)},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		target := &solarv1alpha1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "with-remote-access", Namespace: ns.Name},
			Spec: solarv1alpha1.TargetSpec{
				RenderRegistryRef:    corev1.LocalObjectReference{Name: "reg"},
				AgentAccessSecretRef: &corev1.LocalObjectReference{Name: "remote-kubeconfig"},
			},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())

		Eventually(func() bool {
			got := &solarv1alpha1.Target{}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), got); err != nil {
				return false
			}

			return apimeta.IsStatusConditionTrue(got.Status.Conditions, ConditionTypeAgentInstalled)
		}).Should(BeTrue())

		Expect(fakeAgentInstaller.callCount()).To(Equal(1))
	})

	It("sets AgentInstalled=False with reason InstallFailed when the installer errors", func() {
		fakeAgentInstaller.failWith(errors.New("boom"))

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "remote-kubeconfig-fail", Namespace: ns.Name},
			Data:       map[string][]byte{"kubeconfig": []byte(validKubeconfig)},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		target := &solarv1alpha1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "install-fails", Namespace: ns.Name},
			Spec: solarv1alpha1.TargetSpec{
				RenderRegistryRef:    corev1.LocalObjectReference{Name: "reg"},
				AgentAccessSecretRef: &corev1.LocalObjectReference{Name: "remote-kubeconfig-fail"},
			},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())

		Eventually(func() *metav1.Condition {
			got := &solarv1alpha1.Target{}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(target), got); err != nil {
				return nil
			}

			return apimeta.FindStatusCondition(got.Status.Conditions, ConditionTypeAgentInstalled)
		}).Should(SatisfyAll(
			Not(BeNil()),
			HaveField("Status", metav1.ConditionFalse),
			HaveField("Reason", "InstallFailed"),
		))
	})
})
