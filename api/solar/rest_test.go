// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar_test

import (
	"context"
	"os"

	"go.opendefense.cloud/kit/apiserver/resource"
	"go.opendefense.cloud/kit/apiserver/rest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.opendefense.cloud/solar/api/solar"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// testResourceObject exercises the REST storage boilerplate that every
// resource.Object implementation provides: metadata access, scoping,
// factory methods, group/resource identity, naming, and the
// PrepareForCreate/PrepareForUpdate generation bookkeeping.
func testResourceObject[T resource.Object](
	name string,
	newObj func() T,
	resourceName string,
	newListType runtime.Object,
	singularName string,
	shortNames []string,
	mutateSpec func(old T),
) {
	Describe(name, func() {
		It("exposes its ObjectMeta, scope, factories, and group resource", func() {
			obj := newObj()

			om := obj.GetObjectMeta()
			om.Name = "test-name"
			Expect(obj.GetObjectMeta().Name).To(Equal("test-name"))

			Expect(obj.NamespaceScoped()).To(BeTrue())
			Expect(obj.New()).To(BeAssignableToTypeOf(obj))
			Expect(obj.NewList()).To(BeAssignableToTypeOf(newListType))
			Expect(obj.GetGroupResource()).To(Equal(solar.SchemeGroupVersion.WithResource(resourceName).GroupResource()))
		})

		It("returns its singular name and short names", func() {
			provider := any(newObj()).(interface {
				GetSingularName() string
				ShortNames() []string
			})
			Expect(provider.GetSingularName()).To(Equal(singularName))
			Expect(provider.ShortNames()).To(Equal(shortNames))
		})

		It("sets the generation to 1 on PrepareForCreate", func() {
			obj := newObj()
			any(obj).(rest.PrepareForCreater).PrepareForCreate(context.Background())
			Expect(obj.GetObjectMeta().Generation).To(Equal(int64(1)))
		})

		It("leaves the generation unchanged when the spec is unchanged", func() {
			obj := newObj()
			old := newObj()
			any(obj).(rest.PrepareForUpdater).PrepareForUpdate(context.Background(), old)
			Expect(obj.GetObjectMeta().Generation).To(Equal(int64(0)))
		})

		It("increments the generation when the spec changes", func() {
			obj := newObj()
			old := newObj()
			mutateSpec(old)
			any(obj).(rest.PrepareForUpdater).PrepareForUpdate(context.Background(), old)
			Expect(obj.GetObjectMeta().Generation).To(Equal(int64(1)))
		})
	})
}

var _ = Describe("REST storage boilerplate", func() {
	testResourceObject("Component",
		func() *solar.Component { return &solar.Component{} },
		"components", &solar.ComponentList{}, "component", []string{"comp"},
		func(o *solar.Component) { o.Spec.Registry = "changed" },
	)

	testResourceObject("ComponentVersion",
		func() *solar.ComponentVersion { return &solar.ComponentVersion{} },
		"componentversions", &solar.ComponentVersionList{}, "componentversion", []string{"cv"},
		func(o *solar.ComponentVersion) { o.Spec.Tag = "changed" },
	)

	testResourceObject("Profile",
		func() *solar.Profile { return &solar.Profile{} },
		"profiles", &solar.ProfileList{}, "profile", []string{"prf"},
		func(o *solar.Profile) { o.Spec.ReleaseRef.Name = "changed" },
	)

	testResourceObject("ReferenceGrant",
		func() *solar.ReferenceGrant { return &solar.ReferenceGrant{} },
		"referencegrants", &solar.ReferenceGrantList{}, "referencegrant", []string{"rg"},
		func(o *solar.ReferenceGrant) {
			o.Spec.From = []solar.ReferenceGrantFromSubject{{Group: "g", Kind: "k", Namespace: "ns"}}
		},
	)

	testResourceObject("RegistryBinding",
		func() *solar.RegistryBinding { return &solar.RegistryBinding{} },
		"registrybindings", &solar.RegistryBindingList{}, "registrybinding", []string{"rb"},
		func(o *solar.RegistryBinding) { o.Spec.TargetRef.Name = "changed" },
	)

	testResourceObject("Registry",
		func() *solar.Registry { return &solar.Registry{} },
		"registries", &solar.RegistryList{}, "registry", []string{"reg"},
		func(o *solar.Registry) { o.Spec.Hostname = "changed" },
	)

	testResourceObject("ReleaseBinding",
		func() *solar.ReleaseBinding { return &solar.ReleaseBinding{} },
		"releasebindings", &solar.ReleaseBindingList{}, "releasebinding", []string{"rlb"},
		func(o *solar.ReleaseBinding) { o.Spec.TargetRef.Name = "changed" },
	)

	testResourceObject("Release",
		func() *solar.Release { return &solar.Release{} },
		"releases", &solar.ReleaseList{}, "release", []string{"rel"},
		func(o *solar.Release) { o.Spec.UniqueName = "changed" },
	)

	testResourceObject("RenderArtifact",
		func() *solar.RenderArtifact { return &solar.RenderArtifact{} },
		"renderartifacts", &solar.RenderArtifactList{}, "renderartifact", []string{"ra"},
		func(o *solar.RenderArtifact) { o.Spec.Tag = "changed" },
	)

	testResourceObject("RenderBinding",
		func() *solar.RenderBinding { return &solar.RenderBinding{} },
		"renderbindings", &solar.RenderBindingList{}, "renderbinding", []string{"rbin"},
		func(o *solar.RenderBinding) { o.Spec.OwnerKind = "changed" },
	)

	testResourceObject("RenderTask",
		func() *solar.RenderTask { return &solar.RenderTask{} },
		"rendertasks", &solar.RenderTaskList{}, "rendertask", []string{"rt"},
		func(o *solar.RenderTask) { o.Spec.Tag = "changed" },
	)

	testResourceObject("Target",
		func() *solar.Target { return &solar.Target{} },
		"targets", &solar.TargetList{}, "target", []string{"tgt"},
		func(o *solar.Target) { o.Spec.RenderRegistryNamespace = "changed" },
	)
})

var _ = Describe("CopyStatusTo", func() {
	condition := metav1.Condition{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Done"}

	It("copies Status from a Release", func() {
		src := &solar.Release{Status: solar.ReleaseStatus{Conditions: []metav1.Condition{condition}}}
		dst := &solar.Release{}
		src.CopyStatusTo(dst)
		Expect(dst.Status).To(Equal(src.Status))
	})

	It("copies Status from a Profile", func() {
		src := &solar.Profile{Status: solar.ProfileStatus{MatchedTargets: 3}}
		dst := &solar.Profile{}
		src.CopyStatusTo(dst)
		Expect(dst.Status).To(Equal(src.Status))
	})

	It("copies Status from a RenderArtifact", func() {
		src := &solar.RenderArtifact{Status: solar.RenderArtifactStatus{ChartURL: "oci://example.com/chart:v1"}}
		dst := &solar.RenderArtifact{}
		src.CopyStatusTo(dst)
		Expect(dst.Status).To(Equal(src.Status))
	})

	It("copies Status from a RenderTask", func() {
		src := &solar.RenderTask{Status: solar.RenderTaskStatus{Conditions: []metav1.Condition{condition}}}
		dst := &solar.RenderTask{}
		src.CopyStatusTo(dst)
		Expect(dst.Status).To(Equal(src.Status))
	})

	It("copies Status from a Target", func() {
		src := &solar.Target{Status: solar.TargetStatus{BootstrapVersion: 7}}
		dst := &solar.Target{}
		src.CopyStatusTo(dst)
		Expect(dst.Status).To(Equal(src.Status))
	})
})

var _ = Describe("Registry GetURL", func() {
	It("returns an https URL by default", func() {
		r := &solar.Registry{Spec: solar.RegistrySpec{Hostname: "registry.example.com:5000"}}
		Expect(r.GetURL()).To(Equal("https://registry.example.com:5000"))
	})

	It("returns an http URL when PlainHTTP is set", func() {
		r := &solar.Registry{Spec: solar.RegistrySpec{Hostname: "registry.example.com:5000", PlainHTTP: true}}
		Expect(r.GetURL()).To(Equal("http://registry.example.com:5000"))
	})
})

var _ = Describe("RenderResult", func() {
	It("removes its directory on Close", func() {
		dir, err := os.MkdirTemp("", "render-result-*")
		Expect(err).NotTo(HaveOccurred())

		result := &solar.RenderResult{Dir: dir}
		Expect(result.Close()).To(Succeed())

		_, err = os.Stat(dir)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})
})

var _ = Describe("register.go", func() {
	It("Kind returns a group-qualified GroupKind", func() {
		Expect(solar.Kind("Component")).To(Equal(solar.SchemeGroupVersion.WithKind("Component").GroupKind()))
	})

	It("Resource returns a group-qualified GroupResource", func() {
		Expect(solar.Resource("components")).To(Equal(solar.SchemeGroupVersion.WithResource("components").GroupResource()))
	})

	It("AddToScheme registers all known types", func() {
		scheme := runtime.NewScheme()
		Expect(solar.AddToScheme(scheme)).To(Succeed())
		Expect(scheme.Recognizes(solar.SchemeGroupVersion.WithKind("Component"))).To(BeTrue())
		Expect(scheme.Recognizes(solar.SchemeGroupVersion.WithKind("Target"))).To(BeTrue())
		Expect(scheme.Recognizes(solar.SchemeGroupVersion.WithKind("RenderArtifact"))).To(BeTrue())
	})
})
