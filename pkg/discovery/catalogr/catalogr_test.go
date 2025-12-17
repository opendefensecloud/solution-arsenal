// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package catalogr

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.opendefense.cloud/solar/pkg/discovery"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestCatalogr(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Catalogr Suite")
}

var _ = Describe("Catalogr", Ordered, func() {
	var (
		catalogr   *Catalogr
		eventsChan chan RegistryEvent
	)
	catalogrOptions := []Option{WithLogger(zap.New())}

	BeforeEach(func() {
		eventsChan = make(chan RegistryEvent, 100)
	})

	AfterEach(func() {
		catalogr.Stop()

		// Don't close eventsChan here since tests may still be reading from it
		// Only close it if needed in specific test
	})

	Describe("Start and Stop", func() {
		It("should start and stop the catalogr gracefully", func() {
			catalogr = NewCatalogr(testclient.NewSimpleClientset(), eventsChan, catalogrOptions...)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := catalogr.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Should be able to stop without blocking
			catalogr.Stop()
		})
	})

})
