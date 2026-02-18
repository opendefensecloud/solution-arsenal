// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.opendefense.cloud/solar/pkg/discovery"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Webhook Server", Ordered, func() {
	var (
		log logr.Logger
	)

	BeforeAll(func() {
		log = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	})

	Describe("Start and Stop", func() {

		fakeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		It("should start and stop the server", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			errChan := make(chan discovery.ErrorEvent, 1)

			server := NewWebhookServer("127.0.0.1:0", fakeHandler, errChan, log)
			err := server.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer server.Stop(ctx)

			resp, err := http.Post("http://"+server.Addr, "application/json", bytes.NewBuffer([]byte{}))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(200))

			server.Stop(ctx)
			_, err = http.Post("http://"+server.Addr, "application/json", bytes.NewBuffer([]byte{}))
			Expect(err).To(HaveOccurred())

			Expect(errChan).To(BeEmpty())
		})

		It("should return an error when starting the server fails", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			errChan := make(chan discovery.ErrorEvent, 1)

			server1 := NewWebhookServer("127.0.0.1:0", fakeHandler, errChan, log)
			err := server1.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer server1.Stop(ctx)

			// Starting second server on same port is expected to fail
			server2 := NewWebhookServer(server1.Addr, fakeHandler, errChan, log)
			err = server2.Start(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("address already in use"))

			Expect(errChan).To(BeEmpty())
		})
	})
})
