// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.opendefense.cloud/kit/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	targetReconciler           *TargetReconciler
	releaseReconciler          *ReleaseReconciler
	renderTaskReconciler       *RenderTaskReconciler
	profileReconciler          *ProfileReconciler
	renderArtifactReconciler   *RenderArtifactReconciler
	componentVersionReconciler *ComponentVersionReconciler
	releaseBindingReconciler   *ReleaseBindingReconciler
	registryBindingReconciler  *RegistryBindingReconciler

	targetAgentInstallerReconciler *TargetAgentInstallerReconciler
	fakeAgentInstaller             *stubAgentInstaller

	// fakeTagDeleter is injected into RenderArtifactReconciler so tests can
	// control OCI delete outcomes without making real network calls.
	fakeTagDeleter *stubTagDeleter

	ctx context.Context
)

func TestController(t *testing.T) {
	SetDefaultConsistentlyPollingInterval(pollingInterval)
	SetDefaultEventuallyPollingInterval(pollingInterval)
	SetDefaultEventuallyTimeout(eventuallyTimeout)
	SetDefaultConsistentlyDuration(consistentlyDuration)

	RegisterFailHandler(Fail)

	RunSpecs(t, "SOLAR Controller Suite", Label("integration"))
}

var _ = BeforeSuite(func() {
	var err error

	_ = os.Setenv("CONTROLLER_TEST_MODE", "true")
	DeferCleanup(os.Unsetenv, "CONTROLLER_TEST_MODE")

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

	// Register field indexers (must be done before controller setup)
	Expect(IndexFields(ctx, mgr)).To(Succeed())

	// setup reconcilers
	targetReconciler = &TargetReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Recorder:  fakeRecorder,
		APIReader: mgr.GetAPIReader(),
	}
	Expect(targetReconciler.SetupWithManager(mgr)).To(Succeed())

	releaseReconciler = &ReleaseReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: fakeRecorder,
	}
	Expect(releaseReconciler.SetupWithManager(mgr)).To(Succeed())

	renderTaskReconciler = &RenderTaskReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		Recorder:            fakeRecorder,
		RendererImage:       "image:tag",
		RendererCommand:     "solar-renderer",
		RendererArgs:        []string{"--plain-http"},
		RendererCAConfigMap: "root-bundle",
	}
	Expect(renderTaskReconciler.SetupWithManager(mgr)).To(Succeed())

	profileReconciler = &ProfileReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: fakeRecorder,
	}
	Expect(profileReconciler.SetupWithManager(mgr)).To(Succeed())

	fakeTagDeleter = &stubTagDeleter{}
	renderArtifactReconciler = &RenderArtifactReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Recorder:  fakeRecorder,
		APIReader: mgr.GetAPIReader(),
		DeleteTag: fakeTagDeleter.DeleteTag,
	}
	Expect(renderArtifactReconciler.SetupWithManager(mgr)).To(Succeed())

	componentVersionReconciler = &ComponentVersionReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	Expect(componentVersionReconciler.SetupWithManager(mgr)).To(Succeed())

	releaseBindingReconciler = &ReleaseBindingReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	Expect(releaseBindingReconciler.SetupWithManager(mgr)).To(Succeed())

	registryBindingReconciler = &RegistryBindingReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	Expect(registryBindingReconciler.SetupWithManager(mgr)).To(Succeed())

	fakeAgentInstaller = &stubAgentInstaller{}
	targetAgentInstallerReconciler = &TargetAgentInstallerReconciler{
		Client:    mgr.GetClient(),
		Installer: fakeAgentInstaller,
	}
	Expect(targetAgentInstallerReconciler.SetupWithManager(mgr)).To(Succeed())

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

	// Create push secret in test namespace for RenderTask tests
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rendertask-secret",
			Namespace: ns.Name,
		},
		Type: corev1.SecretTypeOpaque,
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())

	nsName := ns.Name
	targetReconciler.WatchNamespace = nsName
	releaseReconciler.WatchNamespace = nsName
	renderTaskReconciler.WatchNamespace = nsName
	profileReconciler.WatchNamespace = nsName
	renderArtifactReconciler.WatchNamespace = nsName
	componentVersionReconciler.WatchNamespace = nsName
	releaseBindingReconciler.WatchNamespace = nsName
	registryBindingReconciler.WatchNamespace = nsName
	// Reset the fake deleter state for each test
	fakeTagDeleter.reset()
})

var _ = AfterEach(func() {
	// Disable controllers from reconciling to prevent re-creation of objects during cleanup
	targetReconciler.WatchNamespace = "cleanup-disabled"
	releaseReconciler.WatchNamespace = "cleanup-disabled"
	renderTaskReconciler.WatchNamespace = "cleanup-disabled"
	profileReconciler.WatchNamespace = "cleanup-disabled"
	renderArtifactReconciler.WatchNamespace = "cleanup-disabled"
	componentVersionReconciler.WatchNamespace = "cleanup-disabled"
	releaseBindingReconciler.WatchNamespace = "cleanup-disabled"
	registryBindingReconciler.WatchNamespace = "cleanup-disabled"

	// Clean up RenderTasks in the test namespace.
	// Delete first (sets DeletionTimestamp), then force-remove finalizers via patch.
	renderTasks := &solarv1alpha1.RenderTaskList{}
	Expect(k8sClient.List(ctx, renderTasks, client.InNamespace(ns.Name))).To(Succeed())
	for i := range renderTasks.Items {
		rt := &renderTasks.Items[i]
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, rt))).To(Succeed())
		// Force-remove finalizer so the API server can GC immediately
		patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
		_ = client.IgnoreNotFound(k8sClient.Patch(ctx, rt, patch))
	}
	// Poll until all RenderTasks are gone; re-patch any that reappear
	Eventually(func() int {
		list := &solarv1alpha1.RenderTaskList{}
		if err := k8sClient.List(ctx, list, client.InNamespace(ns.Name)); err != nil {
			return -1
		}
		for i := range list.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, &list.Items[i]))
			patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
			_ = client.IgnoreNotFound(k8sClient.Patch(ctx, &list.Items[i], patch))
		}

		return len(list.Items)
	}, 30*time.Second).Should(Equal(0))

	// Clean up Targets with finalizers
	targets := &solarv1alpha1.TargetList{}
	Expect(k8sClient.List(ctx, targets, client.InNamespace(ns.Name))).To(Succeed())
	for i := range targets.Items {
		t := &targets.Items[i]
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, t))).To(Succeed())
		patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
		_ = client.IgnoreNotFound(k8sClient.Patch(ctx, t, patch))
	}
	// Wait until all Targets are gone before deleting the namespace
	Eventually(func() int {
		list := &solarv1alpha1.TargetList{}
		if err := k8sClient.List(ctx, list, client.InNamespace(ns.Name)); err != nil {
			return -1
		}
		for i := range list.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, &list.Items[i]))
			patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
			_ = client.IgnoreNotFound(k8sClient.Patch(ctx, &list.Items[i], patch))
		}

		return len(list.Items)
	}, 30*time.Second).Should(Equal(0))

	// Clean up RenderBindings
	renderBindings := &solarv1alpha1.RenderBindingList{}
	Expect(k8sClient.List(ctx, renderBindings, client.InNamespace(ns.Name))).To(Succeed())
	for i := range renderBindings.Items {
		_ = client.IgnoreNotFound(k8sClient.Delete(ctx, &renderBindings.Items[i]))
	}

	// Clean up RenderArtifacts (remove finalizers first)
	renderArtifacts := &solarv1alpha1.RenderArtifactList{}
	Expect(k8sClient.List(ctx, renderArtifacts, client.InNamespace(ns.Name))).To(Succeed())
	for i := range renderArtifacts.Items {
		_ = client.IgnoreNotFound(k8sClient.Delete(ctx, &renderArtifacts.Items[i]))
		patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
		_ = client.IgnoreNotFound(k8sClient.Patch(ctx, &renderArtifacts.Items[i], patch))
	}
	Eventually(func() int {
		list := &solarv1alpha1.RenderArtifactList{}
		if err := k8sClient.List(ctx, list, client.InNamespace(ns.Name)); err != nil {
			return -1
		}
		for i := range list.Items {
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, &list.Items[i]))
			patch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
			_ = client.IgnoreNotFound(k8sClient.Patch(ctx, &list.Items[i], patch))
		}

		return len(list.Items)
	}, 30*time.Second).Should(Equal(0))

	// Force-delete resources that now carry finalizers so the namespace can be deleted.
	finalizerPatch := client.RawPatch(types.JSONPatchType, []byte(`[{"op":"replace","path":"/metadata/finalizers","value":[]}]`))
	forceDelete := func(objs []client.Object) {
		for _, obj := range objs {
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, obj))
			_ = client.IgnoreNotFound(k8sClient.Patch(ctx, obj, finalizerPatch))
		}
	}

	releaseList := &solarv1alpha1.ReleaseList{}
	if err := k8sClient.List(ctx, releaseList, client.InNamespace(ns.Name)); err == nil {
		objs := make([]client.Object, len(releaseList.Items))
		for i := range releaseList.Items {
			objs[i] = &releaseList.Items[i]
		}
		forceDelete(objs)
	}

	profileList := &solarv1alpha1.ProfileList{}
	if err := k8sClient.List(ctx, profileList, client.InNamespace(ns.Name)); err == nil {
		objs := make([]client.Object, len(profileList.Items))
		for i := range profileList.Items {
			objs[i] = &profileList.Items[i]
		}
		forceDelete(objs)
	}

	cvList := &solarv1alpha1.ComponentVersionList{}
	if err := k8sClient.List(ctx, cvList, client.InNamespace(ns.Name)); err == nil {
		objs := make([]client.Object, len(cvList.Items))
		for i := range cvList.Items {
			objs[i] = &cvList.Items[i]
		}
		forceDelete(objs)
	}

	rbList := &solarv1alpha1.ReleaseBindingList{}
	if err := k8sClient.List(ctx, rbList, client.InNamespace(ns.Name)); err == nil {
		objs := make([]client.Object, len(rbList.Items))
		for i := range rbList.Items {
			objs[i] = &rbList.Items[i]
		}
		forceDelete(objs)
	}

	regbList := &solarv1alpha1.RegistryBindingList{}
	if err := k8sClient.List(ctx, regbList, client.InNamespace(ns.Name)); err == nil {
		objs := make([]client.Object, len(regbList.Items))
		for i := range regbList.Items {
			objs[i] = &regbList.Items[i]
		}
		forceDelete(objs)
	}

	compList := &solarv1alpha1.ComponentList{}
	if err := k8sClient.List(ctx, compList, client.InNamespace(ns.Name)); err == nil {
		objs := make([]client.Object, len(compList.Items))
		for i := range compList.Items {
			objs[i] = &compList.Items[i]
		}
		forceDelete(objs)
	}

	registryList := &solarv1alpha1.RegistryList{}
	if err := k8sClient.List(ctx, registryList, client.InNamespace(ns.Name)); err == nil {
		objs := make([]client.Object, len(registryList.Items))
		for i := range registryList.Items {
			objs[i] = &registryList.Items[i]
		}
		forceDelete(objs)
	}

	Expect(k8sClient.Delete(ctx, ns)).To(Succeed())

	targetReconciler.WatchNamespace = ""
	releaseReconciler.WatchNamespace = ""
	renderTaskReconciler.WatchNamespace = ""
	profileReconciler.WatchNamespace = ""
	renderArtifactReconciler.WatchNamespace = ""
	componentVersionReconciler.WatchNamespace = ""
	releaseBindingReconciler.WatchNamespace = ""
	registryBindingReconciler.WatchNamespace = ""
})
