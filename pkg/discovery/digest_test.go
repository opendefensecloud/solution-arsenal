// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResolveManifestDigest", func() {
	var (
		host string
		reg  *solarv1alpha1.Registry
	)

	push := func(repo, tag string) string {
		img, err := random.Image(64, 1)
		Expect(err).NotTo(HaveOccurred())
		ref, err := name.ParseReference(fmt.Sprintf("%s/%s:%s", host, repo, tag), name.Insecure)
		Expect(err).NotTo(HaveOccurred())
		Expect(remote.Write(ref, img)).To(Succeed())
		d, err := img.Digest()
		Expect(err).NotTo(HaveOccurred())

		return d.String()
	}

	BeforeEach(func() {
		srv := httptest.NewServer(registry.New())
		DeferCleanup(srv.Close)
		host = strings.TrimPrefix(srv.URL, "http://")
		reg = &solarv1alpha1.Registry{Name: "r", Spec: solarv1alpha1.RegistrySpec{Hostname: host, PlainHTTP: true}}
	})

	It("resolves a plain semver tag", func() {
		want := push("test/component-descriptors/example.com/a", "1.0.0")
		got, err := ResolveManifestDigest(context.Background(), reg, nil, "test/component-descriptors/example.com/a", "1.0.0")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(want))
	})

	It("maps build metadata '+' to OCM's '.build-' tag", func() {
		want := push("test/component-descriptors/example.com/a", "1.0.0.build-abc")
		got, err := ResolveManifestDigest(context.Background(), reg, nil, "test/component-descriptors/example.com/a", "1.0.0+abc")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(want))
	})

	It("fails for a missing tag", func() {
		push("test/component-descriptors/example.com/a", "1.0.0")
		_, err := ResolveManifestDigest(context.Background(), reg, nil, "test/component-descriptors/example.com/a", "9.9.9")
		Expect(err).To(HaveOccurred())
	})
})
