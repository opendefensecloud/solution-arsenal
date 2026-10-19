// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// brokenDiscovery fails every discovery call, standing in for an unreachable
// target cluster.
type brokenDiscovery struct {
	discovery.DiscoveryInterface
	err error
}

func (b brokenDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return nil, b.err
}

var _ = Describe("FluxCrds", func() {
	ctx := context.Background()

	apiResourceList := func(groupVersion, resource string) *metav1.APIResourceList {
		return &metav1.APIResourceList{
			GroupVersion: groupVersion,
			APIResources: []metav1.APIResource{{Name: resource}},
		}
	}

	discoveryWith := func(resources ...*metav1.APIResourceList) *fakediscovery.FakeDiscovery {
		d := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}}
		d.Resources = resources

		return d
	}

	ociList := apiResourceList("source.toolkit.fluxcd.io/v1", ociRepositoryGVR.Resource)
	helmList := apiResourceList("helm.toolkit.fluxcd.io/v2", helmReleaseGVR.Resource)

	It("passes when both Flux group/versions are served", func() {
		check := FluxCrds{Discovery: discoveryWith(ociList, helmList)}

		Expect(check.Type()).To(Equal("FluxCrds"))

		status, reason, _ := check.Run(ctx)
		Expect(status).To(Equal(metav1.ConditionTrue))
		Expect(reason).To(Equal(reasonPresent))
	})

	It("fails when a Flux group/version is not served", func() {
		check := FluxCrds{Discovery: discoveryWith(ociList)}

		status, reason, message := check.Run(ctx)
		Expect(status).To(Equal(metav1.ConditionFalse))
		Expect(reason).To(Equal(reasonMissing))
		Expect(message).To(ContainSubstring("helm.toolkit.fluxcd.io/v2"))
	})

	It("reports Unknown when discovery cannot answer", func() {
		check := FluxCrds{Discovery: brokenDiscovery{err: errors.New("cluster unreachable")}}

		status, reason, _ := check.Run(ctx)
		Expect(status).To(Equal(metav1.ConditionUnknown))
		Expect(reason).To(Equal(reasonUnavailable))
	})
})
