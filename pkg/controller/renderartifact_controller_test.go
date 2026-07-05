// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package controller

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// callRecord records a single DeleteTag invocation.
type callRecord struct {
	rawRef   string
	insecure bool
}

// stubTagDeleter is a thread-safe fake whose behaviour is controlled by tests.
// The zero value succeeds silently.
type stubTagDeleter struct {
	mu         sync.Mutex
	failErr    error        // if non-nil, DeleteTag returns this error
	calledWith []callRecord // invocations passed to DeleteTag
}

func (s *stubTagDeleter) DeleteTag(_ context.Context, rawRef string, _ authn.Authenticator, insecure bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calledWith = append(s.calledWith, callRecord{rawRef: rawRef, insecure: insecure})

	return s.failErr
}

// failWith makes the stub return err on the next call(s).
func (s *stubTagDeleter) failWith(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failErr = err
}

// calls returns a copy of all refs that were passed to DeleteTag.
func (s *stubTagDeleter) calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calledWith))
	for i, c := range s.calledWith {
		out[i] = c.rawRef
	}

	return out
}

// callsWithOpts returns a copy of all call records.
func (s *stubTagDeleter) callsWithOpts() []callRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]callRecord, len(s.calledWith))
	copy(out, s.calledWith)

	return out
}

// reset clears the recorded calls and removes any configured failure.
func (s *stubTagDeleter) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failErr = nil
	s.calledWith = nil
}

var _ = Describe("RenderArtifactController", Ordered, func() {
	// Helper: build a minimal RenderArtifact in the current test namespace.
	newArtifact := func(name string) *solarv1alpha1.RenderArtifact {
		return &solarv1alpha1.RenderArtifact{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns.Name,
			},
			Spec: solarv1alpha1.RenderArtifactSpec{
				BaseURL:        "registry.example.com",
				Repository:     "ns/myapp",
				Tag:            "v1.0.0",
				RenderTaskRef:  "rt-" + name,
				RegistryFlavor: "zot",
			},
		}
	}

	// Helper: build a RenderBinding that points to an artifact.
	newBinding := func(name, artifactName string) *solarv1alpha1.RenderBinding {
		return &solarv1alpha1.RenderBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns.Name,
			},
			Spec: solarv1alpha1.RenderBindingSpec{
				RenderArtifactRef: corev1.LocalObjectReference{Name: artifactName},
				OwnerKind:         "Target",
				OwnerName:         "test-target",
				OwnerNamespace:    ns.Name,
			},
		}
	}

	Context("finalizer lifecycle", Label("renderartifact"), func() {
		It("should add the finalizer to a new RenderArtifact", func() {
			art := newArtifact("art-finalizer")
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			// Create a binding immediately so the artifact is not GC'd before we
			// can observe the finalizer being added.
			binding := newBinding("binding-finalizer", "art-finalizer")
			Expect(k8sClient.Create(ctx, binding)).To(Succeed())

			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Finalizers).To(ContainElement(renderArtifactFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})
	})

	Context("status.ChartURL population", Label("renderartifact"), func() {
		It("should set status.ChartURL from spec coordinates", func() {
			art := newArtifact("art-charturl")
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			// Hold a binding so the artifact is not GC'd before we observe the status.
			binding := newBinding("binding-charturl", "art-charturl")
			Expect(k8sClient.Create(ctx, binding)).To(Succeed())

			expectedURL := renderChartURL("registry.example.com", "ns/myapp", "v1.0.0")
			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Status.ChartURL).To(Equal(expectedURL))
			}, eventuallyTimeout).Should(Succeed())
		})
	})

	Context("GC: no RenderBindings", Label("renderartifact"), func() {
		It("should delete the RenderArtifact when no RenderBindings reference it", func() {
			art := newArtifact("art-gc-no-bindings")
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			// With no bindings, the controller should GC the artifact.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue(), "RenderArtifact should be garbage-collected")
		})

		It("should call the injected DeleteTag function", func() {
			art := newArtifact("art-gc-deleter-called")
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue())

			expectedRef := "registry.example.com/ns/myapp:v1.0.0"
			Expect(fakeTagDeleter.calls()).To(ContainElement(expectedRef))
		})
	})

	Context("GC: with RenderBindings", Label("renderartifact"), func() {
		It("should NOT delete the RenderArtifact while RenderBindings reference it", func() {
			art := newArtifact("art-gc-has-binding")
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			// Create a binding before the reconciler can GC the artifact.
			binding := newBinding("binding-keeps-alive", "art-gc-has-binding")
			Expect(k8sClient.Create(ctx, binding)).To(Succeed())

			// Wait for finalizer to be set (reconciler has run at least once and seen the binding).
			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Finalizers).To(ContainElement(renderArtifactFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Artifact should persist as long as the binding exists.
			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return err == nil
			}, consistentlyDuration).Should(BeTrue())

			// Delete the binding -> controller should now GC the artifact.
			Expect(k8sClient.Delete(ctx, binding)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue(), "RenderArtifact should be GC'd after last binding is removed")
		})
	})

	Context("resolveAuth: cross-namespace push secret", Label("renderartifact"), func() {
		It("should read the secret from PushSecretNamespace when set", func() {
			// Create a separate namespace to host the push secret.
			crossNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "cross-ns-"},
			}
			Expect(k8sClient.Create(ctx, crossNs)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, crossNs) })

			// Secret exists only in crossNs, not in ns.Name.
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "push-creds",
					Namespace: crossNs.Name,
				},
				Type: corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("user"),
					corev1.BasicAuthPasswordKey: []byte("pass"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			reconciler := &RenderArtifactReconciler{Client: k8sClient}

			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{Name: "art-crossns-auth", Namespace: ns.Name},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:             "registry.example.com",
					Repository:          "ns/myapp",
					Tag:                 "v1.0.0",
					PushSecretRef:       &corev1.LocalObjectReference{Name: "push-creds"},
					PushSecretNamespace: crossNs.Name,
				},
			}
			auth, err := reconciler.resolveAuth(ctx, art, art.Spec.BaseURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(auth).NotTo(Equal(authn.Anonymous))

			artSameNs := art.DeepCopy()
			artSameNs.Spec.PushSecretNamespace = ""
			_, err = reconciler.resolveAuth(ctx, artSameNs, artSameNs.Spec.BaseURL)
			Expect(err).To(HaveOccurred(), "secret does not exist in art's own namespace, should return error")
		})
	})

	Context("OCI delete failure surfaces as condition", Label("renderartifact"), func() {
		It("should set OCICleanup=False condition and keep the finalizer when DeleteTag fails", func() {
			art := newArtifact("art-oci-fail")
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			// Hold a binding so the artifact is not GC'd before we inject the failure.
			binding := newBinding("binding-oci-fail", "art-oci-fail")
			Expect(k8sClient.Create(ctx, binding)).To(Succeed())

			// Wait for the finalizer to be set so we know the reconciler has run.
			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Finalizers).To(ContainElement(renderArtifactFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			// Inject a failure so the next OCI delete attempt fails.
			deleteErr := errors.New("registry temporarily unavailable")
			fakeTagDeleter.failWith(deleteErr)

			// Remove the binding -> controller should now GC the artifact
			// which sets DeletionTimestamp and enters the finalizer path,
			// which calls DeleteTag and hits the injected failure.
			Expect(k8sClient.Delete(ctx, binding)).To(Succeed())

			// Expect the OCICleanup=False condition to be set on the artifact.
			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				cond := apimeta.FindStatusCondition(a.Status.Conditions, ConditionTypeOCICleanup)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("DeleteFailed"))
				g.Expect(cond.Message).To(ContainSubstring("registry temporarily unavailable"))
			}, eventuallyTimeout).Should(Succeed())

			// Finalizer must still be present (deletion must be blocked).
			a := &solarv1alpha1.RenderArtifact{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
			Expect(a.Finalizers).To(ContainElement(renderArtifactFinalizer))

			// Let the delete succeed -> finalizer removed -> object disappears.
			fakeTagDeleter.reset()

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue(), "RenderArtifact should be deleted after OCI cleanup succeeds")
		})
	})

	Context("GC: OCI 404 treated as already deleted", Label("renderartifact"), func() {
		It("should delete the RenderArtifact normally when DeleteTag returns 404", func() {
			art := newArtifact("art-oci-404")
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			// Configure the stub to return a 404 transport error.
			fakeTagDeleter.failWith(&transport.Error{StatusCode: http.StatusNotFound})
			DeferCleanup(func() { fakeTagDeleter.reset() })

			// controller should GC the artifact. The 404 from DeleteTag
			// must be treated as "already gone" and not block deletion.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue(), "RenderArtifact should be GC'd even when DeleteTag returns 404")
		})
	})

	Context("GC with PlainHTTP", Label("renderartifact"), func() {
		It("should pass Insecure=true to DeleteTag when PlainHTTP is set on the artifact", func() {
			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "art-plainhttp",
					Namespace: ns.Name,
				},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:       "registry.example.com",
					Repository:    "ns/myapp",
					Tag:           "v1.0.0",
					RenderTaskRef: "rt-plainhttp",
					PlainHTTP:     true,
				},
			}
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			// No bindings → GC should delete the artifact and call DeleteTag.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue())

			records := fakeTagDeleter.callsWithOpts()
			Expect(records).NotTo(BeEmpty())
			for _, rec := range records {
				if rec.rawRef == "registry.example.com/ns/myapp:v1.0.0" {
					Expect(rec.insecure).To(BeTrue(), "PlainHTTP artifact should delete with Insecure=true")
					return
				}
			}
			Fail("expected DeleteTag call for registry.example.com/ns/myapp:v1.0.0")
		})

		It("should pass Insecure=false to DeleteTag when PlainHTTP is not set", func() {
			art := newArtifact("art-secure-default")
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue())

			records := fakeTagDeleter.callsWithOpts()
			Expect(records).NotTo(BeEmpty())
			for _, rec := range records {
				if rec.rawRef == "registry.example.com/ns/myapp:v1.0.0" {
					Expect(rec.insecure).To(BeFalse(), "default artifact should delete with Insecure=false")
					return
				}
			}
			Fail("expected DeleteTag call for registry.example.com/ns/myapp:v1.0.0")
		})
	})
})
