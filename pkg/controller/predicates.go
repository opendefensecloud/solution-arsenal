// Copyright 2026 BWI GmbH and Solution Arsenal contributors
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
// This avoids unnecessary reconciliation of the owning Release or Bootstrap
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
