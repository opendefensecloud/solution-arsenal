// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// hostile fails the render if Helm evaluates it and adds a Namespace if YAML
// splits it into documents.
const hostile = "{{ fail \"evaluated\" }}\n---\napiVersion: v1\nkind: Namespace\nmetadata:\n  name: pwned\n"

func nestedString(obj map[string]any, fields ...string) string {
	GinkgoHelper()
	s, found, err := unstructured.NestedString(obj, fields...)
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue(), "%v not found", fields)

	return s
}

func kinds(ms []unstructured.Unstructured) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.GetKind())
	}

	return out
}

var _ = Describe("manifest injection", func() {
	var result *solarv1alpha1.RenderResult

	AfterEach(func() {
		if result != nil {
			Expect(result.Close()).To(Succeed())
			result = nil
		}
	})

	It("keeps SolAr's delimiters out of the Helm templates", func() {
		for _, dir := range []string{"template/release/templates", "template/bootstrap/templates"} {
			fsys := releaseFS
			if dir == "template/bootstrap/templates" {
				fsys = bootstrapFS
			}
			entries, err := fsys.ReadDir(dir)
			Expect(err).NotTo(HaveOccurred())
			for _, e := range entries {
				content, err := fsys.ReadFile(dir + "/" + e.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).NotTo(ContainSubstring("<<"), "%s/%s: Helm would evaluate what SolAr inserts", dir, e.Name())
			}
		}
	})

	Describe("release chart", func() {
		releaseConfig := func() solarv1alpha1.ReleaseConfig {
			values, err := json.Marshal(map[string]string{"x": hostile})
			Expect(err).NotTo(HaveOccurred())

			return solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{Name: "release", Description: hostile, Version: "1.0.0", AppVersion: "1.0.0"},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{Name: "comp"},
					Resources: map[string]solarv1alpha1.ResolvedResourceAccess{
						"chart": {Repository: "example.com/charts/app", Tag: "1.0.0", PullSecretName: "pull"},
					},
					Entrypoint: solarv1alpha1.Entrypoint{ResourceName: "chart", Type: solarv1alpha1.EntrypointTypeHelm},
				},
				TargetNamespace: "apps",
				Values:          runtime.RawExtension{Raw: values},
				AgentCredential: &solarv1alpha1.AgentCredentialConfig{
					APIServerURL: "https://solar.example", TargetName: hostile, TargetNamespace: "fleet"},
			}
		}

		It("keeps free-form text as data", func() {
			var err error
			result, err = renderRelease(releaseConfig(), hostile, hostile)
			Expect(err).NotTo(HaveOccurred())

			ms, err := helmTemplate("bar", "test-ns", result.Dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(kinds(ms)).To(ConsistOf("OCIRepository", "HelmRelease", "ConfigMap", "Secret"))

			byKind := map[string]map[string]any{}
			for _, m := range ms {
				byKind[m.GetKind()] = m.Object
			}
			Expect(nestedString(byKind["HelmRelease"], "spec", "values", "x")).To(Equal(hostile))
			Expect(nestedString(byKind["ConfigMap"], "data", "values.yaml")).To(Equal(hostile))

			agent := nestedString(byKind["Secret"], "stringData", "values.yaml")
			var v struct {
				TargetName     string `json:"targetName"`
				WatchNamespace string `json:"watchNamespace"`
				Kubeconfig     struct {
					Token string `json:"token"`
				} `json:"apiserverKubeconfig"`
			}
			Expect(yaml.Unmarshal([]byte(agent), &v)).To(Succeed())
			Expect(v.TargetName).To(Equal(hostile))
			Expect(v.Kubeconfig.Token).To(Equal(hostile))
			Expect(v.WatchNamespace).To(Equal("test-ns"))

			chart, err := os.ReadFile(filepath.Join(result.Dir, "Chart.yaml"))
			Expect(err).NotTo(HaveOccurred())
			var c struct {
				Description string `json:"description"`
			}
			Expect(yaml.Unmarshal(chart, &c)).To(Succeed())
			Expect(c.Description).To(Equal(hostile))
		})

		DescribeTable("rejects fields that name things",
			func(mutate func(*solarv1alpha1.ReleaseConfig)) {
				c := releaseConfig()
				mutate(&c)
				_, err := renderRelease(c, "", "tok")
				Expect(err).To(MatchError(ContainSubstring("invalid release config")))
			},
			Entry("targetNamespace", func(c *solarv1alpha1.ReleaseConfig) { c.TargetNamespace = hostile }),
			Entry("component name", func(c *solarv1alpha1.ReleaseConfig) { c.Input.Component.Name = hostile }),
			Entry("repository", func(c *solarv1alpha1.ReleaseConfig) {
				c.Input.Resources["chart"] = solarv1alpha1.ResolvedResourceAccess{Repository: hostile, Tag: "1.0.0"}
			}),
			Entry("tag", func(c *solarv1alpha1.ReleaseConfig) {
				c.Input.Resources["chart"] = solarv1alpha1.ResolvedResourceAccess{Repository: "example.com/a", Tag: hostile}
			}),
			Entry("digest", func(c *solarv1alpha1.ReleaseConfig) {
				c.Input.Resources["chart"] = solarv1alpha1.ResolvedResourceAccess{Repository: "example.com/a", Tag: "@" + hostile}
			}),
			Entry("pullSecretName", func(c *solarv1alpha1.ReleaseConfig) {
				c.Input.Resources["chart"] = solarv1alpha1.ResolvedResourceAccess{Repository: "example.com/a", Tag: "1.0.0", PullSecretName: hostile}
			}),
		)
	})

	Describe("bootstrap chart", func() {
		bootstrapConfig := func(name string, r solarv1alpha1.ResolvedResourceAccess) solarv1alpha1.BootstrapConfig {
			userdata, err := json.Marshal(map[string]string{"x": hostile})
			Expect(err).NotTo(HaveOccurred())

			return solarv1alpha1.BootstrapConfig{
				Chart: solarv1alpha1.ChartConfig{Name: "bootstrap", Description: hostile, Version: "1.0.0", AppVersion: "1.0.0"},
				Input: solarv1alpha1.BootstrapInput{
					Releases: map[string]solarv1alpha1.ResolvedResourceAccess{name: r},
					Userdata: runtime.RawExtension{Raw: userdata},
				},
			}
		}

		It("keeps free-form text as data", func() {
			var err error
			result, err = RenderBootstrap(bootstrapConfig("app", solarv1alpha1.ResolvedResourceAccess{
				Repository: "example.com/charts/app", Tag: hostile, PullSecretName: "pull",
			}))
			Expect(err).NotTo(HaveOccurred())

			ms, err := helmTemplate("bar", "test-ns", result.Dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(kinds(ms)).To(ConsistOf("OCIRepository", "HelmRelease"))
			for _, m := range ms {
				switch m.GetKind() {
				case "OCIRepository":
					Expect(nestedString(m.Object, "spec", "ref", "semver")).To(Equal(hostile))
				case "HelmRelease":
					Expect(nestedString(m.Object, "spec", "values", "userdata", "x")).To(Equal(hostile))
				}
			}
		})

		DescribeTable("rejects fields that name things",
			func(name string, r solarv1alpha1.ResolvedResourceAccess) {
				_, err := RenderBootstrap(bootstrapConfig(name, r))
				Expect(err).To(MatchError(ContainSubstring("invalid bootstrap config")))
			},
			Entry("release name", hostile, solarv1alpha1.ResolvedResourceAccess{Repository: "example.com/a", Tag: "1.0.0"}),
			Entry("repository", "app", solarv1alpha1.ResolvedResourceAccess{Repository: hostile, Tag: "1.0.0"}),
			Entry("pullSecretName", "app", solarv1alpha1.ResolvedResourceAccess{Repository: "example.com/a", Tag: "1.0.0", PullSecretName: hostile}),
		)
	})
})
