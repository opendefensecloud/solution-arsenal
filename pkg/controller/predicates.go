// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// renderTaskStatusChangePredicate returns a predicate that filters RenderTask
// events to only trigger reconciliation when the RenderTask's status changes.
// This avoids unnecessary reconciliation of the owning Target
// when only the RenderTask's metadata (e.g. finalizers) changes.
func renderTaskStatusChangePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldRT, ok := e.ObjectOld.(*solarv1alpha1.RenderTask)
			if !ok {
				return true
			}
			newRT, ok := e.ObjectNew.(*solarv1alpha1.RenderTask)
			if !ok {
				return true
			}

			return !apiequality.Semantic.DeepEqual(oldRT.Status, newRT.Status)
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return true },
	}
}

// componentVersionUpdatePredicate drops updates that only touch status. The
// reconciler writes the Available condition itself; without this filter every
// status write would trigger another registry check, and an error message that
// changes between attempts would turn into a loop against the registry.
func componentVersionUpdatePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCV, ok := e.ObjectOld.(*solarv1alpha1.ComponentVersion)
			if !ok {
				return true
			}
			newCV, ok := e.ObjectNew.(*solarv1alpha1.ComponentVersion)
			if !ok {
				return true
			}
			o, n := oldCV.DeepCopy(), newCV.DeepCopy()
			o.Status, n.Status = solarv1alpha1.ComponentVersionStatus{}, solarv1alpha1.ComponentVersionStatus{}
			o.ResourceVersion, n.ResourceVersion = "", ""
			o.ManagedFields, n.ManagedFields = nil, nil

			return !apiequality.Semantic.DeepEqual(o, n)
		},
	}
}
