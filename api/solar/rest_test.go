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

// resourceObjectCase describes the REST storage boilerplate to exercise for
// a single resource.Object implementation: metadata access, scoping, factory
// methods, group/resource identity, naming, and the
// PrepareForCreate/PrepareForUpdate generation bookkeeping.
type resourceObjectCase[T resource.Object] struct {
	// name is the Describe label for this resource type, e.g. "Component".
	name string
	// newObj returns a fresh zero-value instance of the resource type.
	newObj func() T
	// resourceName is the plural REST resource name, e.g. "components".
	resourceName string
	// newListType is a zero-value instance of the resource's list type.
	newListType runtime.Object
	// singularName is the expected GetSingularName() value, e.g. "component".
	singularName string
	// shortNames is the expected ShortNames() value, e.g. []string{"comp"}.
	shortNames []string
	// mutateSpec mutates old's spec so it differs from a fresh newObj(),
	// used to verify generation bumping on update.
	mutateSpec func(old T)
}

func testResourceObject[T resource.Object](c resourceObjectCase[T]) {
	Describe(c.name, func() {
		It("exposes its ObjectMeta, scope, factories, and group resource", func() {
			obj := c.newObj()

			om := obj.GetObjectMeta()
			om.Name = "test-name"
			Expect(obj.GetObjectMeta().Name).To(Equal("test-name"))

			Expect(obj.NamespaceScoped()).To(BeTrue())
			Expect(obj.New()).To(BeAssignableToTypeOf(obj))
			Expect(obj.NewList()).To(BeAssignableToTypeOf(c.newListType))
			Expect(obj.GetGroupResource()).To(Equal(solar.SchemeGroupVersion.WithResource(c.resourceName).GroupResource()))
		})

		It("returns its singular name and short names", func() {
			provider := any(c.newObj()).(interface {
				GetSingularName() string
				ShortNames() []string
			})
			Expect(provider.GetSingularName()).To(Equal(c.singularName))
			Expect(provider.ShortNames()).To(Equal(c.shortNames))
		})

		It("sets the generation to 1 on PrepareForCreate", func() {
			obj := c.newObj()
			any(obj).(rest.PrepareForCreater).PrepareForCreate(context.Background())
			Expect(obj.GetObjectMeta().Generation).To(Equal(int64(1)))
		})

		It("leaves the generation unchanged when the spec is unchanged", func() {
			obj := c.newObj()
			old := c.newObj()
			any(obj).(rest.PrepareForUpdater).PrepareForUpdate(context.Background(), old)
			Expect(obj.GetObjectMeta().Generation).To(Equal(int64(0)))
		})

		It("increments the generation when the spec changes", func() {
			obj := c.newObj()
			old := c.newObj()
			c.mutateSpec(old)
			any(obj).(rest.PrepareForUpdater).PrepareForUpdate(context.Background(), old)
			Expect(obj.GetObjectMeta().Generation).To(Equal(int64(1)))
		})
	})
}

var _ = Describe("REST storage boilerplate", func() {
	testResourceObject(resourceObjectCase[*solar.Component]{
		name:         "Component",
		newObj:       func() *solar.Component { return &solar.Component{} },
		resourceName: "components",
		newListType:  &solar.ComponentList{},
		singularName: "component",
		shortNames:   []string{"comp"},
		mutateSpec:   func(o *solar.Component) { o.Spec.Registry = "changed" },
	})

	testResourceObject(resourceObjectCase[*solar.ComponentVersion]{
		name:         "ComponentVersion",
		newObj:       func() *solar.ComponentVersion { return &solar.ComponentVersion{} },
		resourceName: "componentversions",
		newListType:  &solar.ComponentVersionList{},
		singularName: "componentversion",
		shortNames:   []string{"cv"},
		mutateSpec:   func(o *solar.ComponentVersion) { o.Spec.Tag = "changed" },
	})

	testResourceObject(resourceObjectCase[*solar.Profile]{
		name:         "Profile",
		newObj:       func() *solar.Profile { return &solar.Profile{} },
		resourceName: "profiles",
		newListType:  &solar.ProfileList{},
		singularName: "profile",
		shortNames:   []string{"prf"},
		mutateSpec:   func(o *solar.Profile) { o.Spec.ReleaseRef.Name = "changed" },
	})

	testResourceObject(resourceObjectCase[*solar.ReferenceGrant]{
		name:         "ReferenceGrant",
		newObj:       func() *solar.ReferenceGrant { return &solar.ReferenceGrant{} },
		resourceName: "referencegrants",
		newListType:  &solar.ReferenceGrantList{},
		singularName: "referencegrant",
		shortNames:   []string{"rg"},
		mutateSpec: func(o *solar.ReferenceGrant) {
			o.Spec.From = []solar.ReferenceGrantFromSubject{{Group: "g", Kind: "k", Namespace: "ns"}}
		},
	})

	testResourceObject(resourceObjectCase[*solar.RegistryBinding]{
		name:         "RegistryBinding",
		newObj:       func() *solar.RegistryBinding { return &solar.RegistryBinding{} },
		resourceName: "registrybindings",
		newListType:  &solar.RegistryBindingList{},
		singularName: "registrybinding",
		shortNames:   []string{"rb"},
		mutateSpec:   func(o *solar.RegistryBinding) { o.Spec.TargetRef.Name = "changed" },
	})

	testResourceObject(resourceObjectCase[*solar.Registry]{
		name:         "Registry",
		newObj:       func() *solar.Registry { return &solar.Registry{} },
		resourceName: "registries",
		newListType:  &solar.RegistryList{},
		singularName: "registry",
		shortNames:   []string{"reg"},
		mutateSpec:   func(o *solar.Registry) { o.Spec.Hostname = "changed" },
	})

	testResourceObject(resourceObjectCase[*solar.ReleaseBinding]{
		name:         "ReleaseBinding",
		newObj:       func() *solar.ReleaseBinding { return &solar.ReleaseBinding{} },
		resourceName: "releasebindings",
		newListType:  &solar.ReleaseBindingList{},
		singularName: "releasebinding",
		shortNames:   []string{"rlb"},
		mutateSpec:   func(o *solar.ReleaseBinding) { o.Spec.TargetRef.Name = "changed" },
	})

	testResourceObject(resourceObjectCase[*solar.Release]{
		name:         "Release",
		newObj:       func() *solar.Release { return &solar.Release{} },
		resourceName: "releases",
		newListType:  &solar.ReleaseList{},
		singularName: "release",
		shortNames:   []string{"rel"},
		mutateSpec:   func(o *solar.Release) { o.Spec.UniqueName = "changed" },
	})

	testResourceObject(resourceObjectCase[*solar.RenderArtifact]{
		name:         "RenderArtifact",
		newObj:       func() *solar.RenderArtifact { return &solar.RenderArtifact{} },
		resourceName: "renderartifacts",
		newListType:  &solar.RenderArtifactList{},
		singularName: "renderartifact",
		shortNames:   []string{"ra"},
		mutateSpec:   func(o *solar.RenderArtifact) { o.Spec.Tag = "changed" },
	})

	testResourceObject(resourceObjectCase[*solar.RenderBinding]{
		name:         "RenderBinding",
		newObj:       func() *solar.RenderBinding { return &solar.RenderBinding{} },
		resourceName: "renderbindings",
		newListType:  &solar.RenderBindingList{},
		singularName: "renderbinding",
		shortNames:   []string{"rbin"},
		mutateSpec:   func(o *solar.RenderBinding) { o.Spec.OwnerKind = "changed" },
	})

	testResourceObject(resourceObjectCase[*solar.RenderTask]{
		name:         "RenderTask",
		newObj:       func() *solar.RenderTask { return &solar.RenderTask{} },
		resourceName: "rendertasks",
		newListType:  &solar.RenderTaskList{},
		singularName: "rendertask",
		shortNames:   []string{"rt"},
		mutateSpec:   func(o *solar.RenderTask) { o.Spec.Tag = "changed" },
	})

	testResourceObject(resourceObjectCase[*solar.Target]{
		name:         "Target",
		newObj:       func() *solar.Target { return &solar.Target{} },
		resourceName: "targets",
		newListType:  &solar.TargetList{},
		singularName: "target",
		shortNames:   []string{"tgt"},
		mutateSpec:   func(o *solar.Target) { o.Spec.RenderRegistryNamespace = "changed" },
	})
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
