// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("agentValues", func() {
	cfg := &solarv1alpha1.AgentCredentialConfig{
		APIServerURL: "https://solar.example:6443", CAData: []byte("PEM"),
		TargetName: "edge-01", TargetNamespace: "fleet",
	}

	It("renders target, kubeconfig with tokenFile, and the token", func() {
		out, err := agentValues(cfg, "tok")
		Expect(err).NotTo(HaveOccurred())

		var v struct {
			TargetName          string `json:"targetName"`
			TargetNamespace     string `json:"targetNamespace"`
			APIServerKubeconfig struct {
				Value string `json:"value"`
				Token string `json:"token"`
			} `json:"apiserverKubeconfig"`
		}
		Expect(yaml.Unmarshal([]byte(out), &v)).To(Succeed())
		Expect(v.TargetName).To(Equal("edge-01"))
		Expect(v.TargetNamespace).To(Equal("fleet"))
		Expect(v.APIServerKubeconfig.Token).To(Equal("tok"))

		kc, err := clientcmd.Load([]byte(v.APIServerKubeconfig.Value))
		Expect(err).NotTo(HaveOccurred())
		Expect(kc.Clusters[kc.Contexts[kc.CurrentContext].Cluster].Server).To(Equal("https://solar.example:6443"))
		Expect(kc.Clusters[kc.Contexts[kc.CurrentContext].Cluster].CertificateAuthorityData).To(Equal([]byte("PEM")))
		Expect(kc.AuthInfos[kc.Contexts[kc.CurrentContext].AuthInfo].TokenFile).To(Equal("/etc/solar-agent/token"))
		Expect(kc.AuthInfos[kc.Contexts[kc.CurrentContext].AuthInfo].Token).To(BeEmpty(), "the kubeconfig itself holds no secret")
	})

	It("refuses an empty token", func() {
		_, err := agentValues(cfg, "")
		Expect(err).To(HaveOccurred())
	})
})
