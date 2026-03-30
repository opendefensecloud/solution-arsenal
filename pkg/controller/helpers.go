// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	// Labels used to track ownership of cluster-scoped RenderTasks
	labelOwnerName      = "solar.opendefense.cloud/owner-name"
	labelOwnerNamespace = "solar.opendefense.cloud/owner-namespace"
	labelOwnerKind      = "solar.opendefense.cloud/owner-kind"

	maxK8sObjectNameLen = 253
	maxK8sLabelValueLen = 63
)

// truncateName truncates a name to maxLen characters. If truncation is needed,
// it appends a short hash suffix to preserve uniqueness.
func truncateName(name string, maxLen int) string {
	if maxLen < 10 {
		maxLen = 10
	}
	if len(name) <= maxLen {
		return name
	}
	hash := sha256.Sum256([]byte(name))
	hashStr := hex.EncodeToString(hash[:])[:8]

	return name[:maxLen-9] + "-" + hashStr
}

func renderTaskName(res metav1.Object) string {
	base := fmt.Sprintf("%s-%s-%d", res.GetNamespace(), res.GetName(), res.GetGeneration())
	return truncateName(base, maxK8sObjectNameLen)
}

// renderTaskLabels returns labels to set on a RenderTask for ownership tracking.
func renderTaskLabels(res metav1.Object, kind string) map[string]string {
	return map[string]string{
		labelOwnerName:      res.GetName(),
		labelOwnerNamespace: res.GetNamespace(),
		labelOwnerKind:      kind,
	}
}

// mapRenderTaskToOwner returns a handler.MapFunc that maps RenderTask events
// to reconcile requests for the owning resource of the specified kind.
func mapRenderTaskToOwner(kind string) handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		rt, ok := obj.(*solarv1alpha1.RenderTask)
		if !ok {
			return nil
		}
		labels := rt.GetLabels()
		if labels == nil {
			return nil
		}

		ownerKind := labels[labelOwnerKind]
		ownerName := labels[labelOwnerName]
		ownerNamespace := labels[labelOwnerNamespace]

		if ownerKind != kind || ownerName == "" || ownerNamespace == "" {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      ownerName,
					Namespace: ownerNamespace,
				},
			},
		}
	}
}
