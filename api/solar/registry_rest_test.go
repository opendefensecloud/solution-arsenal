// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar_test

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.opendefense.cloud/solar/api/solar"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Registry REST", func() {
	Describe("Validate (create path)", func() {
		It("accepts a registry without a webhook path or scan interval", func() {
			r := &solar.Registry{
				Spec: solar.RegistrySpec{Hostname: "registry.example.com:5000"},
			}
			Expect(r.Validate(context.Background())).To(BeEmpty())
		})

		It("rejects a webhookPath without a flavor", func() {
			r := &solar.Registry{
				Spec: solar.RegistrySpec{
					Hostname:    "registry.example.com:5000",
					WebhookPath: "my-registry",
				},
			}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.flavor"))
		})

		It("accepts a webhookPath with a flavor", func() {
			r := &solar.Registry{
				Spec: solar.RegistrySpec{
					Hostname:    "registry.example.com:5000",
					WebhookPath: "my-registry",
					Flavor:      "zot",
				},
			}
			Expect(r.Validate(context.Background())).To(BeEmpty())
		})

		It("rejects a non-positive scanInterval", func() {
			r := &solar.Registry{
				Spec: solar.RegistrySpec{
					Hostname:     "registry.example.com:5000",
					ScanInterval: &metav1.Duration{Duration: 0},
				},
			}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.scanInterval"))
		})

		It("accepts a positive scanInterval", func() {
			r := &solar.Registry{
				Spec: solar.RegistrySpec{
					Hostname:     "registry.example.com:5000",
					ScanInterval: &metav1.Duration{Duration: 30},
				},
			}
			Expect(r.Validate(context.Background())).To(BeEmpty())
		})
	})

	Describe("ValidateUpdate (update path)", func() {
		It("rejects the same invalid state as Validate", func() {
			old := &solar.Registry{
				Spec: solar.RegistrySpec{Hostname: "registry.example.com:5000"},
			}
			updated := old.DeepCopy()
			updated.Spec.WebhookPath = "my-registry"

			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.flavor"))
		})
	})
})
