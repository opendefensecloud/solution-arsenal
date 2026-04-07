// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"net/http"
	"net/http/httptest"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.opendefense.cloud/solar/pkg/discovery"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebhookRouter", func() {
	var (
		eventChan chan discovery.RepositoryEvent
		router    *WebhookRouter
	)

	BeforeEach(func() {
		UnregisterAllHandlers()
		eventChan = make(chan discovery.RepositoryEvent, 10)
		router = NewWebhookRouter(eventChan)
	})

	AfterEach(func() {
		UnregisterAllHandlers()
	})

	Describe("NewWebhookRouter", func() {
		It("should return a non-nil router with an initialized paths map", func() {
			Expect(router).NotTo(BeNil())
			Expect(router.paths).NotTo(BeNil())
			Expect(router.paths).To(BeEmpty())
		})

		It("should store the provided event channel", func() {
			Expect(router.eventOuts).NotTo(BeNil())
		})
	})

	Describe("WithLogger", func() {
		It("should replace the default discard logger with the provided one", func() {
			Expect(router.logger).To(Equal(logr.Discard()))

			log := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
			router.WithLogger(log)

			Expect(router.logger).To(Equal(log))
		})
	})

	Describe("RegisterPath", func() {
		It("should register a handler for a known flavor and unused path", func() {
			var called bool
			registerFakeFlavor("test-flavor", &called)

			err := router.RegisterPath(&discovery.Registry{
				Flavor:      "test-flavor",
				WebhookPath: "my-registry",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(router.paths).To(HaveKey("my-registry"))
		})

		It("should return an error when the registry flavor is not registered", func() {
			err := router.RegisterPath(&discovery.Registry{
				Flavor:      "unknown",
				WebhookPath: "some-path",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown flavor"))
		})

		It("should return an error when the webhook path is already registered", func() {
			var called bool
			registerFakeFlavor("test-flavor", &called)

			err := router.RegisterPath(&discovery.Registry{
				Flavor:      "test-flavor",
				WebhookPath: "dup-path",
			})
			Expect(err).NotTo(HaveOccurred())

			err = router.RegisterPath(&discovery.Registry{
				Flavor:      "test-flavor",
				WebhookPath: "dup-path",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already exists"))
		})

		It("should allow registering different paths for the same flavor", func() {
			var called bool
			registerFakeFlavor("shared-flavor", &called)

			err := router.RegisterPath(&discovery.Registry{
				Flavor:      "shared-flavor",
				WebhookPath: "path-a",
			})
			Expect(err).NotTo(HaveOccurred())

			err = router.RegisterPath(&discovery.Registry{
				Flavor:      "shared-flavor",
				WebhookPath: "path-b",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(router.paths).To(HaveLen(2))
		})

		It("should allow registering different flavors on different paths", func() {
			var calledA, calledB bool
			registerFakeFlavor("flavor-a", &calledA)
			registerFakeFlavor("flavor-b", &calledB)

			err := router.RegisterPath(&discovery.Registry{
				Flavor:      "flavor-a",
				WebhookPath: "path-a",
			})
			Expect(err).NotTo(HaveOccurred())

			err = router.RegisterPath(&discovery.Registry{
				Flavor:      "flavor-b",
				WebhookPath: "path-b",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(router.paths).To(HaveLen(2))
		})
	})

	Describe("ServeHTTP", func() {

		DescribeTable("should return 405 Method Not Allowed for non-POST methods",
			func(method string) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(method, "/webhook/any", nil)

				router.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
			},
			Entry("GET", http.MethodGet),
			Entry("PUT", http.MethodPut),
			Entry("DELETE", http.MethodDelete),
			Entry("PATCH", http.MethodPatch),
		)

		It("should return 404 Not Found when the path does not start with /webhook", func() {
			rec := httptest.NewRecorder()
			req := newPostRequest("/other/path")

			router.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("should return 404 Not Found for the exact path /webhook without a trailing slash", func() {
			var called bool
			registerFakeFlavor("test-flavor", &called)

			err := router.RegisterPath(&discovery.Registry{
				Flavor:      "test-flavor",
				WebhookPath: "my-path",
			})
			Expect(err).NotTo(HaveOccurred())

			rec := httptest.NewRecorder()
			req := newPostRequest("/webhook")

			router.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
			Expect(called).To(BeFalse())
		})

		It("should forward a POST to /webhook/<path> to the matching registered handler", func() {
			var called bool
			registerFakeFlavor("test-flavor", &called)

			err := router.RegisterPath(&discovery.Registry{
				Flavor:      "test-flavor",
				WebhookPath: "my-reg",
			})
			Expect(err).NotTo(HaveOccurred())

			rec := httptest.NewRecorder()
			req := newPostRequest("/webhook/my-reg")

			router.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusAccepted))
			Expect(called).To(BeTrue())
		})

		It("should return 404 Not Found when no handler is registered for the given sub-path", func() {
			rec := httptest.NewRecorder()
			req := newPostRequest("/webhook/nonexistent")

			router.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("should inject the router's logger into the request context", func() {
			log := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
			router.WithLogger(log)

			var ctxLogger logr.Logger
			RegisterHandler("ctx-flavor", func(_ *discovery.Registry, _ chan<- discovery.RepositoryEvent) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctxLogger = logr.FromContextOrDiscard(r.Context())
					w.WriteHeader(http.StatusOK)
				})
			})

			err := router.RegisterPath(&discovery.Registry{
				Flavor:      "ctx-flavor",
				WebhookPath: "ctx-path",
			})
			Expect(err).NotTo(HaveOccurred())

			rec := httptest.NewRecorder()
			req := newPostRequest("/webhook/ctx-path")

			router.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(ctxLogger).NotTo(Equal(logr.Discard()))
		})

		It("should route to the correct handler when multiple paths are registered", func() {
			var calledA, calledB bool
			registerFakeFlavor("flavor-a", &calledA)
			registerFakeFlavor("flavor-b", &calledB)

			Expect(router.RegisterPath(&discovery.Registry{
				Flavor:      "flavor-a",
				WebhookPath: "path-a",
			})).To(Succeed())

			Expect(router.RegisterPath(&discovery.Registry{
				Flavor:      "flavor-b",
				WebhookPath: "path-b",
			})).To(Succeed())

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, newPostRequest("/webhook/path-b"))

			Expect(rec.Code).To(Equal(http.StatusAccepted))
			Expect(calledA).To(BeFalse())
			Expect(calledB).To(BeTrue())
		})
	})
})

// fakeHandler returns an http.Handler that records it was called and echoes back
// the given status code.
func fakeHandler(called *bool, status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*called = true
		w.WriteHeader(status)
	})
}

// registerFakeFlavor registers a fake InitHandlerFunc under the given flavor
// name in the global registeredHandlers map. The returned handler uses fakeHandler.
func registerFakeFlavor(flavor string, called *bool) {
	RegisterHandler(flavor, func(_ *discovery.Registry, _ chan<- discovery.RepositoryEvent) http.Handler {
		return fakeHandler(called, http.StatusAccepted)
	})
}

// newPostRequest creates a POST *http.Request for the given path.
func newPostRequest(path string) *http.Request {
	return httptest.NewRequest(http.MethodPost, path, nil)
}
