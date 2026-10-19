// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

// FluxCrds verifies the target cluster serves the Flux CRDs the bootstrap chart
// writes through. It reads the local cluster (the one the agent runs on), not
// solar-apiserver.
type FluxCrds struct {
	Discovery discovery.DiscoveryInterface
}

// Type is the check's stable PascalCase identifier.
func (FluxCrds) Type() string { return "FluxCrds" }

// Run reports whether both Flux group/versions are served. Missing means a CRD
// the bootstrap chart needs is absent; Unknown means discovery could not answer.
func (c FluxCrds) Run(context.Context) (metav1.ConditionStatus, string, string) {
	for _, gvr := range []schema.GroupVersionResource{ociRepositoryGVR, helmReleaseGVR} {
		groupVersion := gvr.GroupVersion().String()

		resources, err := c.Discovery.ServerResourcesForGroupVersion(groupVersion)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return metav1.ConditionFalse, reasonMissing, fmt.Sprintf("%s is not served", groupVersion)
			}

			return metav1.ConditionUnknown, reasonUnavailable, fmt.Sprintf("discovering %s: %v", groupVersion, err)
		}

		if !hasResource(resources, gvr.Resource) {
			return metav1.ConditionFalse, reasonMissing, fmt.Sprintf("%s does not serve %s", groupVersion, gvr.Resource)
		}
	}

	return metav1.ConditionTrue, reasonPresent, fmt.Sprintf("%s and %s are served",
		helmReleaseGVR.GroupResource().String(),
		ociRepositoryGVR.GroupResource().String())
}

func hasResource(list *metav1.APIResourceList, resource string) bool {
	if list == nil {
		return false
	}

	for _, r := range list.APIResources {
		if r.Name == resource {
			return true
		}
	}

	return false
}
