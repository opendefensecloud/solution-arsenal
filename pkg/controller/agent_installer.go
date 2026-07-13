// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// agentInstallNamespace is where the marker (and, eventually, the real
// solar-agent Deployment) is created on the target's own cluster.
const agentInstallNamespace = "solar-system"

// MarkerInstaller proves the remote-kubeconfig-driven install mechanism end
// to end without a real solar-agent chart/image, which don't exist yet: it
// creates a namespace and a ConfigMap on the target cluster via the
// provided restConfig
type MarkerInstaller struct{}

func (MarkerInstaller) Install(ctx context.Context, restConfig *rest.Config, target *solarv1alpha1.Target) error {
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("building client for target cluster: %w", err)
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: agentInstallNamespace}}
	if _, err := client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating %s namespace: %w", agentInstallNamespace, err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "solar-agent-installed", Namespace: agentInstallNamespace},
		Data: map[string]string{
			"target": target.Namespace + "/" + target.Name,
		},
	}
	if _, err := client.CoreV1().ConfigMaps(agentInstallNamespace).Create(ctx, cm, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating solar-agent-installed marker: %w", err)
	}

	return nil
}
