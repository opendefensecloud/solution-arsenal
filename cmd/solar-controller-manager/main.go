// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/tls"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/controller"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(solarv1alpha1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr                                      string
		secureMetrics                                    bool
		metricsCertPath, metricsCertName, metricsCertKey string
		enableHTTP2                                      bool
		enableLeaderElection                             bool
		probeAddr                                        string
		pprofAddr                                        string
		prefixAllocationTimeout                          time.Duration
		volumeBindTimeout                                time.Duration
		virtualIPBindTimeout                             time.Duration
		networkInterfaceBindTimeout                      time.Duration
		tlsOpts                                          []func(*tls.Config)
		workerImage, workerCommand                       string
		rendererImage, rendererCommand                   string
		rendererArgs                                     string
		rendererBaseURL                                  string
		rendererCAConfigMap                              string
		rendererPushSecretName                           string
		podNS                                            string
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0",
		"The address the metrics endpoint binds to. "+
			"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt",
		"The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key",
		"The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics server")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.StringVar(&pprofAddr, "pprof-bind-address", "",
		"The address the Pprof endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager."+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&prefixAllocationTimeout, "prefix-allocation-timeout", 1*time.Second,
		"Time to wait until considering a pending allocation failed.")
	flag.DurationVar(&volumeBindTimeout, "volume-bind-timeout", 10*time.Second,
		"Time to wait until considering a volume bind to be failed.")
	flag.DurationVar(&virtualIPBindTimeout, "virtual-ip-bind-timeout", 10*time.Second,
		"Time to wait until considering a virtual ip bind to be failed.")
	flag.DurationVar(&networkInterfaceBindTimeout, "network-interface-bind-timeout", 10*time.Second,
		"Time to wait until considering a network interface bind to be failed.")
	flag.StringVar(&workerImage, "discovery-worker-image", "ghcr.io/opendefensecloud/solar-discovery-worker:latest",
		"The image of the discovery worker container.")
	flag.StringVar(&workerCommand, "discovery-worker-command", "/solar-discovery-worker",
		"The command of the discovery worker container.")
	flag.StringVar(&rendererImage, "renderer-image", "ghcr.io/opendefensecloud/solar-renderer:latest",
		"The image for renderer containers.")
	flag.StringVar(&rendererCommand, "renderer-command", "/solar-renderer",
		"The command for renderer containers.")
	flag.StringVar(&rendererBaseURL, "renderer-base-url", "",
		"The url to push rendered objects to.")
	flag.StringVar(&rendererCAConfigMap, "renderer-ca-configmap", "",
		"ConfigMap name containing CA bundle for registry connections.")
	flag.StringVar(&rendererArgs, "renderer-args", "",
		"Comma separated list of additional args for the renderer cli.")
	flag.StringVar(&rendererPushSecretName, "renderer-push-secret-name", "",
		"Name of the secret in each namespace containing credential information.")
	flag.StringVar(&podNS, "namespace", "default",
		"Namespace the controller-manager pod is running in.")
	flag.Parse()

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)
	ctx := ctrl.SetupSignalHandler()

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Metrics endpoint is enabled in 'config/controller/default/kustomization.yaml'. The Metrics options configure
	// the server. More info:
	//
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	//
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/controller/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/controller/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/controller/prometheus/kustomization.yaml for TLS certification.

	// Create watchers for metrics certificates
	var metricsCertWatcher *certwatcher.CertWatcher

	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	config := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Logger:                 logger,
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		HealthProbeBindAddress: probeAddr,
		PprofBindAddress:       pprofAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "solar.opendefense.cloud",
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}

	// Register controllers
	if err := (&controller.DiscoveryReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorder("discovery-controller"),
		WorkerImage:   workerImage,
		WorkerCommand: workerCommand,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "discovery")
		os.Exit(1)
	}
	if err := (&controller.TargetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorder("target-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "target")
		os.Exit(1)
	}
	if err := (&controller.HydratedTargetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorder("hydratedtarget-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "hydratedtarget")
		os.Exit(1)
	}
	if err := (&controller.ReleaseReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorder("release-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "release")
		os.Exit(1)
	}

	var rendererPushSecretRef *corev1.SecretReference
	if rendererPushSecretName != "" {
		rendererPushSecretRef = &corev1.SecretReference{
			Name:      rendererPushSecretName,
			Namespace: podNS,
		}
	} else {
		setupLog.Info("no push credentials were configured, continuing to start the controller without authentication", "controller", "rendertask")
	}
	// strings.Split("", ",") returns [""], not [], so we need to handle empty string specially
	// to avoid passing an empty arg to the renderer CLI
	var rendererArgsSlice []string
	if rendererArgs != "" {
		rendererArgsSlice = strings.Split(rendererArgs, ",")
	}
	if err := (&controller.RenderTaskReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		Recorder:            mgr.GetEventRecorder("rendertask-controller"),
		RendererImage:       rendererImage,
		RendererCommand:     rendererCommand,
		RendererArgs:        rendererArgsSlice,
		PushSecretRef:       rendererPushSecretRef,
		BaseURL:             rendererBaseURL,
		RendererCAConfigMap: rendererCAConfigMap,
		Namespace:           podNS,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "rendertask")
		os.Exit(1)
	}

	// healthz / readyz setup

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
