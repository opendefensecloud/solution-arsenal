// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solaradmission "go.opendefense.cloud/solar/pkg/admission"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TargetController agent credential", func() {
	var agentRelease, plainRelease *solarv1alpha1.Release

	// The namespace is recreated for every spec, so the fixtures are too.
	BeforeEach(func() {
		Expect(k8sClient.Create(ctx, &solarv1alpha1.Registry{
			Name: "agentcred-registry", Namespace: ns.Name,
			Spec: solarv1alpha1.RegistrySpec{
				Hostname:       "registry.example.com",
				SolarSecretRef: &corev1.LocalObjectReference{Name: "registry-credentials"},
			},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &solarv1alpha1.ComponentVersion{
			Name: "agentcred-cv", Namespace: ns.Name,
			Spec: solarv1alpha1.ComponentVersionSpec{
				ComponentName: "agentcred-component",
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
					ComponentVersionRef: solarv1alpha1.ObjectReference{Name: "agentcred-cv"},
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

	createTargetBoundTo := func(name string, rel *solarv1alpha1.Release) {
		t := &solarv1alpha1.Target{
			Name: name, Namespace: ns.Name,
			Spec: solarv1alpha1.TargetSpec{
				RenderRegistryRef: solarv1alpha1.ObjectReference{Name: "agentcred-registry"},
				Userdata:          runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
			},
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

	rtKey := func(target string, rel *solarv1alpha1.Release) client.ObjectKey {
		return client.ObjectKey{Namespace: ns.Name,
			Name: releaseRenderTaskName(rel.Namespace, rel.Name, target, rel.GetGeneration())}
	}
	// getRT reads the RenderTask from inside an Eventually(func(g Gomega)).
	getRT := func(g Gomega, target string, rel *solarv1alpha1.Release) *solarv1alpha1.RenderTask {
		rt := &solarv1alpha1.RenderTask{}
		g.Expect(k8sClient.Get(ctx, rtKey(target, rel), rt)).To(Succeed())

		return rt
	}

	It("adds the credential reference, connection config and credential tag to an annotated Release", func() {
		createTargetBoundTo("tc-agent", agentRelease)
		Eventually(func(g Gomega) {
			rt := getRT(g, "tc-agent", agentRelease)
			cur, err := currentAgentCredential(ctx, k8sClient, &solarv1alpha1.Target{Name: "tc-agent", Namespace: ns.Name})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cur).NotTo(BeNil())
			g.Expect(rt.Spec.TargetCredentialSecretRef).To(Equal(&corev1.LocalObjectReference{Name: cur.Name}))
			g.Expect(rt.Spec.ReleaseConfig.AgentCredential).To(Equal(&solarv1alpha1.AgentCredentialConfig{
				APIServerURL: "https://solar.example:6443", CAData: []byte("TEST-CA"),
				TargetName: "tc-agent", TargetNamespace: ns.Name,
			}))
			g.Expect(rt.Spec.Tag).To(HaveSuffix("-" + credentialTag(cur)))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("leaves a Release without the annotation untouched", func() {
		createTargetBoundTo("tc-plain", plainRelease)
		Eventually(func(g Gomega) {
			rt := getRT(g, "tc-plain", plainRelease)
			g.Expect(rt.Spec.TargetCredentialSecretRef).To(BeNil())
			g.Expect(rt.Spec.ReleaseConfig.AgentCredential).To(BeNil())
		}, eventuallyTimeout).Should(Succeed())
	})

	It("gives two Targets in one namespace different agent chart tags", func() {
		createTargetBoundTo("tc-two-a", agentRelease)
		createTargetBoundTo("tc-two-b", agentRelease)
		Eventually(func(g Gomega) {
			a, b := getRT(g, "tc-two-a", agentRelease), getRT(g, "tc-two-b", agentRelease)
			g.Expect(a.Spec.TargetCredentialSecretRef).NotTo(BeNil())
			g.Expect(b.Spec.TargetCredentialSecretRef).NotTo(BeNil())
			g.Expect(a.Spec.Repository).To(Equal(b.Spec.Repository), "precondition: shared repository")
			g.Expect(a.Spec.Tag).NotTo(Equal(b.Spec.Tag))
		}, eventuallyTimeout).Should(Succeed())
	})

	It("stops rendering the credential once the Target is revoked", func() {
		createTargetBoundTo("tc-revoked", agentRelease)
		Eventually(func(g Gomega) {
			g.Expect(getRT(g, "tc-revoked", agentRelease).Spec.TargetCredentialSecretRef).NotTo(BeNil())
		}, eventuallyTimeout).Should(Succeed())

		Eventually(func(g Gomega) {
			t := &solarv1alpha1.Target{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "tc-revoked"}, t)).To(Succeed())
			if t.Annotations == nil {
				t.Annotations = map[string]string{}
			}
			t.Annotations[AnnotationAgentCredential] = AgentCredentialRevoked
			g.Expect(k8sClient.Update(ctx, t)).To(Succeed())
		}, eventuallyTimeout).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(getRT(g, "tc-revoked", agentRelease).Spec.TargetCredentialSecretRef).To(BeNil())
		}, eventuallyTimeout).Should(Succeed())
	})
})
