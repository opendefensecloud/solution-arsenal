// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar_test

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"go.opendefense.cloud/solar/api/solar"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReleaseBinding REST", func() {
	Describe("ValidateUpdate (update path)", func() {
		old := &solar.ReleaseBinding{
			Spec: solar.ReleaseBindingSpec{
				TargetRef:  solar.ObjectReference{Name: "a", Namespace: "ns-a"},
				ReleaseRef: corev1.LocalObjectReference{Name: "my-release"},
			},
		}

		It("accepts an unchanged targetRef", func() {
			updated := old.DeepCopy()
			updated.Spec.ReleaseRef.Name = "other-release"
			Expect(updated.ValidateUpdate(context.Background(), old)).To(BeEmpty())
		})

		It("rejects a changed targetRef.name", func() {
			updated := old.DeepCopy()
			updated.Spec.TargetRef.Name = "b"
			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Field).To(Equal("spec.targetRef"))
		})

		It("rejects a changed targetRef.namespace", func() {
			updated := old.DeepCopy()
			updated.Spec.TargetRef.Namespace = ""
			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Field).To(Equal("spec.targetRef"))
		})
	})
})
