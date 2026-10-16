// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// startTestOCIRegistry starts an in-process OCI registry for the current spec
// and returns its host:port.
func startTestOCIRegistry() string {
	srv := httptest.NewServer(ggcrregistry.New())
	DeferCleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	Expect(err).NotTo(HaveOccurred())

	return u.Host
}

// pushTestDescriptor writes a random image to host/repo:tag and returns its
// manifest digest and the image, so a test can push it again later.
func pushTestDescriptor(host, repo, tag string) (string, v1.Image) {
	img, err := random.Image(64, 1)
	Expect(err).NotTo(HaveOccurred())
	ref, err := name.ParseReference(fmt.Sprintf("%s/%s:%s", host, repo, tag), name.Insecure)
	Expect(err).NotTo(HaveOccurred())
	Expect(remote.Write(ref, img)).To(Succeed())
	d, err := img.Digest()
	Expect(err).NotTo(HaveOccurred())

	return d.String(), img
}

func deleteTestManifest(host, repo, digest string) {
	ref, err := name.ParseReference(fmt.Sprintf("%s/%s@%s", host, repo, digest), name.Insecure)
	Expect(err).NotTo(HaveOccurred())
	Expect(remote.Delete(ref)).To(Succeed())
}

// availableOf returns "<status>/<reason>" of the CV's Available condition, or "" if unset.
func availableOf(g Gomega, key client.ObjectKey) string {
	cv := &solarv1alpha1.ComponentVersion{}
	g.Expect(k8sClient.Get(ctx, key, cv)).To(Succeed())
	c := apimeta.FindStatusCondition(cv.Status.Conditions, solarv1alpha1.ConditionTypeAvailable)
	if c == nil {
		return ""
	}

	return string(c.Status) + "/" + c.Reason
}

var _ = Describe("ComponentVersionReconciler", Ordered, func() {
	var (
		validCV = func(name string, componentName string) *solarv1alpha1.ComponentVersion {
			return &solarv1alpha1.ComponentVersion{
				Name:      name,
				Namespace: ns.Name,
				Spec: solarv1alpha1.ComponentVersionSpec{
					ComponentName: componentName,
					Tag:           "v1.0.0",
				},
			}
		}
	)

	Describe("self-finalizer", func() {
		It("adds componentVersionFinalizer to a live ComponentVersion", func() {
			cv := validCV("cvf-cv-add", "example.com/cvf")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, cv, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, cv))
			})

			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.ComponentVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(componentVersionFinalizer))
			}, eventuallyTimeout).Should(Succeed())
		})

		It("removes componentVersionFinalizer when the ComponentVersion is deleted", func() {
			cv := validCV("cvf-cv-del", "example.com/cvf")
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())

			Eventually(func(g Gomega) {
				updated := &solarv1alpha1.ComponentVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), updated)).To(Succeed())
				g.Expect(updated.Finalizers).To(ContainElement(componentVersionFinalizer))
			}, eventuallyTimeout).Should(Succeed())

			Expect(k8sClient.Delete(ctx, cv)).To(Succeed())

			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(cv), &solarv1alpha1.ComponentVersion{}))
			}, eventuallyTimeout).Should(BeTrue())
		})
	})

	Describe("availability", func() {
		const component = "example.com/avail"
		var host string

		availCV := func(name, registryHost, digest string) *solarv1alpha1.ComponentVersion {
			cv := validCV(name, component)
			cv.Spec.Scheme = "http"
			cv.Spec.Registry = registryHost
			cv.Spec.Repository = "test/" + component
			cv.Spec.Digest = digest

			return cv
		}
		createCV := func(cv *solarv1alpha1.ComponentVersion) {
			Expect(k8sClient.Create(ctx, cv)).To(Succeed())
			DeferCleanup(func() {
				patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
				_ = client.IgnoreNotFound(k8sClient.Patch(ctx, cv, patch))
				_ = client.IgnoreNotFound(k8sClient.Delete(ctx, cv))
			})
		}
		createRegistry := func(regName, hostname string) {
			reg := &solarv1alpha1.Registry{Name: regName, Namespace: ns.Name,
				Spec: solarv1alpha1.RegistrySpec{Hostname: hostname, PlainHTTP: true}}
			Expect(k8sClient.Create(ctx, reg)).To(Succeed())
			DeferCleanup(func() { _ = client.IgnoreNotFound(k8sClient.Delete(ctx, reg)) })
		}

		BeforeEach(func() {
			host = startTestOCIRegistry()
			createRegistry("avail-reg", host)
		})

		It("marks a CV whose manifest resolves as Available=True", func() {
			digest, _ := pushTestDescriptor(host, "test/component-descriptors/"+component, "v1.0.0")
			cv := availCV("avail-true", host, digest)
			createCV(cv)

			Eventually(func(g Gomega) string { return availableOf(g, client.ObjectKeyFromObject(cv)) }).
				Should(Equal("True/" + solarv1alpha1.ReasonDescriptorAvailable))
		})

		It("goes pending, then red after the grace period, never deletes, and recovers when the artifact returns", func() {
			repo := "test/component-descriptors/" + component
			digest, img := pushTestDescriptor(host, repo, "v1.0.0")
			cv := availCV("avail-cycle", host, digest)
			createCV(cv)
			key := client.ObjectKeyFromObject(cv)
			Eventually(func(g Gomega) string { return availableOf(g, key) }).Should(Equal("True/" + solarv1alpha1.ReasonDescriptorAvailable))

			deleteTestManifest(host, repo, digest)
			Eventually(func(g Gomega) string { return availableOf(g, key) }).Should(Equal("Unknown/" + solarv1alpha1.ReasonDescriptorMissingPending))
			Eventually(func(g Gomega) string { return availableOf(g, key) }).Should(Equal("False/" + solarv1alpha1.ReasonDescriptorMissing))
			Consistently(func() error { return k8sClient.Get(ctx, key, &solarv1alpha1.ComponentVersion{}) }).Should(Succeed())

			// Re-push by digest: the test registry's DELETE removes only the digest entry,
			// so a tag push would see the surviving tag and skip the upload.
			ref, err := name.ParseReference(fmt.Sprintf("%s/%s@%s", host, repo, digest), name.Insecure)
			Expect(err).NotTo(HaveOccurred())
			Expect(remote.Write(ref, img)).To(Succeed())
			Eventually(func(g Gomega) string { return availableOf(g, key) }).Should(Equal("True/" + solarv1alpha1.ReasonDescriptorAvailable))
		})

		It("finds a CV by the spec.digest field selector", func() {
			digest := "sha256:" + strings.Repeat("c", 64)
			match := availCV("avail-fs-match", host, digest)
			createCV(match)
			createCV(availCV("avail-fs-other", host, "sha256:"+strings.Repeat("d", 64)))

			list := &solarv1alpha1.ComponentVersionList{}
			Expect(k8sClient.List(ctx, list, client.InNamespace(ns.Name), client.MatchingFields{"spec.digest": digest})).To(Succeed())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].Name).To(Equal(match.Name))
		})

		It("reports NoDigest for a CV without spec.digest", func() {
			cv := availCV("avail-nodigest", host, "")
			createCV(cv)
			Eventually(func(g Gomega) string { return availableOf(g, client.ObjectKeyFromObject(cv)) }).
				Should(Equal("Unknown/" + solarv1alpha1.ReasonNoDigest))
		})

		It("reports RegistryNotConfigured when no Registry serves the CV's host", func() {
			cv := availCV("avail-noreg", "nowhere.invalid:5000", "sha256:"+strings.Repeat("a", 64))
			createCV(cv)
			Eventually(func(g Gomega) string { return availableOf(g, client.ObjectKeyFromObject(cv)) }).
				Should(Equal("Unknown/" + solarv1alpha1.ReasonRegistryNotConfigured))
		})

		It("reports RegistryUnreachable, never False, when the registry errors", func() {
			broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			DeferCleanup(broken.Close)
			u, err := url.Parse(broken.URL)
			Expect(err).NotTo(HaveOccurred())
			createRegistry("avail-broken", u.Host)

			cv := availCV("avail-broken", u.Host, "sha256:"+strings.Repeat("b", 64))
			createCV(cv)
			key := client.ObjectKeyFromObject(cv)
			Eventually(func(g Gomega) string { return availableOf(g, key) }).Should(Equal("Unknown/" + solarv1alpha1.ReasonRegistryUnreachable))
			Consistently(func(g Gomega) string { return availableOf(g, key) }, 4*time.Second).
				ShouldNot(HavePrefix("False/"))
		})
	})
})
