// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ocmv2

import (
	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	descv2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	ociaccessv1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
	"ocm.software/open-component-model/bindings/go/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	// repoBaseURL is "<host>/<namespace>": host and namespace deliberately
	// differ so tests can tell which of the two a form resolves against.
	repoBaseURL = "zot.example.com:443/my-namespace"
	component   = "opendefense.cloud/ocm-demo"
)

// resource builds a descriptor resource carrying access.
func resource(name, version string, access runtime.Typed) descruntime.Resource {
	res := descruntime.Resource{Access: access} //nolint:modernize
	res.Name = name
	res.Version = version

	return res
}

// raw wraps an access payload the way a repository hands back a type its scheme
// does not know.
func raw(typeName, payload string) *runtime.Raw {
	return &runtime.Raw{
		Type: runtime.NewVersionedType(typeName, "v1"),
		Data: []byte(payload),
	}
}

var _ = Describe("ResolveOCIReference", func() {
	It("returns an absolute OCIImage reference unchanged", func() {
		res := resource("nginx", "1.28.3", &ociaccessv1.OCIImage{
			ImageReference: "ghcr.io/linuxserver/nginx:1.28.3",
		})

		ref, ok, err := ResolveOCIReference(res, repoBaseURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(ref.Host).To(Equal("ghcr.io"))
		Expect(ref.Repository).To(Equal("linuxserver/nginx"))
		Expect(ref.Tag).To(Equal("1.28.3"))
	})

	It("strips a scheme from an absolute reference, as OCM v2 transfers emit one", func() {
		res := resource("nginx", "1.28.3", &ociaccessv1.OCIImage{
			ImageReference: "https://localhost:4443/testv2oci/linuxserver/nginx:1.28.3",
		})

		ref, ok, err := ResolveOCIReference(res, repoBaseURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(ref.Host).To(Equal("localhost:4443"))
		Expect(ref.Repository).To(Equal("testv2oci/linuxserver/nginx"))
	})

	It("refuses a referenceName, because a transfer publishes nothing at it", func() {
		// What a `--copy-resources` transfer without `--upload-as ociArtifact`
		// records. The referenceName looks like a location, but measured against
		// a new namespace the transfer leaves only
		// "<ns>/component-descriptors/<component>" — so resolving it hands out a
		// reference that fails at pull time with "not found".
		res := resource("nginx", "1.28.3", raw("LocalBlob",
			`{"type":"LocalBlob/v1","localReference":"sha256:20cfa1fc5858147b702ab226b637beb19d9373dc909096bdc783693a542101c3","mediaType":"application/vnd.oci.image.index.v1+json","referenceName":"linuxserver/nginx:1.28.3"}`))

		_, ok, err := ResolveOCIReference(res, repoBaseURL)
		Expect(err).To(MatchError(ContainSubstring("--upload-as ociArtifact")))
		Expect(ok).To(BeFalse())
	})

	It("refuses OCI content held as a bare local blob", func() {
		res := resource("demo-chart", "v0.1.0", raw("LocalBlob",
			`{"type":"LocalBlob/v1","localReference":"sha256:f97b44d5868e173c0c2dd6d50be832c0ddaf47a59c781edb33d9cc098eb1c217","mediaType":"application/vnd.oci.image.manifest.v1+json"}`))

		_, ok, err := ResolveOCIReference(res, repoBaseURL)
		Expect(err).To(MatchError(ContainSubstring("not pullable")))
		Expect(ok).To(BeFalse())
	})

	It("reports no reference for a local-only blob that is not an OCI artifact", func() {
		// The values-template resource: a plain text layer, addressable only as a
		// blob inside the component. Skipping it is correct.
		res := resource("helm-values-template", "v0.1.0-v2", raw("LocalBlob",
			`{"type":"LocalBlob/v1","localReference":"sha256:21138a198f8328c5c87a2cedb32263deb187c956722e377d27fecc48fab896a7","mediaType":"text/plain"}`))

		_, ok, err := ResolveOCIReference(res, repoBaseURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	It("reports no reference for a typed LocalBlob carrying only a digest", func() {
		res := resource("blob", "v1", &descv2.LocalBlob{
			Type:           runtime.NewVersionedType("LocalBlob", "v1"),
			LocalReference: "sha256:20cfa1fc5858147b702ab226b637beb19d9373dc909096bdc783693a542101c3",
			MediaType:      "application/octet-stream",
		})

		_, ok, err := ResolveOCIReference(res, repoBaseURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	It("reports no reference for an unrelated access type", func() {
		res := resource("helm-repo", "1.0.0", raw("helm", `{"type":"helm/v1","helmRepository":"https://charts.example.com"}`))

		_, ok, err := ResolveOCIReference(res, repoBaseURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

})

var _ = Describe("SplitScheme", func() {
	It("selects plain HTTP only for an http scheme", func() {
		base, plain := SplitScheme("http://zot.local:5000/ns")
		Expect(base).To(Equal("zot.local:5000/ns"))
		Expect(plain).To(BeTrue())

		base, plain = SplitScheme("https://zot.local/ns")
		Expect(base).To(Equal("zot.local/ns"))
		Expect(plain).To(BeFalse())

		base, plain = SplitScheme("zot.local/ns")
		Expect(base).To(Equal("zot.local/ns"))
		Expect(plain).To(BeFalse())
	})
})

var _ = Describe("reanchoring transferred references", func() {
	It("corrects the host of an absolute reference under the repository namespace", func() {
		// `transfer cv --copy-resources --upload-as ociArtifact` publishes the
		// image as a real tagged artifact but records the hostname it was pushed
		// through — here a port-forward.
		res := resource("nginx", "1.28.3", &ociaccessv1.OCIImage{
			ImageReference: "https://localhost:4443/my-namespace/linuxserver/nginx:1.28.3",
		})

		ref, ok, err := ResolveOCIReference(res, repoBaseURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(ref.Host).To(Equal("zot.example.com:443"), "host should be re-anchored to the registry being read")
		Expect(ref.Repository).To(Equal("my-namespace/linuxserver/nginx"))
		Expect(ref.Tag).To(Equal("1.28.3"))
	})

	It("leaves a genuinely external reference alone", func() {
		res := resource("nginx", "1.28.3", &ociaccessv1.OCIImage{
			ImageReference: "ghcr.io/linuxserver/nginx:1.28.3",
		})

		ref, ok, err := ResolveOCIReference(res, repoBaseURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(ref.Host).To(Equal("ghcr.io"))
		Expect(ref.Repository).To(Equal("linuxserver/nginx"))
	})

	It("does not re-anchor when the repository sits at the registry root", func() {
		// Without a namespace there is nothing to distinguish a copied reference
		// from an upstream one.
		res := resource("nginx", "1.28.3", &ociaccessv1.OCIImage{
			ImageReference: "ghcr.io/linuxserver/nginx:1.28.3",
		})

		ref, ok, err := ResolveOCIReference(res, "zot.example.com:443")
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(ref.Host).To(Equal("ghcr.io"))
	})
})
