// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

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
				BaseURL:       "registry.example.com",
				Repository:    "ns/myapp",
				Tag:           "v1.0.0",
				RenderTaskRef: "rt-" + name,
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

	Context("repinCredentials: no binding carries a reference", Label("renderartifact"), func() {
		It("should keep the artifact's reference when no binding carries one", func() {
			// Binding first: an artifact with no binding is GC'd promptly (see the
			// "GC: no RenderBindings" context), which would race this spec.
			binding := newBinding("binding-repin-keep", "art-repin-keep")
			Expect(k8sClient.Create(ctx, binding)).To(Succeed())

			art := newArtifact("art-repin-keep")
			art.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "keep-registry"}
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			Consistently(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Spec.RegistryRef).To(Equal(&solarv1alpha1.ObjectReference{Name: "keep-registry"}))
			}, consistentlyDuration).Should(Succeed(), "a stale-but-valid ref deletes the tag, nil does not")
		})
	})

	Context("resolveAuth: RegistryRef resolution", Label("renderartifact"), func() {
		It("should resolve credentials from a same-namespace Registry", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "same-ns-creds", Namespace: ns.Name},
				Type:       corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("user"),
					corev1.BasicAuthPasswordKey: []byte("pass"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "same-ns-registry", Namespace: ns.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:       "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{Name: "same-ns-creds"},
					PlainHTTP:      true,
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			reconciler := &RenderArtifactReconciler{Client: k8sClient, APIReader: k8sClient}
			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{Name: "art-same-ns-auth", Namespace: ns.Name},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:     "registry.example.com",
					Repository:  "ns/myapp",
					Tag:         "v1.0.0",
					RegistryRef: &solarv1alpha1.ObjectReference{Name: "same-ns-registry"},
				},
			}

			auth, plainHTTP, err := reconciler.resolveAuth(ctx, art, art.Spec.BaseURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(auth).NotTo(Equal(authn.Anonymous))
			Expect(plainHTTP).To(BeTrue())
		})

		It("should refuse Registry credentials when the artifact targets a different host", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "host-scope-creds", Namespace: ns.Name},
				Type:       corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("user"),
					corev1.BasicAuthPasswordKey: []byte("pass"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "host-scope-registry", Namespace: ns.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:       "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{Name: "host-scope-creds"},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			// A hand-authored artifact aiming the Registry's credentials at another host.
			reconciler := &RenderArtifactReconciler{Client: k8sClient, APIReader: k8sClient}
			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{Name: "art-host-scope", Namespace: ns.Name},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:     "other-registry.example.com",
					Repository:  "victim/chart",
					Tag:         "v1.0.0",
					RegistryRef: &solarv1alpha1.ObjectReference{Name: "host-scope-registry"},
				},
			}

			auth, _, err := reconciler.resolveAuth(ctx, art, art.Spec.BaseURL)
			Expect(err).To(HaveOccurred(), "credentials must not be usable against a host the Registry does not serve")
			Expect(auth).To(BeNil())
		})

		It("should resolve credentials from a cross-namespace Registry when a ReferenceGrant permits it", func() {
			crossNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "cross-ns-"}}
			Expect(k8sClient.Create(ctx, crossNs)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, crossNs) })

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cross-ns-creds", Namespace: crossNs.Name},
				Type:       corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("user"),
					corev1.BasicAuthPasswordKey: []byte("pass"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "cross-ns-registry", Namespace: crossNs.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:       "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{Name: "cross-ns-creds"},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			// The grant must name RenderArtifact: the artifact's RegistryRef is meant to be
			// controller-copied from the Target, but the API lets it be authored, so cleanup
			// does not ride the Target's grant.
			grant := &solarv1alpha1.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{Name: "grant", Namespace: crossNs.Name},
				Spec: solarv1alpha1.ReferenceGrantSpec{
					From: []solarv1alpha1.ReferenceGrantFromSubject{
						{Group: solarGroup, Kind: "RenderArtifact", Namespace: ns.Name},
					},
					To: []solarv1alpha1.ReferenceGrantToTarget{
						{Group: solarGroup, Kind: "Registry"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, grant)).To(Succeed())

			reconciler := &RenderArtifactReconciler{Client: k8sClient, APIReader: k8sClient}
			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{Name: "art-cross-ns-auth", Namespace: ns.Name},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:    "registry.example.com",
					Repository: "ns/myapp",
					Tag:        "v1.0.0",
					RegistryRef: &solarv1alpha1.ObjectReference{
						Name:      "cross-ns-registry",
						Namespace: crossNs.Name,
					},
				},
			}

			auth, _, err := reconciler.resolveAuth(ctx, art, art.Spec.BaseURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(auth).NotTo(Equal(authn.Anonymous))
		})

		It("should fail when no ReferenceGrant permits the cross-namespace Registry", func() {
			crossNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "cross-ns-ungranted-"}}
			Expect(k8sClient.Create(ctx, crossNs)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, crossNs) })

			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "ungranted-registry", Namespace: crossNs.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:       "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{Name: "does-not-matter"},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())
			// No ReferenceGrant created in crossNs — access must be denied.

			reconciler := &RenderArtifactReconciler{Client: k8sClient, APIReader: k8sClient}
			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{Name: "art-ungranted-auth", Namespace: ns.Name},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:    "registry.example.com",
					Repository: "ns/myapp",
					Tag:        "v1.0.0",
					RegistryRef: &solarv1alpha1.ObjectReference{
						Name:      "ungranted-registry",
						Namespace: crossNs.Name,
					},
				},
			}

			_, _, err := reconciler.resolveAuth(ctx, art, art.Spec.BaseURL)
			Expect(err).To(HaveOccurred(), "cross-namespace Registry access without a ReferenceGrant must fail")
			Expect(err.Error()).To(ContainSubstring("from[].kind=RenderArtifact"),
				"the error must name the grant kind operators actually need")
		})

		It("should fail when only a Target grant covers the cross-namespace Registry", func() {
			crossNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "cross-ns-target-grant-"}}
			Expect(k8sClient.Create(ctx, crossNs)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, crossNs) })

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "target-grant-creds", Namespace: crossNs.Name},
				Type:       corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("user"),
					corev1.BasicAuthPasswordKey: []byte("pass"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "target-grant-registry", Namespace: crossNs.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:       "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{Name: "target-grant-creds"},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			grant := &solarv1alpha1.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{Name: "target-only-grant", Namespace: crossNs.Name},
				Spec: solarv1alpha1.ReferenceGrantSpec{
					From: []solarv1alpha1.ReferenceGrantFromSubject{
						{Group: solarGroup, Kind: "Target", Namespace: ns.Name},
					},
					To: []solarv1alpha1.ReferenceGrantToTarget{
						{Group: solarGroup, Kind: "Registry"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, grant)).To(Succeed())

			// A hand-authored artifact borrowing the Target's grant to reach the Registry.
			reconciler := &RenderArtifactReconciler{Client: k8sClient, APIReader: k8sClient}
			art := &solarv1alpha1.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{Name: "art-target-grant", Namespace: ns.Name},
				Spec: solarv1alpha1.RenderArtifactSpec{
					BaseURL:    "registry.example.com",
					Repository: "victim/chart",
					Tag:        "v1.0.0",
					RegistryRef: &solarv1alpha1.ObjectReference{
						Name:      "target-grant-registry",
						Namespace: crossNs.Name,
					},
				},
			}

			auth, _, err := reconciler.resolveAuth(ctx, art, art.Spec.BaseURL)
			Expect(err).To(HaveOccurred(), "a Target-only grant must not authorize a RenderArtifact")
			Expect(auth).To(BeNil())
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
		It("should pass Insecure=true to DeleteTag when the Registry has PlainHTTP set", func() {
			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: "plainhttp-registry", Namespace: ns.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:  "registry.example.com",
					PlainHTTP: true,
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

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
					RegistryRef:   &solarv1alpha1.ObjectReference{Name: "plainhttp-registry"},
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
					Expect(rec.insecure).To(BeTrue(), "PlainHTTP Registry should delete with Insecure=true")
					return
				}
			}
			Fail("expected DeleteTag call for registry.example.com/ns/myapp:v1.0.0")
		})

		It("should pass Insecure=false to DeleteTag when the artifact has no RegistryRef", func() {
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

	Context("RegistryRef persistence", Label("renderartifact"), func() {
		It("should persist a cross-namespace RegistryRef on RenderArtifact and RenderBinding", func() {
			ref := solarv1alpha1.ObjectReference{Name: "reg-a", Namespace: "ns-a"}

			// Binding first: an artifact with zero bindings is GC'd promptly, which would
			// race the Gets below. Both carry the same ref so repinCredentials is a no-op
			binding := newBinding("binding-registryref-roundtrip", "art-registryref-roundtrip")
			binding.Spec.RegistryRef = ref.DeepCopy()
			Expect(k8sClient.Create(ctx, binding)).To(Succeed())

			art := newArtifact("art-registryref-roundtrip")
			art.Spec.RegistryRef = ref.DeepCopy()
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			gotArt := &solarv1alpha1.RenderArtifact{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), gotArt)).To(Succeed())
			Expect(gotArt.Spec.RegistryRef).To(Equal(&ref))

			gotBinding := &solarv1alpha1.RenderBinding{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), gotBinding)).To(Succeed())
			Expect(gotBinding.Spec.RegistryRef).To(Equal(&ref))
		})
	})

	Context("credential re-pinning", Label("renderartifact"), func() {
		newRegistryWithSecret := func(name, secretName string) (*solarv1alpha1.Registry, *corev1.Secret) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns.Name},
				Type:       corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("user-" + secretName),
					corev1.BasicAuthPasswordKey: []byte("pass-" + secretName),
				},
			}
			registry := &solarv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns.Name},
				Spec: solarv1alpha1.RegistrySpec{
					Hostname:       "registry.example.com",
					SolarSecretRef: &corev1.LocalObjectReference{Name: secretName},
				},
			}

			return registry, secret
		}

		It("should re-pin RegistryRef to a surviving binding when the currently-pinned binding is removed", func() {
			registryA, secretA := newRegistryWithSecret("a-registry-repin", "a-secret-repin")
			Expect(k8sClient.Create(ctx, secretA)).To(Succeed())
			Expect(k8sClient.Create(ctx, registryA)).To(Succeed())

			registryB, secretB := newRegistryWithSecret("b-registry-repin", "b-secret-repin")
			Expect(k8sClient.Create(ctx, secretB)).To(Succeed())
			Expect(k8sClient.Create(ctx, registryB)).To(Succeed())

			// Bindings are created before the artifact throughout this context: an artifact
			// that exists with no binding is GC'd promptly (see "GC: no RenderBindings"),
			// which would race every assertion below.
			bindingA := newBinding("a-binding-repin", "art-repin")
			bindingA.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "a-registry-repin"}
			Expect(k8sClient.Create(ctx, bindingA)).To(Succeed())

			bindingB := newBinding("b-binding-repin", "art-repin")
			bindingB.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "b-registry-repin"}
			Expect(k8sClient.Create(ctx, bindingB)).To(Succeed())

			art := newArtifact("art-repin")
			art.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "a-registry-repin"}
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			// Sanity: artifact starts pinned to registry A (alphabetically first binding).
			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Spec.RegistryRef.Name).To(Equal("a-registry-repin"))
			}, eventuallyTimeout).Should(Succeed())

			// Simulate Target A (and its Registry/Secret) being torn down: remove its binding.
			Expect(k8sClient.Delete(ctx, bindingA)).To(Succeed())
			Expect(k8sClient.Delete(ctx, registryA)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secretA)).To(Succeed())

			// The artifact must re-pin to the surviving binding's Registry.
			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Spec.RegistryRef.Name).To(Equal("b-registry-repin"))
			}, eventuallyTimeout).Should(Succeed(), "RenderArtifact should re-pin to the surviving binding's Registry")
		})

		It("should GC successfully using the last surviving binding's Registry instead of getting stuck terminating", func() {
			DeferCleanup(fakeTagDeleter.reset)

			registryA, secretA := newRegistryWithSecret("a-registry-gc", "a-secret-gc")
			Expect(k8sClient.Create(ctx, secretA)).To(Succeed())
			Expect(k8sClient.Create(ctx, registryA)).To(Succeed())

			registryB, secretB := newRegistryWithSecret("b-registry-gc", "b-secret-gc")
			Expect(k8sClient.Create(ctx, secretB)).To(Succeed())
			Expect(k8sClient.Create(ctx, registryB)).To(Succeed())

			bindingA := newBinding("a-binding-gc", "art-repin-gc")
			bindingA.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "a-registry-gc"}
			Expect(k8sClient.Create(ctx, bindingA)).To(Succeed())

			bindingB := newBinding("b-binding-gc", "art-repin-gc")
			bindingB.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "b-registry-gc"}
			Expect(k8sClient.Create(ctx, bindingB)).To(Succeed())

			art := newArtifact("art-repin-gc")
			art.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "a-registry-gc"}
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			// Target A's Registry+Secret go away first, while Target B is still bound —
			// this is the exact scenario that used to leave the RenderArtifact stuck.
			Expect(k8sClient.Delete(ctx, bindingA)).To(Succeed())
			Expect(k8sClient.Delete(ctx, registryA)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secretA)).To(Succeed())

			Eventually(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Spec.RegistryRef.Name).To(Equal("b-registry-gc"))
			}, eventuallyTimeout).Should(Succeed())

			// Now Target B's binding is removed too — this must GC cleanly, not get stuck.
			Expect(k8sClient.Delete(ctx, bindingB)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(art), &solarv1alpha1.RenderArtifact{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout).Should(BeTrue(), "RenderArtifact must be GC'd, not stuck terminating with a dangling RegistryRef")

			Expect(fakeTagDeleter.calls()).To(ContainElement("registry.example.com/ns/myapp:v1.0.0"))
		})

		It("should not pin nil over a working RegistryRef when a legacy binding has none", func() {
			registryB, secretB := newRegistryWithSecret("b-registry-legacy", "b-secret-legacy")
			Expect(k8sClient.Create(ctx, secretB)).To(Succeed())
			Expect(k8sClient.Create(ctx, registryB)).To(Succeed())

			// Pre-upgrade binding: sorts first by name, but carries no RegistryRef.
			legacy := newBinding("a-binding-legacy", "art-legacy-nil")
			Expect(k8sClient.Create(ctx, legacy)).To(Succeed())

			bindingB := newBinding("b-binding-legacy", "art-legacy-nil")
			bindingB.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "b-registry-legacy"}
			Expect(k8sClient.Create(ctx, bindingB)).To(Succeed())

			art := newArtifact("art-legacy-nil")
			art.Spec.RegistryRef = &solarv1alpha1.ObjectReference{Name: "b-registry-legacy"}
			Expect(k8sClient.Create(ctx, art)).To(Succeed())

			Consistently(func(g Gomega) {
				a := &solarv1alpha1.RenderArtifact{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(art), a)).To(Succeed())
				g.Expect(a.Spec.RegistryRef).NotTo(BeNil(), "legacy binding must not clear the pinned Registry")
				g.Expect(a.Spec.RegistryRef.Name).To(Equal("b-registry-legacy"))
			}, consistentlyDuration).Should(Succeed())
		})
	})
})
