// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar_test

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.opendefense.cloud/solar/api/solar"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RenderBinding REST", func() {
	validSpec := func() solar.RenderBindingSpec {
		return solar.RenderBindingSpec{
			RenderArtifactRef: corev1.LocalObjectReference{Name: "my-renderartifact"},
			OwnerKind:         "Target",
			OwnerName:         "my-target",
			OwnerNamespace:    "default",
		}
	}

	Describe("Validate (create path)", func() {
		It("accepts a fully populated spec", func() {
			r := &solar.RenderBinding{Spec: validSpec()}
			Expect(r.Validate(context.Background())).To(BeEmpty())
		})

		It("rejects an empty renderArtifactRef.name", func() {
			spec := validSpec()
			spec.RenderArtifactRef.Name = ""
			r := &solar.RenderBinding{Spec: spec}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.renderArtifactRef.name"))
		})

		It("rejects an empty ownerKind", func() {
			spec := validSpec()
			spec.OwnerKind = ""
			r := &solar.RenderBinding{Spec: spec}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.ownerKind"))
		})

		It("rejects an empty ownerName", func() {
			spec := validSpec()
			spec.OwnerName = ""
			r := &solar.RenderBinding{Spec: spec}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.ownerName"))
		})

		It("rejects an empty ownerNamespace", func() {
			spec := validSpec()
			spec.OwnerNamespace = ""
			r := &solar.RenderBinding{Spec: spec}
			errs := r.Validate(context.Background())
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.ownerNamespace"))
		})

		It("reports all missing required fields at once", func() {
			r := &solar.RenderBinding{}
			Expect(r.Validate(context.Background())).To(HaveLen(4))
		})
	})

	Describe("ValidateUpdate (update path)", func() {
		It("rejects the same invalid state as Validate", func() {
			old := &solar.RenderBinding{Spec: validSpec()}
			updated := old.DeepCopy()
			updated.Spec.OwnerName = ""

			errs := updated.ValidateUpdate(context.Background(), old)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Field).To(Equal("spec.ownerName"))
		})
	})

	Describe("ConvertToTable", func() {
		It("should return correct columns and cells", func() {
			obj := &solar.RenderBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-renderbinding",
					CreationTimestamp: metav1.Now(),
				},
				Spec: validSpec(),
			}

			table, err := obj.ConvertToTable(context.Background(), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(5))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("Artifact"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("OwnerKind"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("OwnerName"))
			Expect(table.ColumnDefinitions[4].Name).To(Equal("Age"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells[0]).To(Equal("my-renderbinding"))
			Expect(table.Rows[0].Cells[1]).To(Equal("my-renderartifact"))
			Expect(table.Rows[0].Cells[2]).To(Equal("Target"))
			Expect(table.Rows[0].Cells[3]).To(Equal("my-target"))
			Expect(table.Rows[0].Cells[4]).To(BeAssignableToTypeOf(""))
		})
	})
})
