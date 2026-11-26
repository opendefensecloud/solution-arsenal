// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar_test

import (
	"context"
	"encoding/json"

	"go.opendefense.cloud/solar/api/solar"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Release REST", func() {
	Describe("Validate (create path)", func() {
		It("rejects an empty componentVersionRef.name", func() {
			r := &solar.Release{
				Spec: solar.ReleaseSpec{
					ComponentVersionRef: solar.ObjectReference{Name: ""},
					UniqueName:          "kyverno",
				},
			}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.componentVersionRef.name"))
		})

		DescribeTable("rejects names the renderer would print into manifests",
			func(field string, mutate func(*solar.ReleaseSpec)) {
				r := &solar.Release{
					Spec: solar.ReleaseSpec{ComponentVersionRef: solar.ObjectReference{Name: "kyverno-v1"}},
				}
				mutate(&r.Spec)
				errs := r.Validate(context.Background())
				Expect(errs).NotTo(BeEmpty())
				Expect(errs[0].Field).To(Equal(field))
			},
			Entry("targetNamespace", "spec.targetNamespace", func(s *solar.ReleaseSpec) {
				s.TargetNamespace = new("apps\n---\nkind: Namespace")
			}),
			Entry("uniqueName", "spec.uniqueName", func(s *solar.ReleaseSpec) { s.UniqueName = "Kyverno" }),
			Entry("componentVersionRef.name", "spec.componentVersionRef.name", func(s *solar.ReleaseSpec) {
				s.ComponentVersionRef.Name = "kyverno\nv1"
			}),
		)

		It("accepts a targetNamespace", func() {
			r := &solar.Release{
				Spec: solar.ReleaseSpec{
					ComponentVersionRef: solar.ObjectReference{Name: "kyverno-v1"},
					TargetNamespace:     new("kyverno"),
				},
			}
			Expect(r.Validate(context.Background())).To(BeEmpty())
		})

		It("accepts a non-empty UniqueName", func() {
			r := &solar.Release{
				Spec: solar.ReleaseSpec{
					ComponentVersionRef: solar.ObjectReference{Name: "kyverno-v1"},
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
					ComponentVersionRef: solar.ObjectReference{Name: "kyverno-v1"},
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
					ComponentVersionRef: solar.ObjectReference{Name: "kyverno-v1"},
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
					ComponentVersionRef: solar.ObjectReference{Name: "kyverno-v1"},
					UniqueName:          "kyverno",
				},
			}
			Expect(r.ValidateUpdate(context.Background(), r.DeepCopy())).To(BeEmpty())
		})
	})

	Describe("ReleaseSpec JSON", func() {
		It("serializes UniqueName", func() {
			spec := solar.ReleaseSpec{
				ComponentVersionRef: solar.ObjectReference{Name: "kyverno-v1"},
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
				ComponentVersionRef: solar.ObjectReference{Name: "kyverno-v1"},
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
