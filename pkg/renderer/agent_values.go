// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"errors"
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// agentTokenFile is where the solar-agent chart mounts the token key.
const agentTokenFile = "/etc/solar-agent/token"

// agentValues renders the solar-agent chart values that connect one agent to
// SolAr. The kubeconfig points at the token file instead of embedding the
// token, so a rotated token is picked up without a restart.
func agentValues(c *solarv1alpha1.AgentCredentialConfig, token string) (string, error) {
	if token == "" {
		return "", errors.New("agent credential requested but TARGET_CREDENTIAL_TOKEN is empty")
	}

	kc := clientcmdapi.NewConfig()
	kc.Clusters["solar"] = &clientcmdapi.Cluster{Server: c.APIServerURL, CertificateAuthorityData: c.CAData}
	kc.AuthInfos["agent"] = &clientcmdapi.AuthInfo{TokenFile: agentTokenFile}
	kc.Contexts["solar"] = &clientcmdapi.Context{Cluster: "solar", AuthInfo: "agent"}
	kc.CurrentContext = "solar"

	kubeconfig, err := clientcmd.Write(*kc)
	if err != nil {
		return "", fmt.Errorf("writing agent kubeconfig: %w", err)
	}

	out, err := yaml.Marshal(map[string]any{
		"targetName":      c.TargetName,
		"targetNamespace": c.TargetNamespace,
		"apiserverKubeconfig": map[string]any{
			"value": string(kubeconfig),
			"token": token,
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshalling agent values: %w", err)
	}

	return string(out), nil
}
