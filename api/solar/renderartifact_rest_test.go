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

var _ = Describe("RenderArtifact REST", func() {
	validSpec := func() solar.RenderArtifactSpec {
		return solar.RenderArtifactSpec{
			BaseURL:       "registry.example.com:5000",
			Repository:    "charts/mychart",
			Tag:           "1.0.0",
			RenderTaskRef: "my-rendertask",
		}
	}

	Describe("Validate (create path)", func() {
		It("accepts a fully populated spec", func() {
			r := &solar.RenderArtifact{Spec: validSpec()}
			Expect(r.Validate(context.Background())).To(BeEmpty())
		})

		It("rejects an empty baseURL", func() {
			spec := validSpec()
			spec.BaseURL = ""
			r := &solar.RenderArtifact{Spec: spec}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.baseURL"))
		})

		It("rejects an empty repository", func() {
			spec := validSpec()
			spec.Repository = ""
			r := &solar.RenderArtifact{Spec: spec}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.repository"))
		})

		It("rejects an empty tag", func() {
			spec := validSpec()
			spec.Tag = ""
			r := &solar.RenderArtifact{Spec: spec}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.tag"))
		})

		It("rejects an empty renderTaskRef", func() {
			spec := validSpec()
			spec.RenderTaskRef = ""
			r := &solar.RenderArtifact{Spec: spec}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.renderTaskRef"))
		})

		It("reports all missing required fields at once", func() {
			r := &solar.RenderArtifact{}
			Expect(r.Validate(context.Background())).To(HaveLen(4))
		})
	})

	Describe("ValidateUpdate (update path)", func() {
		It("rejects the same invalid state as Validate", func() {
			old := &solar.RenderArtifact{Spec: validSpec()}
			updated := old.DeepCopy()
			updated.Spec.Tag = ""

			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.tag"))
		})
	})

	Describe("ConvertToTable", func() {
		It("should return correct columns and cells", func() {
			obj := &solar.RenderArtifact{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-renderartifact",
					CreationTimestamp: metav1.Now(),
				},
				Spec: validSpec(),
				Status: solar.RenderArtifactStatus{
					ChartURL: "oci://registry.example.com:5000/charts/mychart:1.0.0",
				},
			}

			table, err := obj.ConvertToTable(context.Background(), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(5))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("Repository"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("Tag"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("ChartURL"))
			Expect(table.ColumnDefinitions[4].Name).To(Equal("Age"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells[0]).To(Equal("my-renderartifact"))
			Expect(table.Rows[0].Cells[1]).To(Equal("charts/mychart"))
			Expect(table.Rows[0].Cells[2]).To(Equal("1.0.0"))
			Expect(table.Rows[0].Cells[3]).To(Equal("oci://registry.example.com:5000/charts/mychart:1.0.0"))
			Expect(table.Rows[0].Cells[4]).To(BeAssignableToTypeOf(""))
		})
	})
})
