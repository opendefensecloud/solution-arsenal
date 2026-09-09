// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solarfake "go.opendefense.cloud/solar/client-go/clientset/versioned/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TargetResolver", func() {
	ctx := context.Background()

	It("returns the target a user created for this agent", func() {
		existing := &solarv1alpha1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-1", Namespace: "tenant-a"},
			Spec:       solarv1alpha1.TargetSpec{RenderRegistryRef: solarv1alpha1.ObjectReference{Name: "deploy-registry"}},
		}
		r := &TargetResolver{Client: solarfake.NewSimpleClientset(existing), Namespace: "tenant-a", Name: "cluster-1"}

		target, err := r.ResolveTarget(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(target.Spec.RenderRegistryRef.Name).To(Equal("deploy-registry"))
	})

	It("fails instead of creating a target that isn't there", func() {
		client := solarfake.NewSimpleClientset()
		r := &TargetResolver{Client: client, Namespace: "tenant-a", Name: "cluster-1"}

		_, err := r.ResolveTarget(ctx)
		Expect(err).To(HaveOccurred())

		list, listErr := client.SolarV1alpha1().Targets("tenant-a").List(ctx, metav1.ListOptions{})
		Expect(listErr).NotTo(HaveOccurred())
		Expect(list.Items).To(BeEmpty())
	})
})
