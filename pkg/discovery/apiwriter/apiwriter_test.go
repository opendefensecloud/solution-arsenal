// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package apiwriter

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"ocm.software/ocm/api/ocm/compdesc"
	compmetav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociartifact"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/relativeociref"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/client-go/clientset/versioned/fake"
	solarv1alpha1client "go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/test"
	testregistry "go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestQualifier(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "APIWriter Suite")
}

// createEvent builds a WriteAPIResourceEvent for the given event type.
func createEvent(eventType discovery.EventType) discovery.WriteAPIResourceEvent {
	ev := discovery.WriteAPIResourceEvent{
		Source: discovery.ComponentVersionEvent{
			Source: discovery.RepositoryEvent{
				Registry:   "test-registry",
				Repository: "test/component-descriptors/opendefense.cloud/ocm-demo",
				Version:    "v26.4.1",
				Digest:     "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				Type:       eventType,
				Timestamp:  time.Now(),
			},
			Component: "opendefense.cloud/ocm-demo",
			Timestamp: time.Now(),
		},
		Timestamp: time.Now(),
	}

	switch eventType {
	case discovery.EventCreated, discovery.EventUpdated:
		ev.HelmDiscovery = discovery.HelmDiscovery{
			ResourceName: "mychart",
			Name:         "MyChart",
			Description:  "my helm chart",
			Version:      "v1.0.0",
			AppVersion:   "v1.0.0",
			Digest:       "sha256:123456789",
		}
		ev.ComponentSpec = compdesc.ComponentSpec{
			ObjectMeta: compmetav1.ObjectMeta{
				Name:    "opendefense.cloud/ocm-demo",
				Version: "v26.4.1",
			},
			Resources: compdesc.Resources{
				{
					ResourceMeta: compdesc.ResourceMeta{
						ElementMeta: compdesc.ElementMeta{
							Name:    "mychart",
							Version: "v1.0.0",
						},
					},
					Access: &ociartifact.AccessSpec{
						ImageReference: "oci://zot.local/mychart:v1.0.0",
					},
				},
				{
					ResourceMeta: compdesc.ResourceMeta{
						ElementMeta: compdesc.ElementMeta{
							Name:    "myimage1",
							Version: "v1.1.1",
						},
					},
					Access: &ociartifact.AccessSpec{
						ImageReference: "zot.local:443/myimage1:v1.1.1",
					},
				},
				{
					ResourceMeta: compdesc.ResourceMeta{
						ElementMeta: compdesc.ElementMeta{
							Name:    "myimage2",
							Version: "v2.2.2",
						},
					},
					Access: &relativeociref.AccessSpec{
						Reference: "myimage2:v2.2.2",
					},
				},
			},
		}
	case discovery.EventDeleted:
		// Empty ComponentSpec and HelmDiscovery — the artifact no longer
		// exists in the registry so the Handler cannot populate these.
	}

	return ev
}

var _ = Describe("APIWriter", Ordered, func() {
	var (
		ctx              context.Context
		cancel           context.CancelFunc
		writer           *APIWriter
		inputChan        chan discovery.WriteAPIResourceEvent
		errChan          chan discovery.ErrorEvent
		solarClient      solarv1alpha1client.SolarV1alpha1Interface
		registryProvider = discovery.NewRegistryProvider()
		testRegistry     *discovery.Registry
		testServer       *httptest.Server
	)
	opts := []discovery.RunnerOption[discovery.WriteAPIResourceEvent, any]{
		discovery.WithLogger[discovery.WriteAPIResourceEvent, any](zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))),
	}

	BeforeAll(func() {
		reg := testregistry.New(registry.Logger(log.New(GinkgoWriter, "registry", log.Flags())))
		registryProvider = discovery.NewRegistryProvider()
		testServer = httptest.NewServer(reg.HandleFunc())
		scheme := runtime.NewScheme()
		Expect(solarv1alpha1.AddToScheme(scheme)).Should(Succeed())

		testServerUrl, err := url.Parse(testServer.URL)
		Expect(err).NotTo(HaveOccurred())

		testRegistry = &discovery.Registry{
			Name:      "test-registry",
			Hostname:  testServerUrl.Host,
			PlainHTTP: true,
		}

		Expect(registryProvider.Register(testRegistry)).To(Succeed())

		_, err = test.Run(exec.Command(
			"./bin/ocm", "transfer", "ctf", "./test/fixtures/ocm-demo-ctf", fmt.Sprintf("%s/test", testRegistry.GetURL()),
		))
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		inputChan = make(chan discovery.WriteAPIResourceEvent, 100)
		errChan = make(chan discovery.ErrorEvent, 100)
		// nolint:staticcheck
		solarClient = fake.NewSimpleClientset().SolarV1alpha1() // FIXME: Use NewClientSet() for better field management (blocked by https://github.com/kubernetes/kubernetes/issues/126850)
		writer = NewAPIWriter(solarClient, "default", registryProvider, inputChan, errChan, opts...)
	})

	AfterEach(func() {
		writer.Stop()
		cancel()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("Start and Stop", func() {
		It("should start and stop the APIWriter gracefully", func() {
			err := writer.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			writer.Stop()
		})
	})

	Describe("Creation", func() {
		It("should create a ComponentVersion when an event is received", func() {
			Expect(writer.Start(ctx)).To(Succeed())
			inputChan <- createEvent(discovery.EventCreated)

			cv := &solarv1alpha1.ComponentVersion{}
			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				mcv, err := solarClient.ComponentVersions("default").Get(ctx, "opendefense-cloud-ocm-demo-v26-4-1", metav1.GetOptions{})
				cv = mcv

				return err
			}).ShouldNot(HaveOccurred())

			Expect(cv.Spec.ComponentRef.Name).To(Equal("opendefense-cloud-ocm-demo"))

			Expect(cv.Spec.Resources).NotTo(BeNil())
			Expect(cv.Spec.Resources["mychart"].Repository).To(Equal("zot.local/mychart"))
			Expect(cv.Spec.Resources["mychart"].Tag).To(Equal("v1.0.0"))
			Expect(cv.Spec.Resources["myimage1"].Repository).To(Equal("zot.local:443/myimage1"))
			Expect(cv.Spec.Resources["myimage1"].Tag).To(Equal("v1.1.1"))
			Expect(cv.Spec.Resources["myimage2"].Repository).To(Equal(strings.TrimPrefix(testRegistry.GetURL(), "http://") + "/myimage2"))
			Expect(cv.Spec.Resources["myimage2"].Insecure).To(BeTrue())
			Expect(cv.Spec.Resources["myimage2"].Tag).To(Equal("v2.2.2"))

			Expect(cv.Spec.Entrypoint.Type).To(Equal(solarv1alpha1.EntrypointTypeHelm))
			Expect(cv.Spec.Entrypoint.ResourceName).To(Equal("mychart"))

			// Helm metadata should be attached to the chart resource
			Expect(cv.Spec.Resources["mychart"].Helm).NotTo(BeNil())
			Expect(cv.Spec.Resources["mychart"].Helm.Name).To(Equal("MyChart"))
			Expect(cv.Spec.Resources["mychart"].Helm.Description).To(Equal("my helm chart"))
			Expect(cv.Spec.Resources["mychart"].Helm.Version).To(Equal("v1.0.0"))
			Expect(cv.Spec.Resources["mychart"].Helm.AppVersion).To(Equal("v1.0.0"))

			// Non-chart resources should not have Helm metadata
			Expect(cv.Spec.Resources["myimage1"].Helm).To(BeNil())
			Expect(cv.Spec.Resources["myimage2"].Helm).To(BeNil())
		})

		It("should store ValuesTemplate on the chart resource when present", func() {
			Expect(writer.Start(ctx)).To(Succeed())

			ev := createEvent(discovery.EventCreated)
			rendered := "image:\n  repository: registry.example.com/nginx\n  tag: 1.28.3\n"
			ev.HelmDiscovery.ValuesTemplate = &rendered
			inputChan <- ev

			cv := &solarv1alpha1.ComponentVersion{}
			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				mcv, err := solarClient.ComponentVersions("default").Get(ctx, "opendefense-cloud-ocm-demo-v26-4-1", metav1.GetOptions{})
				cv = mcv

				return err
			}).ShouldNot(HaveOccurred())

			Expect(cv.Spec.Resources["mychart"].Helm).NotTo(BeNil())
			Expect(cv.Spec.Resources["mychart"].Helm.ValuesTemplate).NotTo(BeNil())
			Expect(*cv.Spec.Resources["mychart"].Helm.ValuesTemplate).To(Equal(rendered))
		})

		It("should leave ValuesTemplate nil when not present in discovery", func() {
			Expect(writer.Start(ctx)).To(Succeed())
			inputChan <- createEvent(discovery.EventCreated)

			cv := &solarv1alpha1.ComponentVersion{}
			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				mcv, err := solarClient.ComponentVersions("default").Get(ctx, "opendefense-cloud-ocm-demo-v26-4-1", metav1.GetOptions{})
				cv = mcv

				return err
			}).ShouldNot(HaveOccurred())

			Expect(cv.Spec.Resources["mychart"].Helm).NotTo(BeNil())
			Expect(cv.Spec.Resources["mychart"].Helm.ValuesTemplate).To(BeNil())
		})

		It("should create a Component when an event is received and no component for componentversion exists", func() {
			Expect(writer.Start(ctx)).To(Succeed())
			inputChan <- createEvent(discovery.EventCreated)

			c := &solarv1alpha1.Component{}
			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				mc, err := solarClient.Components("default").Get(ctx, "opendefense-cloud-ocm-demo", metav1.GetOptions{})
				c = mc

				return err
			}).ShouldNot(HaveOccurred())

			Expect(c.Spec.Scheme).To(Equal("http"))
			Expect(c.Spec.Repository).To(Equal("opendefense.cloud/ocm-demo"))
			Expect(c.Spec.Registry).To(Equal(strings.TrimPrefix(testRegistry.GetURL(), "http://")))
		})
	})

	Describe("Updates", func() {
		It("should update when an update event is received", func() {
			Expect(writer.Start(ctx)).To(Succeed())
			inputChan <- createEvent(discovery.EventCreated)

			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err := solarClient.ComponentVersions("default").Get(ctx, "opendefense-cloud-ocm-demo-v26-4-1", metav1.GetOptions{})

				return err
			}).ShouldNot(HaveOccurred())

			// Update Event
			ev := createEvent(discovery.EventUpdated)
			ev.ComponentSpec.Resources = compdesc.Resources{
				{
					ResourceMeta: compdesc.ResourceMeta{
						ElementMeta: compdesc.ElementMeta{
							Name:    "mychart",
							Version: "v2.0.0",
						},
					},
					Access: &ociartifact.AccessSpec{
						ImageReference: "oci://zot.local/mychart:v2.0.0",
					},
				},
			}

			ev.HelmDiscovery.Version = "v2.0.0"
			inputChan <- ev

			Eventually(func() bool {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				cv, err := solarClient.ComponentVersions("default").Get(ctx, "opendefense-cloud-ocm-demo-v26-4-1", metav1.GetOptions{})
				if err != nil {
					return false
				}
				if cv.Spec.Resources == nil {
					return false
				}

				return cv.Spec.Resources["mychart"].Tag == "v2.0.0"
			}).Should(BeTrue())
		})
	})

	Describe("Deletion", func() {
		It("should delete ComponentVersion and Component when a delete event is received", func() {
			Expect(writer.Start(ctx)).To(Succeed())
			inputChan <- createEvent(discovery.EventCreated)
			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err := solarClient.ComponentVersions("default").Get(ctx, "opendefense-cloud-ocm-demo-v26-4-1", metav1.GetOptions{})
				if err != nil {
					return err
				}
				_, err = solarClient.Components("default").Get(ctx, "opendefense-cloud-ocm-demo", metav1.GetOptions{})

				return err
			}).ShouldNot(HaveOccurred())

			inputChan <- createEvent(discovery.EventDeleted)
			var err error = nil
			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err = solarClient.ComponentVersions("default").Get(ctx, "opendefense-cloud-ocm-demo-v26-4-1", metav1.GetOptions{})

				return err
			}).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err = solarClient.Components("default").Get(ctx, "opendefense-cloud-ocm-demo", metav1.GetOptions{})

				return err
			}).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should delete ComponentVersion but keep Component when a delete event is received", func() {
			Expect(writer.Start(ctx)).To(Succeed())

			// Setup 2 componentversions referencing the same component
			ev2 := createEvent(discovery.EventCreated)
			ev2.Source.Source.Version = "v26.5.0"
			ev2.Source.Source.Digest = "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"

			inputChan <- createEvent(discovery.EventCreated)
			inputChan <- ev2

			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err := solarClient.ComponentVersions("default").Get(ctx, "opendefense-cloud-ocm-demo-v26-4-1", metav1.GetOptions{})
				if err != nil {
					return err
				}
				_, err = solarClient.ComponentVersions("default").Get(ctx, "opendefense-cloud-ocm-demo-v26-5-0", metav1.GetOptions{})
				if err != nil {
					return err
				}
				_, err = solarClient.Components("default").Get(ctx, "opendefense-cloud-ocm-demo", metav1.GetOptions{})

				return err
			}).ShouldNot(HaveOccurred())

			// Remove one componentversion
			inputChan <- createEvent(discovery.EventDeleted)
			Eventually(func() bool {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err := solarClient.ComponentVersions("default").Get(ctx, "opendefense-cloud-ocm-demo-v26-4-1", metav1.GetOptions{})

				return apierrors.IsNotFound(err)
			}).To(BeTrue())

			// Verify component is still there
			_, err := solarClient.Components("default").Get(ctx, "opendefense-cloud-ocm-demo", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
