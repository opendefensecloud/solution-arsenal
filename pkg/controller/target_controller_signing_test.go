// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"slices"
	"testing"

	"github.com/sigstore/cosign/v3/pkg/cosign"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solaradmission "go.opendefense.cloud/solar/pkg/admission"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TargetController signing key", func() {
	var agentRelease, plainRelease *solarv1alpha1.Release

	// The namespace is recreated for every spec, so the fixtures are too.
	BeforeEach(func() {
		Expect(k8sClient.Create(ctx, &solarv1alpha1.Registry{
			Name: "signing-registry", Namespace: ns.Name,
			Spec: solarv1alpha1.RegistrySpec{
				Hostname:       "registry.example.com",
				SolarSecretRef: &corev1.LocalObjectReference{Name: "registry-credentials"},
			},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &solarv1alpha1.ComponentVersion{
			Name: "signing-cv", Namespace: ns.Name,
			Spec: solarv1alpha1.ComponentVersionSpec{
				ComponentName: "signing-component",
				Tag:           "v1.0.0",
				Resources: map[string]solarv1alpha1.ResourceAccess{
					"chart": {Repository: "example.com/resources/chart", Tag: "1.0.0"},
				},
				Entrypoint: solarv1alpha1.Entrypoint{ResourceName: "chart", Type: solarv1alpha1.EntrypointTypeHelm},
			},
		})).To(Succeed())

		newRel := func(name string, annotations map[string]string) *solarv1alpha1.Release {
			rel := &solarv1alpha1.Release{
				Name: name, Namespace: ns.Name, Annotations: annotations,
				Spec: solarv1alpha1.ReleaseSpec{
					ComponentVersionRef: solarv1alpha1.ObjectReference{Name: "signing-cv"},
					UniqueName:          name,
					Values:              runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
				},
			}
			Expect(k8sClient.Create(ctx, rel)).To(Succeed())

			return rel
		}
		agentRelease = newRel("agent", map[string]string{solaradmission.AnnotationTargetCredential: "true"})
		plainRelease = newRel("plain", nil)
	})

	newKey := func() []byte {
		kb, err := cosign.GenerateKeyPair(func(bool) ([]byte, error) { return []byte(testKeyPassword), nil })
		Expect(err).NotTo(HaveOccurred())

		return kb.PrivateBytes
	}

	createKeySecret := func(name string, key []byte, password string) *corev1.Secret {
		s := &corev1.Secret{
			Name: name, Namespace: ns.Name,
			Data: map[string][]byte{signingKeyField: key, signingPasswordField: []byte(password)},
		}
		Expect(k8sClient.Create(ctx, s)).To(Succeed())

		return s
	}

	// createTarget binds a new Target to rel. An empty secretName leaves it unsigned.
	createTarget := func(name, secretName string, rel *solarv1alpha1.Release) {
		t := &solarv1alpha1.Target{
			Name: name, Namespace: ns.Name,
			Spec: solarv1alpha1.TargetSpec{
				RenderRegistryRef: solarv1alpha1.ObjectReference{Name: "signing-registry"},
				Userdata:          runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
			},
		}
		if secretName != "" {
			t.Spec.Signing = &solarv1alpha1.TargetSigning{KeySecretRef: corev1.LocalObjectReference{Name: secretName}}
		}
		Expect(k8sClient.Create(ctx, t)).To(Succeed())
		Expect(k8sClient.Create(ctx, &solarv1alpha1.ReleaseBinding{
			Name: name + "-" + rel.Name, Namespace: ns.Name,
			Spec: solarv1alpha1.ReleaseBindingSpec{
				TargetRef:  solarv1alpha1.ObjectReference{Name: name},
				ReleaseRef: corev1.LocalObjectReference{Name: rel.Name},
			},
		})).To(Succeed())
	}

	setSigning := func(name string, signing *solarv1alpha1.TargetSigning) {
		Eventually(func(g Gomega) {
			t := &solarv1alpha1.Target{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: name}, t)).To(Succeed())
			t.Spec.Signing = signing
			g.Expect(k8sClient.Update(ctx, t)).To(Succeed())
		}, eventuallyTimeout).Should(Succeed())
	}

	signingCondition := func(g Gomega, name string) *metav1.Condition {
		t := &solarv1alpha1.Target{}
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: name}, t)).To(Succeed())

		return apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeSigningKeyResolved)
	}

	expectCondition := func(name string, status metav1.ConditionStatus, reason string) *metav1.Condition {
		var cond *metav1.Condition
		EventuallyWithOffset(1, func(g Gomega) {
			cond = signingCondition(g, name)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(status))
			g.Expect(cond.Reason).To(Equal(reason))
		}, eventuallyTimeout).Should(Succeed())

		return cond
	}

	ownedTasks := func(g Gomega, name string) []solarv1alpha1.RenderTask {
		list := &solarv1alpha1.RenderTaskList{}
		g.Expect(k8sClient.List(ctx, list, client.InNamespace(ns.Name))).To(Succeed())

		var owned []solarv1alpha1.RenderTask
		for _, rt := range list.Items {
			if rt.Spec.OwnerName == name {
				owned = append(owned, rt)
			}
		}

		return owned
	}

	// signedName is what releaseRenderTaskName gets as its target argument for a signed Target.
	signedName := func(target string, key *corev1.Secret) string {
		return target + "@" + signingKeyTag(key)
	}

	// getRT reads a release RenderTask from inside an Eventually(func(g Gomega)).
	getRT := func(g Gomega, rtTarget string, rel *solarv1alpha1.Release) *solarv1alpha1.RenderTask {
		rt := &solarv1alpha1.RenderTask{}
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{
			Namespace: ns.Name,
			Name:      releaseRenderTaskName(rel.Namespace, rel.Name, rtTarget, rel.GetGeneration()),
		}, rt)).To(Succeed())

		return rt
	}

	markTask := func(rtTarget string, rel *solarv1alpha1.Release, condType, reason string) {
		Eventually(func(g Gomega) {
			rt := getRT(g, rtTarget, rel)
			apimeta.SetStatusCondition(&rt.Status.Conditions, metav1.Condition{
				Type: condType, Status: metav1.ConditionTrue, Reason: reason,
			})
			rt.Status.ChartURL = fmt.Sprintf("oci://registry.example.com/%s:%s", rt.Spec.Repository, rt.Spec.Tag)
			g.Expect(k8sClient.Status().Update(ctx, rt)).To(Succeed())
		}, eventuallyTimeout).Should(Succeed())
	}

	getBootstrapRT := func(g Gomega, target string, version int64) *solarv1alpha1.RenderTask {
		rt := &solarv1alpha1.RenderTask{}
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{
			Namespace: ns.Name,
			Name:      targetRenderTaskName(target, version),
		}, rt)).To(Succeed())

		return rt
	}

	It("reports a missing key and creates no RenderTask", func() {
		createTarget("tc-missing", "no-such-secret", plainRelease)

		cond := expectCondition("tc-missing", metav1.ConditionFalse, ReasonNotFound)
		Expect(cond.Message).To(ContainSubstring("no-such-secret"))
		Consistently(func(g Gomega) {
			g.Expect(ownedTasks(g, "tc-missing")).To(BeEmpty())
		}, consistentlyDuration).Should(Succeed())
	})

	It("recovers when the Secret appears", func() {
		createTarget("tc-appears", "late-key", plainRelease)
		expectCondition("tc-appears", metav1.ConditionFalse, ReasonNotFound)

		createKeySecret("late-key", newKey(), testKeyPassword)

		expectCondition("tc-appears", metav1.ConditionTrue, ReasonResolved)
		Eventually(func(g Gomega) {
			g.Expect(ownedTasks(g, "tc-appears")).NotTo(BeEmpty())
		}, eventuallyTimeout).Should(Succeed())
	})

	It("accepts a wrong password and leaves it to the renderer", func() {
		s := createKeySecret("wrong-pw-key", newKey(), "wrong-password")
		createTarget("tc-wrong-pw", "wrong-pw-key", plainRelease)

		expectCondition("tc-wrong-pw", metav1.ConditionTrue, ReasonResolved)
		Eventually(func(g Gomega) {
			g.Expect(getRT(g, signedName("tc-wrong-pw", s), plainRelease).Spec.SigningKeySecretRef).
				To(Equal(&corev1.LocalObjectReference{Name: "wrong-pw-key"}))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("creates a new task when the password is fixed", func() {
		s := createKeySecret("fix-pw-key", newKey(), "wrong-password")
		createTarget("tc-fix-pw", "fix-pw-key", plainRelease)
		var old *solarv1alpha1.RenderTask
		Eventually(func(g Gomega) {
			old = getRT(g, signedName("tc-fix-pw", s), plainRelease)
		}, eventuallyTimeout).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(s), s)).To(Succeed())
			s.Data[signingPasswordField] = []byte(testKeyPassword)
			g.Expect(k8sClient.Update(ctx, s)).To(Succeed())
		}, eventuallyTimeout).Should(Succeed())

		Eventually(func(g Gomega) {
			rt := getRT(g, signedName("tc-fix-pw", s), plainRelease)
			g.Expect(rt.Name).NotTo(Equal(old.Name))
			g.Expect(rt.Spec.Tag).NotTo(Equal(old.Spec.Tag))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("reports an invalid key and recovers once fixed", func() {
		s := createKeySecret("bad-key", []byte("not-a-key"), testKeyPassword)
		createTarget("tc-invalid", "bad-key", plainRelease)

		expectCondition("tc-invalid", metav1.ConditionFalse, ReasonInvalidSigningKey)
		Consistently(func(g Gomega) {
			g.Expect(ownedTasks(g, "tc-invalid")).To(BeEmpty())
		}, consistentlyDuration).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(s), s)).To(Succeed())
			s.Data[signingKeyField] = newKey()
			g.Expect(k8sClient.Update(ctx, s)).To(Succeed())
		}, eventuallyTimeout).Should(Succeed())

		expectCondition("tc-invalid", metav1.ConditionTrue, ReasonResolved)
		Eventually(func(g Gomega) {
			g.Expect(getRT(g, signedName("tc-invalid", s), plainRelease).Spec.SigningKeySecretRef).
				To(Equal(&corev1.LocalObjectReference{Name: "bad-key"}))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("does not leak key material in the condition", func() {
		createKeySecret("not-a-key", []byte("do-not-print-this-key"), "do-not-print-this-password")
		createTarget("tc-leak-key", "not-a-key", plainRelease)

		cond := expectCondition("tc-leak-key", metav1.ConditionFalse, ReasonInvalidSigningKey)
		Expect(cond.Message).NotTo(ContainSubstring("do-not-print-this"))
	})

	It("signs nothing and adds no condition without spec.signing", func() {
		createTarget("tc-unsigned", "", plainRelease)

		Eventually(func(g Gomega) {
			rt := getRT(g, "tc-unsigned", plainRelease)
			g.Expect(rt.Spec.SigningKeySecretRef).To(BeNil())
			g.Expect(rt.Spec.Tag).To(Equal(fmt.Sprintf("v0.0.%d-%s", plainRelease.GetGeneration(),
				pullSecretsTag(rt.Spec.ReleaseConfig.Input.Resources, rt.Spec.ReleaseConfig.Input.PullSecrets))))
			g.Expect(signingCondition(g, "tc-unsigned")).To(BeNil())
		}, eventuallyTimeout).Should(Succeed())

		markTask("tc-unsigned", plainRelease, ConditionTypeJobSucceeded, ReasonJobSucceeded)
		Eventually(func(g Gomega) {
			g.Expect(getBootstrapRT(g, "tc-unsigned", 0).Spec.SigningKeySecretRef).To(BeNil())
		}, eventuallyTimeout).Should(Succeed())
	})

	It("sets the ref and key tag on release and bootstrap tasks", func() {
		s := createKeySecret("release-key", newKey(), testKeyPassword)
		createTarget("tc-signed", "release-key", plainRelease)
		ref := &corev1.LocalObjectReference{Name: "release-key"}

		Eventually(func(g Gomega) {
			rt := getRT(g, signedName("tc-signed", s), plainRelease)
			g.Expect(rt.Spec.SigningKeySecretRef).To(Equal(ref))
			g.Expect(rt.Spec.Tag).To(ContainSubstring("-" + signingKeyTag(s)))
		}, eventuallyTimeout).Should(Succeed())

		markTask(signedName("tc-signed", s), plainRelease, ConditionTypeJobSucceeded, ReasonJobSucceeded)

		Eventually(func(g Gomega) {
			g.Expect(getBootstrapRT(g, "tc-signed", 0).Spec.SigningKeySecretRef).To(Equal(ref))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("keeps the credential tag last", func() {
		s := createKeySecret("agent-key", newKey(), testKeyPassword)
		createTarget("tc-agent-signed", "agent-key", agentRelease)

		Eventually(func(g Gomega) {
			rt := getRT(g, signedName("tc-agent-signed", s), agentRelease)
			target := &solarv1alpha1.Target{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "tc-agent-signed", Namespace: ns.Name}, target)).To(Succeed())
			cur, err := currentAgentCredential(ctx, k8sClient, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cur).NotTo(BeNil())
			g.Expect(rt.Spec.Tag).To(HaveSuffix("-" + signingKeyTag(s) + "-" +
				agentConnectionTag("https://solar.example:6443", []byte("TEST-CA")) + "-" + credentialTag(cur)))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("gives different keys different tags", func() {
		a := createKeySecret("key-a", newKey(), testKeyPassword)
		b := createKeySecret("key-b", newKey(), testKeyPassword)
		createTarget("tc-diff-a", "key-a", plainRelease)
		createTarget("tc-diff-b", "key-b", plainRelease)

		Eventually(func(g Gomega) {
			rtA, rtB := getRT(g, signedName("tc-diff-a", a), plainRelease), getRT(g, signedName("tc-diff-b", b), plainRelease)
			g.Expect(rtA.Spec.Repository).To(Equal(rtB.Spec.Repository), "precondition: shared repository")
			g.Expect(rtA.Spec.Tag).NotTo(Equal(rtB.Spec.Tag))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("shares a tag for the same key", func() {
		key := newKey()
		a := createKeySecret("copy-a", key, testKeyPassword)
		b := createKeySecret("copy-b", key, testKeyPassword)
		createTarget("tc-same-a", "copy-a", plainRelease)
		createTarget("tc-same-b", "copy-b", plainRelease)

		Eventually(func(g Gomega) {
			rtA, rtB := getRT(g, signedName("tc-same-a", a), plainRelease), getRT(g, signedName("tc-same-b", b), plainRelease)
			g.Expect(rtA.Spec.Tag).To(Equal(rtB.Spec.Tag))
			g.Expect(rtA.Name).NotTo(Equal(rtB.Name))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("re-renders under a new task when a key is added to a failed task", func() {
		createTarget("tc-failed", "", plainRelease)
		var old *solarv1alpha1.RenderTask
		Eventually(func(g Gomega) {
			old = getRT(g, "tc-failed", plainRelease)
		}, eventuallyTimeout).Should(Succeed())
		markTask("tc-failed", plainRelease, ConditionTypeJobFailed, ReasonJobFailed)
		Eventually(func(g Gomega) {
			t := &solarv1alpha1.Target{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "tc-failed"}, t)).To(Succeed())
			cond := apimeta.FindStatusCondition(t.Status.Conditions, ConditionTypeReleasesRendered)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Reason).To(Equal(ReasonReleaseFailed))
		}, eventuallyTimeout).Should(Succeed())

		s := createKeySecret("failed-key", newKey(), testKeyPassword)
		setSigning("tc-failed", &solarv1alpha1.TargetSigning{KeySecretRef: corev1.LocalObjectReference{Name: "failed-key"}})

		Eventually(func(g Gomega) {
			rt := getRT(g, signedName("tc-failed", s), plainRelease)
			g.Expect(rt.Name).NotTo(Equal(old.Name))
			g.Expect(rt.Spec.SigningKeySecretRef).To(Equal(&corev1.LocalObjectReference{Name: "failed-key"}))
		}, eventuallyTimeout).Should(Succeed())

		// The old name was not taken over: what still holds it is the old, unsigned task.
		same := getRT(Default, "tc-failed", plainRelease)
		Expect(same.UID).To(Equal(old.UID))
		Expect(same.Spec.SigningKeySecretRef).To(BeNil())
	})

	It("bumps the bootstrap version when a key is added", func() {
		createTarget("tc-bump", "", plainRelease)
		markTask("tc-bump", plainRelease, ConditionTypeJobSucceeded, ReasonJobSucceeded)
		Eventually(func(g Gomega) {
			g.Expect(getBootstrapRT(g, "tc-bump", 0).Spec.SigningKeySecretRef).To(BeNil())
		}, eventuallyTimeout).Should(Succeed())

		s := createKeySecret("bump-key", newKey(), testKeyPassword)
		setSigning("tc-bump", &solarv1alpha1.TargetSigning{KeySecretRef: corev1.LocalObjectReference{Name: "bump-key"}})
		markTask(signedName("tc-bump", s), plainRelease, ConditionTypeJobSucceeded, ReasonJobSucceeded)

		Eventually(func(g Gomega) {
			g.Expect(getBootstrapRT(g, "tc-bump", 1).Spec.SigningKeySecretRef).
				To(Equal(&corev1.LocalObjectReference{Name: "bump-key"}))
			t := &solarv1alpha1.Target{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "tc-bump"}, t)).To(Succeed())
			g.Expect(t.Status.BootstrapVersion).To(Equal(int64(1)))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("drops the condition and returns to unsigned when spec.signing is removed", func() {
		s := createKeySecret("removed-key", newKey(), testKeyPassword)
		createTarget("tc-removed", "removed-key", plainRelease)
		expectCondition("tc-removed", metav1.ConditionTrue, ReasonResolved)
		Eventually(func(g Gomega) {
			g.Expect(getRT(g, signedName("tc-removed", s), plainRelease).Spec.SigningKeySecretRef).NotTo(BeNil())
		}, eventuallyTimeout).Should(Succeed())

		setSigning("tc-removed", nil)

		Eventually(func(g Gomega) {
			g.Expect(signingCondition(g, "tc-removed")).To(BeNil())
			rt := getRT(g, "tc-removed", plainRelease)
			g.Expect(rt.Spec.SigningKeySecretRef).To(BeNil())
			g.Expect(rt.Spec.Tag).NotTo(ContainSubstring(signingKeyTag(s)))
		}, eventuallyTimeout).Should(Succeed())

		// Removing the condition writes once, it must not keep the Target reconciling.
		resourceVersion := func(g Gomega) string {
			t := &solarv1alpha1.Target{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "tc-removed"}, t)).To(Succeed())

			return t.ResourceVersion
		}
		var settled string
		Eventually(func(g Gomega) {
			previous := settled
			settled = resourceVersion(g)
			g.Expect(settled).To(Equal(previous))
		}, eventuallyTimeout).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(resourceVersion(g)).To(Equal(settled))
		}, consistentlyDuration).Should(Succeed())
	})
})

func TestMapSigningKeySecretToTargets(t *testing.T) {
	t.Parallel()

	target := func(namespace, name, secretName string) *solarv1alpha1.Target {
		tgt := &solarv1alpha1.Target{Name: name, Namespace: namespace}
		if secretName != "" {
			tgt.Spec.Signing = &solarv1alpha1.TargetSigning{KeySecretRef: corev1.LocalObjectReference{Name: secretName}}
		}

		return tgt
	}
	c := fake.NewClientBuilder().
		WithScheme(newEnsureRenderArtifactScheme(t)).
		WithObjects(
			target("a", "first", "key"),
			target("a", "second", "key"),
			target("a", "other-key", "other"),
			target("a", "unsigned", ""),
			target("b", "elsewhere", "key"),
		).
		Build()
	r := &TargetReconciler{Client: c}

	for _, tc := range []struct {
		desc, namespace, name string
		want                  []string
	}{
		{"only referencing Targets in the namespace", "a", "key", []string{"a/first", "a/second"}},
		{"a same-named Secret in another namespace", "b", "key", []string{"b/elsewhere"}},
		{"a same-named Secret where no Target references it", "c", "key", nil},
		{"an unrelated Secret", "a", "unrelated", nil},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			var got []string
			for _, req := range r.mapSigningKeySecretToTargets(t.Context(), &corev1.Secret{Name: tc.name, Namespace: tc.namespace}) {
				got = append(got, req.Namespace+"/"+req.Name)
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
