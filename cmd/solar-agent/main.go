// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solarclientset "go.opendefense.cloud/solar/client-go/clientset/versioned"
	"go.opendefense.cloud/solar/pkg/agent"
)

func main() {
	var (
		namespace           string
		interval            time.Duration
		apiserverKubeconfig string
		targetNamespace     string
		targetName          string
		renderRegistry      string
		renderRegistryNS    string
	)

	flag.StringVar(&namespace, "namespace", "", "namespace to watch for Flux release objects (\"\" for all namespaces)")
	flag.DurationVar(&interval, "interval", 30*time.Second, "poll/report interval")
	flag.StringVar(&apiserverKubeconfig, "apiserver-kubeconfig", "",
		"kubeconfig for solar-apiserver (the bootstrap credential from the agent config). "+
			"If set, the agent self-registers its own Target on startup.")
	flag.StringVar(&targetNamespace, "target-namespace", "", "tenant namespace to register the Target in")
	flag.StringVar(&targetName, "target-name", "", "name to register the Target under")
	flag.StringVar(&renderRegistry, "render-registry", "", "name of the Registry to render this target's desired state to")
	flag.StringVar(&renderRegistryNS, "render-registry-namespace", "",
		"namespace of the Registry, if different from target-namespace. Requires a ReferenceGrant "+
			"in that namespace permitting Target access from target-namespace (see ADR-012).")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	log := zap.New(zap.UseFlagOptions(&opts)).WithName("solar-agent")

	cfg, err := ctrl.GetConfig()
	if err != nil {
		log.Error(err, "loading local cluster kubeconfig")
		os.Exit(1)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "building kubernetes client")
		os.Exit(1)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "building dynamic client")
		os.Exit(1)
	}

	if apiserverKubeconfig != "" {
		if err := registerTarget(log, apiserverKubeconfig, targetNamespace, targetName, renderRegistry, renderRegistryNS); err != nil {
			log.Error(err, "self-registering target")
			os.Exit(1)
		}
	}

	a := &agent.Agent{
		Collector: &agent.Collector{Client: client, Dynamic: dyn, Namespace: namespace},
		Publisher: agent.LogPublisher{Log: log},
		Interval:  interval,
		Log:       log,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("starting solar-agent (POC)", "interval", interval, "namespace", namespace)
	a.Run(ctx)
}

func registerTarget(log logr.Logger, kubeconfigPath, namespace, name, renderRegistry, renderRegistryNamespace string) error {
	apiserverCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	solarClient, err := solarclientset.NewForConfig(apiserverCfg)
	if err != nil {
		return err
	}

	registrar := &agent.Registrar{
		Client:    solarClient,
		Namespace: namespace,
		Name:      name,
		Spec: solarv1alpha1.TargetSpec{
			RenderRegistryRef:       corev1.LocalObjectReference{Name: renderRegistry},
			RenderRegistryNamespace: renderRegistryNamespace,
		},
	}

	target, err := registrar.EnsureTarget(context.Background())
	if err != nil {
		return err
	}

	log.Info("target registered", "namespace", target.Namespace, "name", target.Name)

	return nil
}
