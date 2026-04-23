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

	Describe("rewriteRepository", func() {
		It("rewrites host", func() {
			result := rewriteRepository("ghcr.io/org/chart", "harbor.edge.dmz:443", "")
			Expect(result).To(Equal("harbor.edge.dmz:443/org/chart"))
		})

		It("rewrites host with prefix", func() {
			result := rewriteRepository("ghcr.io/org/chart", "harbor.edge.dmz:443", "mirror/")
			Expect(result).To(Equal("harbor.edge.dmz:443/mirror/org/chart"))
		})

		It("strips oci:// prefix", func() {
			result := rewriteRepository("oci://ghcr.io/org/chart", "harbor.edge.dmz:443", "")
			Expect(result).To(Equal("harbor.edge.dmz:443/org/chart"))
		})

		It("handles prefix with trailing slash", func() {
			result := rewriteRepository("ghcr.io/org/chart", "harbor.edge.dmz:443", "mirror/")
			Expect(result).To(Equal("harbor.edge.dmz:443/mirror/org/chart"))
		})

		It("handles repository without path", func() {
			result := rewriteRepository("ghcr.io", "harbor.edge.dmz:443", "mirror")
			Expect(result).To(Equal("harbor.edge.dmz:443/mirror"))
		})
	})

	Describe("resolveResources", func() {
		var (
			newBinding = func(name, registryName string, rewrite *solarv1alpha1.RegistryBindingRewrite) *solarv1alpha1.RegistryBinding {
				return &solarv1alpha1.RegistryBinding{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
					Spec: solarv1alpha1.RegistryBindingSpec{
						TargetRef:   corev1.LocalObjectReference{Name: "target1"},
						RegistryRef: corev1.LocalObjectReference{Name: registryName},
						Rewrite:     rewrite,
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

		It("resolves with rewrite binding", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{
				"chart": {Repository: "ghcr.io/org/chart", Tag: "v1.0.0"},
			}

			reg := newRegistry("harbor-edge", "harbor.edge.dmz:443", "harbor-pull-cred")
			binding := newBinding("b1", "harbor-edge", &solarv1alpha1.RegistryBindingRewrite{
				SourceEndpoint:   "ghcr.io",
				RepositoryPrefix: "mirror/",
			})

			bindings := []registryBindingInfo{{binding: binding, registry: reg}}

			resolved, err := resolveResources(resources, bindings)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveKey("chart"))
			Expect(resolved["chart"].Repository).To(Equal("harbor.edge.dmz:443/mirror/org/chart"))
			Expect(resolved["chart"].Tag).To(Equal("v1.0.0"))
			Expect(resolved["chart"].PullSecretName).To(Equal("harbor-pull-cred"))
			Expect(resolved["chart"].Insecure).To(BeFalse())
		})

		It("resolves with identity binding (no rewrite)", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{
				"chart": {Repository: "harbor.edge.dmz:443/org/chart", Tag: "v1.0.0"},
			}

			reg := newRegistry("harbor-edge", "harbor.edge.dmz:443", "harbor-pull-cred")
			binding := newBinding("b1", "harbor-edge", nil)

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
			binding := newBinding("b1", "harbor-edge", nil)

			bindings := []registryBindingInfo{{binding: binding, registry: reg}}

			_, err := resolveResources(resources, bindings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown.io"))
		})

		It("prefers rewrite over identity binding", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{
				"chart": {Repository: "ghcr.io/org/chart", Tag: "v1.0.0"},
			}

			identityReg := newRegistry("ghcr", "ghcr.io", "ghcr-pull")
			identityBinding := newBinding("b-identity", "ghcr", nil)

			rewriteReg := newRegistry("harbor", "harbor.edge:443", "harbor-pull")
			rewriteBinding := newBinding("b-rewrite", "harbor", &solarv1alpha1.RegistryBindingRewrite{
				SourceEndpoint: "ghcr.io",
			})

			bindings := []registryBindingInfo{
				{binding: identityBinding, registry: identityReg},
				{binding: rewriteBinding, registry: rewriteReg},
			}

			resolved, err := resolveResources(resources, bindings)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["chart"].Repository).To(Equal("harbor.edge:443/org/chart"))
			Expect(resolved["chart"].PullSecretName).To(Equal("harbor-pull"))
		})

		It("resolves multiple resources with different bindings", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{
				"chart1": {Repository: "ghcr.io/org/chart1", Tag: "v1.0.0"},
				"chart2": {Repository: "harbor.edge:443/org/chart2", Tag: "v2.0.0"},
			}

			reg1 := newRegistry("harbor", "harbor.edge:443", "harbor-pull")
			binding1 := newBinding("b1", "harbor", &solarv1alpha1.RegistryBindingRewrite{
				SourceEndpoint: "ghcr.io",
			})

			reg2 := newRegistry("harbor-identity", "harbor.edge:443", "harbor-pull")
			binding2 := newBinding("b2", "harbor-identity", nil)

			bindings := []registryBindingInfo{
				{binding: binding1, registry: reg1},
				{binding: binding2, registry: reg2},
			}

			resolved, err := resolveResources(resources, bindings)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["chart1"].Repository).To(Equal("harbor.edge:443/org/chart1"))
			Expect(resolved["chart2"].Repository).To(Equal("harbor.edge:443/org/chart2"))
		})

		It("handles empty resource map", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{}
			bindings := []registryBindingInfo{}

			resolved, err := resolveResources(resources, bindings)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(BeEmpty())
		})

		It("carries PlainHTTP from rewrite registry", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{
				"chart": {Repository: "ghcr.io/org/chart", Tag: "v1.0.0"},
			}

			reg := newRegistry("harbor", "harbor.local:5000", "pull-secret")
			reg.Spec.PlainHTTP = true
			binding := newBinding("b1", "harbor", &solarv1alpha1.RegistryBindingRewrite{
				SourceEndpoint: "ghcr.io",
			})

			bindings := []registryBindingInfo{{binding: binding, registry: reg}}

			resolved, err := resolveResources(resources, bindings)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["chart"].Insecure).To(BeTrue())
		})

		It("allows anonymous pull when no pullSecretName", func() {
			resources := map[string]solarv1alpha1.ResourceAccess{
				"chart": {Repository: "ghcr.io/org/chart", Tag: "v1.0.0"},
			}

			reg := newRegistry("ghcr", "ghcr.io", "")
			binding := newBinding("b1", "ghcr", nil)

			bindings := []registryBindingInfo{{binding: binding, registry: reg}}

			resolved, err := resolveResources(resources, bindings)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["chart"].PullSecretName).To(BeEmpty())
		})
	})
})
