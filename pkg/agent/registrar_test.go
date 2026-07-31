// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solarfake "go.opendefense.cloud/solar/client-go/clientset/versioned/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Registrar", func() {
	ctx := context.Background()
	spec := solarv1alpha1.TargetSpec{
		RenderRegistryRef: corev1.LocalObjectReference{Name: "my-registry"},
	}

	It("creates the target when it doesn't exist yet", func() {
		client := solarfake.NewSimpleClientset()
		r := &Registrar{Client: client, Namespace: "tenant-a", Name: "cluster-1", Spec: spec}

		target, err := r.EnsureTarget(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(target.Name).To(Equal("cluster-1"))
		Expect(target.Spec).To(Equal(spec))

		stored, err := client.SolarV1alpha1().Targets("tenant-a").Get(ctx, "cluster-1", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(stored.Spec).To(Equal(spec))
	})

	It("returns the existing target unmodified when one is already present", func() {
		existing := &solarv1alpha1.Target{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-1", Namespace: "tenant-a"},
			Spec:       solarv1alpha1.TargetSpec{RenderRegistryRef: corev1.LocalObjectReference{Name: "someone-elses-registry"}},
		}
		client := solarfake.NewSimpleClientset(existing)
		r := &Registrar{Client: client, Namespace: "tenant-a", Name: "cluster-1", Spec: spec}

		target, err := r.EnsureTarget(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(target.Spec.RenderRegistryRef.Name).To(Equal("someone-elses-registry"))
	})
})
