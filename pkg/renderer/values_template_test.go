// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"

	"go.opendefense.cloud/ocm-kit/helmvalues"
	"ocm.software/ocm/api/ocm"
	ocmreg "ocm.software/ocm/api/ocm/extensions/repositories/ocireg"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/test"
	"go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("pullSecretsFrom", func() {
	It("keys secrets by normalised registry host", func() {
		secrets := pullSecretsFrom(solarv1alpha1.ReleaseInput{
			Resources: map[string]solarv1alpha1.ResolvedResourceAccess{
				"chart":  {Repository: "oci://registry.example.com/charts/my-chart", PullSecretName: "regcred"},
				"image":  {Repository: "Registry.Example.COM:5000/org/app", PullSecretName: "portcred"},
				"public": {Repository: "ghcr.io/org/app", PullSecretName: "ghcred"},
			},
		})

		Expect(secrets).To(Equal(helmvalues.PullSecrets{
			"registry.example.com":      "regcred",
			"registry.example.com:5000": "portcred",
			"ghcr.io":                   "ghcred",
		}))
	})

	It("omits resources whose host has no binding", func() {
		secrets := pullSecretsFrom(solarv1alpha1.ReleaseInput{
			Resources: map[string]solarv1alpha1.ResolvedResourceAccess{
				"bound":   {Repository: "registry.example.com/charts/c", PullSecretName: "regcred"},
				"unbound": {Repository: "quay.io/org/app", PullSecretName: ""},
			},
		})

		Expect(secrets).To(HaveKey("registry.example.com"))
		Expect(secrets).NotTo(HaveKey("quay.io"))
	})

	It("returns an empty map for no resources", func() {
		Expect(pullSecretsFrom(solarv1alpha1.ReleaseInput{})).To(BeEmpty())
	})

	It("includes bindings for hosts no resource references", func() {
		// A values template may call pullSecretFor on any host — a sidecar image
		// hardcoded in the chart, for instance. Deriving the map from resources
		// alone would leave such a binding unreachable.
		secrets := pullSecretsFrom(solarv1alpha1.ReleaseInput{
			Resources: map[string]solarv1alpha1.ResolvedResourceAccess{
				"chart": {Repository: "registry.example.com/charts/c", PullSecretName: "regcred"},
			},
			PullSecrets: map[string]string{
				"registry.example.com": "regcred",
				"quay.io":              "quay-cred",
			},
		})

		Expect(secrets).To(Equal(helmvalues.PullSecrets{
			"registry.example.com": "regcred",
			"quay.io":              "quay-cred",
		}))
	})

	It("drops bindings for registries without a target pull secret", func() {
		// buildPullSecretsLookup includes such registries with an empty value.
		secrets := pullSecretsFrom(solarv1alpha1.ReleaseInput{
			PullSecrets: map[string]string{
				"registry.example.com":  "regcred",
				"anonymous.example.com": "",
			},
		})

		Expect(secrets).To(HaveKey("registry.example.com"))
		Expect(secrets).NotTo(HaveKey("anonymous.example.com"))
	})

	It("lets the binding lookup win over a stale resource-derived entry", func() {
		secrets := pullSecretsFrom(solarv1alpha1.ReleaseInput{
			Resources: map[string]solarv1alpha1.ResolvedResourceAccess{
				"chart": {Repository: "registry.example.com/charts/c", PullSecretName: "old-cred"},
			},
			PullSecrets: map[string]string{"registry.example.com": "new-cred"},
		})

		Expect(secrets["registry.example.com"]).To(Equal("new-cred"))
	})
})

// These specs check that the map pullSecretsFrom builds actually resolves under
// ocm-kit's lookup rules, which walk an image reference from most specific to
// least specific. SolAr only ever populates host-granularity keys, because
// Registry.spec.hostname is host-scoped.
var _ = Describe("pullSecretsFrom + pullSecretFor", func() {
	resources := solarv1alpha1.ReleaseInput{
		Resources: map[string]solarv1alpha1.ResolvedResourceAccess{
			"image": {Repository: "registry.example.com:5000/org/app", PullSecretName: "regcred"},
		},
	}

	renderWith := func(template string, input solarv1alpha1.ReleaseInput) (string, error) {
		return helmvalues.Render(
			&helmvalues.HelmValuesTemplate{ResourceName: "t", TemplateContent: template},
			&helmvalues.RenderingInput{PullSecrets: pullSecretsFrom(input)},
		)
	}

	It("resolves a full image reference down to the host entry", func() {
		out, err := renderWith(`secret: {{ pullSecretFor "registry.example.com:5000/org/app" }}`, resources)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("secret: regcred"))
	})

	It("resolves a bare host", func() {
		out, err := renderWith(`secret: {{ pullSecretFor "registry.example.com:5000" }}`, resources)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("secret: regcred"))
	})

	It("yields empty for a host with no binding", func() {
		out, err := renderWith(`secret: {{ pullSecretFor "quay.io/org/app" }}`, resources)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("secret: "))
	})

	It("omits a guarded block entirely when the host is unbound", func() {
		const guarded = `image: app
{{- with pullSecretFor "quay.io/org/app" }}
imagePullSecrets:
  - name: {{ . }}
{{- end }}`

		out, err := renderWith(guarded, resources)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("image: app"))
		Expect(out).NotTo(ContainSubstring("imagePullSecrets"))
	})

	It("emits a guarded block when the host is bound", func() {
		const guarded = `image: app
{{- with pullSecretFor "registry.example.com:5000/org/app" }}
imagePullSecrets:
  - name: {{ . }}
{{- end }}`

		out, err := renderWith(guarded, resources)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("imagePullSecrets:"))
		Expect(out).To(ContainSubstring("- name: regcred"))
	})

	It("resolves a host that only the template names", func() {
		input := solarv1alpha1.ReleaseInput{
			Resources:   resources.Resources,
			PullSecrets: map[string]string{"quay.io": "quay-cred"},
		}

		out, err := renderWith(`secret: {{ pullSecretFor "quay.io/org/sidecar" }}`, input)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("secret: quay-cred"))
	})
})

var _ = Describe("renderValuesFrom against a transferred component", Ordered, func() {
	const (
		demoComponent = "opendefense.cloud/ocm-demo"
		demoVersion   = "v26.4.2"
		demoChart     = "demo-chart"
	)

	var (
		testServer *httptest.Server
		repo       ocm.Repository
		compVer    ocm.ComponentVersionAccess
	)

	BeforeAll(func() {
		projectDir, err := filepath.Abs("../..")
		Expect(err).NotTo(HaveOccurred())

		ctfPath := filepath.Join(projectDir, "test", "fixtures", "ocm-demo-ctf")
		Expect(ctfPath).To(BeADirectory(), "ocm-demo CTF missing; run `make ocm-transfer-demo`")

		testServer = httptest.NewServer(registry.New().HandleFunc())
		DeferCleanup(testServer.Close)

		_, err = test.Run(exec.Command(
			test.EnvName("ocm"), "transfer", "ctf", ctfPath, fmt.Sprintf("%s/test", testServer.URL),
		))
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		serverURL, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		octx := ocm.New()
		repo, err = octx.RepositoryForSpec(ocmreg.NewRepositorySpec(
			fmt.Sprintf("http://%s/test", serverURL.Host),
		))
		Expect(err).NotTo(HaveOccurred())

		compVer, err = repo.LookupComponentVersion(demoComponent, demoVersion)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if compVer != nil {
			_ = compVer.Close()
		}
		if repo != nil {
			_ = repo.Close()
		}
	})

	newConfig := func(chartResource, pullSecret string) solarv1alpha1.ReleaseConfig {
		return solarv1alpha1.ReleaseConfig{
			Input: solarv1alpha1.ReleaseInput{
				Resources: map[string]solarv1alpha1.ResolvedResourceAccess{
					"nginx-image": {
						Repository:     "ghcr.io/linuxserver/nginx",
						PullSecretName: pullSecret,
					},
				},
				Entrypoint: solarv1alpha1.Entrypoint{
					ResourceName: chartResource,
					Type:         solarv1alpha1.EntrypointTypeHelm,
				},
			},
		}
	}

	It("resolves the component's resources into fully qualified images", func() {
		rendered, err := renderValuesFrom(compVer, newConfig(demoChart, "regcred"))
		Expect(err).NotTo(HaveOccurred())

		Expect(rendered).To(ContainSubstring("nginx"))
		Expect(rendered).NotTo(ContainSubstring("repository: /"),
			"image repository rendered without a registry host: %s", rendered)
	})

	It("returns empty when no template is labelled for the entrypoint chart", func() {
		rendered, err := renderValuesFrom(compVer, newConfig("not-a-chart-resource", "regcred"))
		Expect(err).NotTo(HaveOccurred())
		Expect(rendered).To(BeEmpty())
	})
})

var _ = Describe("renderValuesTemplate", func() {
	It("returns empty without contacting a registry when there is no component ref", func() {
		rendered, err := renderValuesTemplate(GinkgoT().Context(), solarv1alpha1.ReleaseConfig{}, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(rendered).To(BeEmpty())
	})

	It("fails on a malformed component ref", func() {
		cfg := solarv1alpha1.ReleaseConfig{
			Input: solarv1alpha1.ReleaseInput{
				Component: solarv1alpha1.ReleaseComponent{Ref: "not-a-valid-ref"},
			},
		}

		_, err := renderValuesTemplate(GinkgoT().Context(), cfg, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to parse component reference"))
	})
})
