// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	// Field index keys for looking up RenderTasks by owner.
	indexOwnerKind      = "spec.ownerKind"
	indexOwnerName      = "spec.ownerName"
	indexOwnerNamespace = "spec.ownerNamespace"

	// Field index key for looking up ReleaseBindings by target name.
	indexReleaseBindingTargetName = "spec.targetRef.name"

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

// mapRenderTaskToOwner returns a handler.MapFunc that maps RenderTask events
// to reconcile requests for the owning resource of the specified kind.
func mapRenderTaskToOwner(kind string) handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		rt, ok := obj.(*solarv1alpha1.RenderTask)
		if !ok {
			return nil
		}

		if rt.Spec.OwnerKind != kind || rt.Spec.OwnerName == "" || rt.Spec.OwnerNamespace == "" {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      rt.Spec.OwnerName,
					Namespace: rt.Spec.OwnerNamespace,
				},
			},
		}
	}
}

// releaseRenderTaskName returns a deterministic name for a per-release RenderTask
// based on the release name and registry name. This enables dedup across targets
// sharing the same render registry.
func releaseRenderTaskName(releaseName, registryName string) string {
	input := fmt.Sprintf("%s-%s", releaseName, registryName)
	hash := sha256.Sum256([]byte(input))
	hashStr := hex.EncodeToString(hash[:])[:8]

	return truncateName(fmt.Sprintf("render-rel-%s-%s", releaseName, hashStr), maxK8sObjectNameLen)
}

// targetRenderTaskName returns a deterministic name for a per-target bootstrap RenderTask.
// The bootstrapVersion is incremented each time the bootstrap needs re-rendering.
func targetRenderTaskName(targetName string, bootstrapVersion int64) string {
	return truncateName(fmt.Sprintf("render-tgt-%s-%d", targetName, bootstrapVersion), maxK8sObjectNameLen)
}

// IndexFields registers field indexers on the manager for efficient lookups.
// Must be called once before any controller that uses these indexes is set up.
func IndexFields(ctx context.Context, mgr ctrl.Manager) error {
	if err := indexReleaseBindingFields(ctx, mgr); err != nil {
		return err
	}

	return indexRenderTaskOwnerFields(ctx, mgr)
}

func indexReleaseBindingFields(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &solarv1alpha1.ReleaseBinding{}, indexReleaseBindingTargetName, func(obj client.Object) []string {
		rb := obj.(*solarv1alpha1.ReleaseBinding)
		if rb.Spec.TargetRef.Name == "" {
			return nil
		}

		return []string{rb.Spec.TargetRef.Name}
	})
}

// indexRenderTaskOwnerFields registers field indexers on the manager for
// looking up RenderTasks by owner kind, name, and namespace.
func indexRenderTaskOwnerFields(ctx context.Context, mgr ctrl.Manager) error {
	indexer := mgr.GetFieldIndexer()

	if err := indexer.IndexField(ctx, &solarv1alpha1.RenderTask{}, indexOwnerKind, func(obj client.Object) []string {
		rt := obj.(*solarv1alpha1.RenderTask)
		if rt.Spec.OwnerKind == "" {
			return nil
		}

		return []string{rt.Spec.OwnerKind}
	}); err != nil {
		return err
	}

	if err := indexer.IndexField(ctx, &solarv1alpha1.RenderTask{}, indexOwnerName, func(obj client.Object) []string {
		rt := obj.(*solarv1alpha1.RenderTask)
		if rt.Spec.OwnerName == "" {
			return nil
		}

		return []string{rt.Spec.OwnerName}
	}); err != nil {
		return err
	}

	return indexer.IndexField(ctx, &solarv1alpha1.RenderTask{}, indexOwnerNamespace, func(obj client.Object) []string {
		rt := obj.(*solarv1alpha1.RenderTask)
		if rt.Spec.OwnerNamespace == "" {
			return nil
		}

		return []string{rt.Spec.OwnerNamespace}
	})
}
