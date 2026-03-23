// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"go.opendefense.cloud/kit/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	pollingInterval      = 400 * time.Millisecond
	eventuallyTimeout    = 8 * time.Second
	consistentlyDuration = 2 * time.Second
	apiServiceTimeout    = 5 * time.Minute
)

var (
	k8sClient    client.Client
	testEnv      *envtest.Environment
	fakeRecorder *events.FakeRecorder

	ns *corev1.Namespace

	discoveryReconciler      *DiscoveryReconciler
	targetReconciler         *TargetReconciler
	releaseReconciler        *ReleaseReconciler
	hydratedTargetReconciler *HydratedTargetReconciler
	renderTaskReconciler     *RenderTaskReconciler

	ctx context.Context
)

func TestController(t *testing.T) {
	SetDefaultConsistentlyPollingInterval(pollingInterval)
	SetDefaultEventuallyPollingInterval(pollingInterval)
	SetDefaultEventuallyTimeout(eventuallyTimeout)
	SetDefaultConsistentlyDuration(consistentlyDuration)

	RegisterFailHandler(Fail)

	RunSpecs(t, "SOLAR Controller Suite")
}

var _ = BeforeSuite(func() {
	var err error

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")

	Expect(solarv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	testEnv, err = envtest.NewEnvironment(
		"go.opendefense.cloud/solar/cmd/solar-apiserver",
		[]string{},
		[]string{filepath.Join("..", "..", "test", "fixtures", "apiservice")},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(testEnv).NotTo(BeNil())

	k8sClient, err = testEnv.Start(scheme.Scheme, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(testEnv.Stop)

	Expect(testEnv.WaitUntilReadyWithTimeout(apiServiceTimeout)).To(Succeed())

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	DeferCleanup(cancel)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rendertask-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())

	// log all events to GinkgoWriter
	fakeRecorder = events.NewFakeRecorder(1)
	go func() {
		for event := range fakeRecorder.Events {
			logf.Log.Info(fmt.Sprintf("Event: %s", event))
		}
	}()

	mgr, err := ctrl.NewManager(testEnv.GetRESTConfig(), ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricserver.Options{
			BindAddress: "0",
		},
		Controller: ctrlconfig.Controller{SkipNameValidation: new(true)},
	})
	Expect(err).ToNot(HaveOccurred())

	// setup reconcilers
	discoveryReconciler = &DiscoveryReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      fakeRecorder,
		WorkerImage:   "worker",
		WorkerCommand: "start",
	}
	Expect(discoveryReconciler.SetupWithManager(mgr)).To(Succeed())

	targetReconciler = &TargetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: fakeRecorder,
	}
	Expect(targetReconciler.SetupWithManager(mgr)).To(Succeed())

	releaseReconciler = &ReleaseReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: fakeRecorder,
	}
	Expect(releaseReconciler.SetupWithManager(mgr)).To(Succeed())

	hydratedTargetReconciler = &HydratedTargetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: fakeRecorder,
	}
	Expect(hydratedTargetReconciler.SetupWithManager(mgr)).To(Succeed())

	renderTaskReconciler = &RenderTaskReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        fakeRecorder,
		RendererImage:   "image:tag",
		RendererCommand: "solar-renderer",
		RendererArgs: []string{
			"--plain-http",
		},
		PushSecretRef: &corev1.SecretReference{
			Name:      "rendertask-secret",
			Namespace: "default",
		},
		BaseURL:             "example.com",
		RendererCAConfigMap: "root-bundle",
	}
	Expect(renderTaskReconciler.SetupWithManager(mgr)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).To(Succeed(), "failed to start manager")
	}()
})

var _ = BeforeEach(func() {
	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "testns-",
		},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed(), "failed to create test namespace")

	nsName := ns.Name
	discoveryReconciler.WatchNamespace = nsName
	targetReconciler.WatchNamespace = nsName
	releaseReconciler.WatchNamespace = nsName
	hydratedTargetReconciler.WatchNamespace = nsName
	renderTaskReconciler.WatchNamespace = nsName
})

var _ = AfterEach(func() {
	discoveryReconciler.WatchNamespace = ""
	targetReconciler.WatchNamespace = ""
	releaseReconciler.WatchNamespace = ""
	hydratedTargetReconciler.WatchNamespace = ""
	renderTaskReconciler.WatchNamespace = ""

	Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
})
