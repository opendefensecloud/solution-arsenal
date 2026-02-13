// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package apiwriter

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/client-go/clientset/versioned/fake"
	solarv1alpha1 "go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestQualifier(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "APIWriter Suite")
}

var _ = Describe("APIWriter", Ordered, func() {
	var (
		ctx         context.Context
		cancel      context.CancelFunc
		writer      *APIWriter
		inputChan   chan discovery.WriteAPIResourceEvent
		errChan     chan discovery.ErrorEvent
		solarClient solarv1alpha1.SolarV1alpha1Interface
	)
	opts := []discovery.RunnerOption[discovery.WriteAPIResourceEvent, any]{
		discovery.WithLogger[discovery.WriteAPIResourceEvent, any](zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))),
	}

	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		inputChan = make(chan discovery.WriteAPIResourceEvent, 100)
		errChan = make(chan discovery.ErrorEvent, 100)
		solarClient = fake.NewClientset(&v1alpha1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{Name: discovery.SanitizeWithHash("ocm-software-toi-demo-helmdemo-0-12-0"), Namespace: "default"},
		}).SolarV1alpha1()

		writer = NewAPIWriter(solarClient, "default", inputChan, errChan, opts...)
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

	Describe("ComponentVersion Creation", func() {
		It("should create a ComponentVersion when an event is received", func() {
			Expect(writer.Start(ctx)).To(Succeed())

			inputChan <- discovery.WriteAPIResourceEvent{
				Source: discovery.ComponentVersionEvent{
					Source: discovery.RepositoryEvent{
						Registry:   "oci://example.com",
						Repository: "repository",
						Version:    "v1.0.0",
						Type:       discovery.EventCreated,
						Timestamp:  time.Now(),
					},
					Namespace: "components",
					Component: "my-component",
					Timestamp: time.Now(),
				},
				HelmDiscovery: discovery.HelmDiscovery{
					Name:        "MyChart",
					Description: "my helm chart",
					Version:     "v1.0.0",
					AppVersion:  "v1.0.0",
					Digest:      "sha256:123456789",
				},
				Timestamp: time.Now(),
			}

			select {
			case errEvent := <-errChan:
				Expect(errEvent.Error).To(Not(HaveOccurred()))
			case <-time.After(5 * time.Second):
				cv, err := solarClient.ComponentVersions("default").Get(ctx, "my-component-v1-0-0", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				Expect(cv.Spec.ComponentRef.Name).To(Equal("my-component"))
			}

		})
	})
})
