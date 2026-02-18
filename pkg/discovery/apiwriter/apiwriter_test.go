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

	event := func(typ discovery.EventType) discovery.WriteAPIResourceEvent {
		return discovery.WriteAPIResourceEvent{
			Source: discovery.ComponentVersionEvent{
				Source: discovery.RepositoryEvent{
					Registry:   "test-registry",
					Repository: "test/component-descriptors/ocm.software/toi/demo/helmdemo",
					Version:    "0.12.0",
					Type:       typ,
					Timestamp:  time.Now(),
				},
				Component: "ocm.software/toi/demo/helmdemo",
				Timestamp: time.Now(),
			},
			HelmDiscovery: discovery.HelmDiscovery{
				ResourceName: "mychart",
				Name:         "MyChart",
				Description:  "my helm chart",
				Version:      "v1.0.0",
				AppVersion:   "v1.0.0",
				Digest:       "sha256:123456789",
			},
			Timestamp: time.Now(),
			ComponentSpec: compdesc.ComponentSpec{
				ObjectMeta: compmetav1.ObjectMeta{
					Name:    "ocm.software/toi/demo/helmdemo",
					Version: "0.12.0",
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
				},
			},
		}
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
			"./bin/ocm", "transfer", "ctf", "./test/fixtures/helmdemo-ctf", fmt.Sprintf("%s/test", testRegistry.GetURL()),
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
			inputChan <- event(discovery.EventCreated)

			cv := &solarv1alpha1.ComponentVersion{}
			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				mcv, err := solarClient.ComponentVersions("default").Get(ctx, "ocm-software-toi-demo-helmdemo-0-12-0", metav1.GetOptions{})
				cv = mcv

				return err
			}).ShouldNot(HaveOccurred())

			Expect(cv.Spec.ComponentRef.Name).To(Equal("ocm-software-toi-demo-helmdemo"))

			Expect(cv.Spec.Resources).NotTo(BeNil())
			Expect(cv.Spec.Resources["mychart"].Repository).To(Equal("oci://zot.local/mychart"))
			Expect(cv.Spec.Resources["mychart"].Tag).To(Equal("v1.0.0"))

			Expect(cv.Spec.Entrypoint.Type).To(Equal(solarv1alpha1.EntrypointTypeHelm))
			Expect(cv.Spec.Entrypoint.ResourceName).To(Equal("mychart"))
		})

		It("should create a Component when an event is received and no component for componentversion exists", func() {
			Expect(writer.Start(ctx)).To(Succeed())
			inputChan <- event(discovery.EventCreated)

			c := &solarv1alpha1.Component{}
			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				mc, err := solarClient.Components("default").Get(ctx, "ocm-software-toi-demo-helmdemo", metav1.GetOptions{})
				c = mc

				return err
			}).ShouldNot(HaveOccurred())

			Expect(c.Spec.Scheme).To(Equal("http"))
			Expect(c.Spec.Repository).To(Equal("ocm.software/toi/demo/helmdemo"))
			Expect(c.Spec.Registry).To(Equal(strings.TrimPrefix(testRegistry.GetURL(), "http://")))
		})
	})

	Describe("Updates", func() {
		It("should update when an update event is received", func() {
			Expect(writer.Start(ctx)).To(Succeed())
			inputChan <- event(discovery.EventCreated)

			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err := solarClient.ComponentVersions("default").Get(ctx, "ocm-software-toi-demo-helmdemo-0-12-0", metav1.GetOptions{})

				return err
			}).ShouldNot(HaveOccurred())

			// Update Event
			ev := event(discovery.EventUpdated)
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
				cv, err := solarClient.ComponentVersions("default").Get(ctx, "ocm-software-toi-demo-helmdemo-0-12-0", metav1.GetOptions{})
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
			inputChan <- event(discovery.EventCreated)
			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err := solarClient.ComponentVersions("default").Get(ctx, "ocm-software-toi-demo-helmdemo-0-12-0", metav1.GetOptions{})
				if err != nil {
					return err
				}
				_, err = solarClient.Components("default").Get(ctx, "ocm-software-toi-demo-helmdemo", metav1.GetOptions{})

				return err
			}).ShouldNot(HaveOccurred())

			inputChan <- event(discovery.EventDeleted)
			var err error = nil
			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err = solarClient.ComponentVersions("default").Get(ctx, "ocm-software-toi-demo-helmdemo-0-12-0", metav1.GetOptions{})

				return err
			}).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err = solarClient.Components("default").Get(ctx, "ocm-software-toi-demo-helmdemo", metav1.GetOptions{})

				return err
			}).Should(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should delete ComponentVersion but keep Component when a delete event is received", func() {
			Expect(writer.Start(ctx)).To(Succeed())

			// Setup 2 componentversions referencing the same component
			ev2 := event(discovery.EventCreated)
			ev2.Source.Source.Version = "0.13.0"

			inputChan <- event(discovery.EventCreated)
			inputChan <- ev2

			Eventually(func() error {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err := solarClient.ComponentVersions("default").Get(ctx, "ocm-software-toi-demo-helmdemo-0-12-0", metav1.GetOptions{})
				if err != nil {
					return err
				}
				_, err = solarClient.ComponentVersions("default").Get(ctx, "ocm-software-toi-demo-helmdemo-0-13-0", metav1.GetOptions{})
				if err != nil {
					return err
				}
				_, err = solarClient.Components("default").Get(ctx, "ocm-software-toi-demo-helmdemo", metav1.GetOptions{})

				return err
			}).ShouldNot(HaveOccurred())

			// Remove one componentversion
			inputChan <- event(discovery.EventDeleted)
			Eventually(func() bool {
				select {
				case errEvent := <-errChan:
					Expect(errEvent.Error).NotTo(HaveOccurred())
				default:
				}
				_, err := solarClient.ComponentVersions("default").Get(ctx, "ocm-software-toi-demo-helmdemo-0-12-0", metav1.GetOptions{})

				return apierrors.IsNotFound(err)
			}).To(BeTrue())

			// Verify component is still there
			_, err := solarClient.Components("default").Get(ctx, "ocm-software-toi-demo-helmdemo", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
