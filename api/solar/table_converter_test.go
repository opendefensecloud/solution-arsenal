// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar_test

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.opendefense.cloud/solar/api/solar"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TableConverter", func() {
	var ctx context.Context
	var now time.Time

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Now()
	})

	Describe("Target", func() {
		It("should return correct columns and cells", func() {
			obj := &solar.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-target",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.TargetSpec{
					RenderRegistryRef: corev1.LocalObjectReference{Name: "my-registry"},
				},
				Status: solar.TargetStatus{
					BootstrapVersion: 3,
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(4))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("Render Registry"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("Bootstrap Version"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("Age"))
			Expect(table.ColumnDefinitions[0].Type).To(Equal("string"))
			Expect(table.ColumnDefinitions[2].Type).To(Equal("integer"))
			Expect(table.ColumnDefinitions[3].Type).To(Equal("date"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells).To(ConsistOf("my-target", "my-registry", int64(3), now))
			Expect(table.Rows[0].Object).To(Equal(runtime.RawExtension{Object: obj}))
		})
	})

	Describe("Release", func() {
		It("should return correct columns and cells with resolved condition", func() {
			obj := &solar.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-release",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{Name: "my-cv"},
				},
				Status: solar.ReleaseStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "ComponentVersionResolved",
							Status: metav1.ConditionTrue,
							Reason: "Resolved",
						},
					},
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(4))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("ComponentVersion Ref"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("Status"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("Age"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells).To(ConsistOf("my-release", "my-cv", "Resolved", now))
		})

		It("should return Unknown status when no condition exists", func() {
			obj := &solar.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-release",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.ReleaseSpec{
					ComponentVersionRef: corev1.LocalObjectReference{Name: "my-cv"},
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.Rows[0].Cells[2]).To(Equal("Unknown"))
		})
	})

	Describe("Profile", func() {
		It("should return correct columns and cells", func() {
			obj := &solar.Profile{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-profile",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.ProfileSpec{
					ReleaseRef: corev1.LocalObjectReference{Name: "my-release"},
				},
				Status: solar.ProfileStatus{
					MatchedTargets: 5,
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(4))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("Release Ref"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("Matched Targets"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("Age"))
			Expect(table.ColumnDefinitions[2].Type).To(Equal("integer"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells).To(ConsistOf("my-profile", "my-release", 5, now))
		})
	})

	Describe("ReleaseBinding", func() {
		It("should return correct columns and cells", func() {
			obj := &solar.ReleaseBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-releasebinding",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.ReleaseBindingSpec{
					TargetRef:  corev1.LocalObjectReference{Name: "my-target"},
					ReleaseRef: corev1.LocalObjectReference{Name: "my-release"},
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(4))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("Target"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("Release"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("Age"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells).To(ConsistOf("my-releasebinding", "my-target", "my-release", now))
		})
	})

	Describe("Registry", func() {
		It("should return correct columns and cells", func() {
			obj := &solar.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-registry",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.RegistrySpec{
					Hostname:  "registry.example.com:5000",
					PlainHTTP: true,
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(4))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("Hostname"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("Plain HTTP"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("Age"))
			Expect(table.ColumnDefinitions[2].Type).To(Equal("boolean"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells).To(ConsistOf("my-registry", "registry.example.com:5000", true, now))
		})
	})

	Describe("RegistryBinding", func() {
		It("should return correct columns and cells", func() {
			obj := &solar.RegistryBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-registrybinding",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.RegistryBindingSpec{
					TargetRef:   corev1.LocalObjectReference{Name: "my-target"},
					RegistryRef: corev1.LocalObjectReference{Name: "my-registry"},
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(4))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("Target"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("Registry"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("Age"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells).To(ConsistOf("my-registrybinding", "my-target", "my-registry", now))
		})
	})

	Describe("RenderTask", func() {
		It("should return correct columns and cells with JobSucceeded condition", func() {
			obj := &solar.RenderTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-rendertask",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.RenderTaskSpec{
					OwnerKind: "Release",
					OwnerName: "my-release",
				},
				Status: solar.RenderTaskStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "JobScheduled",
							Status: metav1.ConditionTrue,
							Reason: "JobScheduled",
						},
						{
							Type:   "JobSucceeded",
							Status: metav1.ConditionTrue,
							Reason: "JobSucceeded",
						},
					},
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(5))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("Owner Kind"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("Owner Name"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("Status"))
			Expect(table.ColumnDefinitions[4].Name).To(Equal("Age"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells).To(ConsistOf("my-rendertask", "Release", "my-release", "JobSucceeded", now))
		})

		It("should return Unknown status when no matching condition exists", func() {
			obj := &solar.RenderTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-rendertask",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.RenderTaskSpec{
					OwnerKind: "Release",
					OwnerName: "my-release",
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.Rows[0].Cells[3]).To(Equal("Unknown"))
		})

		It("should show JobScheduled when no terminal condition exists", func() {
			obj := &solar.RenderTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-rendertask",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.RenderTaskSpec{
					OwnerKind: "Release",
					OwnerName: "my-release",
				},
				Status: solar.RenderTaskStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "JobScheduled",
							Status: metav1.ConditionTrue,
							Reason: "JobScheduled",
						},
					},
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.Rows[0].Cells[3]).To(Equal("JobScheduled"))
		})

		It("should prefer JobFailed over JobScheduled", func() {
			obj := &solar.RenderTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-rendertask",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.RenderTaskSpec{
					OwnerKind: "Release",
					OwnerName: "my-release",
				},
				Status: solar.RenderTaskStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "JobScheduled",
							Status: metav1.ConditionTrue,
							Reason: "JobScheduled",
						},
						{
							Type:   "JobFailed",
							Status: metav1.ConditionTrue,
							Reason: "JobFailed",
						},
					},
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.Rows[0].Cells[3]).To(Equal("JobFailed"))
		})
	})

	Describe("Component", func() {
		It("should return correct columns and cells", func() {
			obj := &solar.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-component",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.ComponentSpec{
					Registry:   "registry.example.com",
					Repository: "charts/mychart",
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(4))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("Registry"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("Repository"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("Age"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells).To(ConsistOf("my-component", "registry.example.com", "charts/mychart", now))
		})
	})

	Describe("ComponentVersion", func() {
		It("should return correct columns and cells", func() {
			obj := &solar.ComponentVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "my-cv",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: solar.ComponentVersionSpec{
					ComponentRef: corev1.LocalObjectReference{Name: "my-component"},
					Tag:          "1.0.0",
				},
			}

			table, err := obj.ConvertToTable(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(table.ColumnDefinitions).To(HaveLen(4))
			Expect(table.ColumnDefinitions[0].Name).To(Equal("Name"))
			Expect(table.ColumnDefinitions[1].Name).To(Equal("Component Ref"))
			Expect(table.ColumnDefinitions[2].Name).To(Equal("Tag"))
			Expect(table.ColumnDefinitions[3].Name).To(Equal("Age"))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0].Cells).To(ConsistOf("my-cv", "my-component", "1.0.0", now))
		})
	})
})
