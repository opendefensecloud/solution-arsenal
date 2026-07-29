// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar_test

import (
	"context"

	"go.opendefense.cloud/solar/api/solar"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RenderTask REST", func() {
	Describe("ValidateUpdate (update path)", func() {
		It("accepts an unchanged rendererConfig", func() {
			old := &solar.RenderTask{
				Spec: solar.RenderTaskSpec{
					RendererConfig: solar.RendererConfig{Type: solar.RendererConfigType("release")},
				},
			}
			updated := old.DeepCopy()
			Expect(updated.ValidateUpdate(context.Background(), old)).To(BeEmpty())
		})

		It("rejects a changed rendererConfig", func() {
			old := &solar.RenderTask{
				Spec: solar.RenderTaskSpec{
					RendererConfig: solar.RendererConfig{Type: solar.RendererConfigType("release")},
				},
			}
			updated := old.DeepCopy()
			updated.Spec.RendererConfig.Type = solar.RendererConfigType("bootstrap")

			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.rendererConfig"))
		})
	})
})
