// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar_test

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"

	"go.opendefense.cloud/solar/api/solar"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Release REST", func() {
	Describe("Validate (create path)", func() {
		It("rejects an empty UniqueName", func() {
			r := &solar.Release{
				Spec: solar.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{Name: "kyverno-v1"},
					UniqueName:          "",
				},
			}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.uniqueName"))
		})

		It("accepts a non-empty UniqueName", func() {
			r := &solar.Release{
				Spec: solar.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{Name: "kyverno-v1"},
					UniqueName:          "kyverno",
				},
			}
			Expect(r.Validate(context.Background())).To(BeEmpty())
		})
	})

	Describe("ValidateUpdate (update path)", func() {
		It("rejects an empty UniqueName", func() {
			old := &solar.Release{
				Spec: solar.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{Name: "kyverno-v1"},
					UniqueName:          "kyverno",
				},
			}
			updated := old.DeepCopy()
			updated.Spec.UniqueName = ""
			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.uniqueName"))
		})

		It("rejects a changed UniqueName", func() {
			old := &solar.Release{
				Spec: solar.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{Name: "kyverno-v1"},
					UniqueName:          "kyverno",
				},
			}
			updated := old.DeepCopy()
			updated.Spec.UniqueName = "kyverno-renamed"
			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.uniqueName"))
		})

		It("accepts an unchanged UniqueName", func() {
			r := &solar.Release{
				Spec: solar.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{Name: "kyverno-v1"},
					UniqueName:          "kyverno",
				},
			}
			Expect(r.ValidateUpdate(context.Background(), r.DeepCopy())).To(BeEmpty())
		})
	})

	Describe("ReleaseSpec JSON", func() {
		It("serializes UniqueName", func() {
			spec := solar.ReleaseSpec{
				ComponentVersionRef: corev1.LocalObjectReference{Name: "kyverno-v1"},
				UniqueName:          "kyverno",
			}
			data, err := json.Marshal(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring(`"uniqueName":"kyverno"`))
		})

		It("deserializes UniqueName", func() {
			data := []byte(`{"componentVersionRef":{"name":"kyverno-v1"},"uniqueName":"kyverno"}`)
			var spec solar.ReleaseSpec
			Expect(json.Unmarshal(data, &spec)).To(Succeed())
			Expect(spec.UniqueName).To(Equal("kyverno"))
		})

		It("round-trips UniqueName through JSON", func() {
			original := solar.ReleaseSpec{
				ComponentVersionRef: corev1.LocalObjectReference{Name: "kyverno-v1"},
				UniqueName:          "kyverno",
			}
			data, err := json.Marshal(original)
			Expect(err).NotTo(HaveOccurred())
			var restored solar.ReleaseSpec
			Expect(json.Unmarshal(data, &restored)).To(Succeed())
			Expect(restored.UniqueName).To(Equal(original.UniqueName))
		})
	})
})
