// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource resolution", func() {
	Describe("extractHost", func() {
		It("extracts host from simple repository", func() {
			Expect(extractHost("ghcr.io/org/repo")).To(Equal("ghcr.io"))
		})

		It("extracts host with port", func() {
			Expect(extractHost("registry.example.com:5000/charts/mychart")).To(Equal("registry.example.com:5000"))
		})

		It("strips oci:// prefix", func() {
			Expect(extractHost("oci://ghcr.io/org/repo")).To(Equal("ghcr.io"))
		})

		It("returns full string when no path", func() {
			Expect(extractHost("ghcr.io")).To(Equal("ghcr.io"))
		})
	})

	Describe("resolveResources", func() {
		var (
			newBinding = func(name, registryName string) *solarv1alpha1.RegistryBinding {
				return &solarv1alpha1.RegistryBinding{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
					Spec: solarv1alpha1.RegistryBindingSpec{
						TargetRef:   corev1.LocalObjectReference{Name: "target1"},
						RegistryRef: corev1.LocalObjectReference{Name: registryName},
					},
				}
			}

			newRegistry = func(name, hostname, pullSecret string) *solarv1alpha1.Registry {
				return &solarv1alpha1.Registry{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
					Spec: solarv1alpha1.RegistrySpec{
						Hostname:             hostname,
						TargetPullSecretName: pullSecret,
					},
				}
			}
		)

		It("resolves with identity binding", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{
				"chart": {Repository: "harbor.edge.dmz:443/org/chart", Tag: "v1.0.0"},
			}

			reg := newRegistry("harbor-edge", "harbor.edge.dmz:443", "harbor-pull-cred")
			binding := newBinding("b1", "harbor-edge")

			bindings := []registryBindingInfo{{binding: binding, registry: reg}}

			resolved, err := resolveResources(resources, bindings)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["chart"].Repository).To(Equal("harbor.edge.dmz:443/org/chart"))
			Expect(resolved["chart"].PullSecretName).To(Equal("harbor-pull-cred"))
		})

		It("returns error when no matching binding", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{
				"chart": {Repository: "unknown.io/org/chart", Tag: "v1.0.0"},
			}

			reg := newRegistry("harbor-edge", "harbor.edge.dmz:443", "harbor-pull-cred")
			binding := newBinding("b1", "harbor-edge")

			bindings := []registryBindingInfo{{binding: binding, registry: reg}}

			_, err := resolveResources(resources, bindings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown.io"))
		})

		It("handles empty resource map", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{}
			bindings := []registryBindingInfo{}

			resolved, err := resolveResources(resources, bindings)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(BeEmpty())
		})

		It("allows anonymous pull when no pullSecretName", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{
				"chart": {Repository: "ghcr.io/org/chart", Tag: "v1.0.0"},
			}

			reg := newRegistry("ghcr", "ghcr.io", "")
			binding := newBinding("b1", "ghcr")

			bindings := []registryBindingInfo{{binding: binding, registry: reg}}

			resolved, err := resolveResources(resources, bindings)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["chart"].PullSecretName).To(BeEmpty())
		})
	})
})
