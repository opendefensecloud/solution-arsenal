// Copyright BWI GmbH and Solution Arsenal contributors
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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	solarclientset "go.opendefense.cloud/solar/client-go/clientset/versioned"
	"go.opendefense.cloud/solar/pkg/agent"
)

func main() {
	var (
		namespace           string
		interval            time.Duration
		maxReportAge        time.Duration
		apiserverKubeconfig string
		targetNamespace     string
		targetName          string
	)

	flag.StringVar(&namespace, "namespace", "", "namespace to watch for Flux release objects (\"\" for all namespaces)")
	flag.DurationVar(&interval, "interval", 30*time.Second, "poll/report interval")
	flag.DurationVar(&maxReportAge, "max-report-age", 0, "maximum age of report (0 = assume default 10*interval)")
	flag.StringVar(&apiserverKubeconfig, "apiserver-kubeconfig", "",
		"credential for solar-apiserver, rendered into this agent's manifests alongside the Target it belongs to")
	flag.StringVar(&targetNamespace, "target-namespace", "", "namespace of the Target this agent reports for")
	flag.StringVar(&targetName, "target-name", "", "name of the Target this agent reports for")
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

	var solarClient solarclientset.Interface
	if apiserverKubeconfig != "" {
		var err error
		solarClient, err = loadSolarClient(apiserverKubeconfig)
		if err != nil {
			log.Error(err, "loading solar-apiserver kubeconfig")
			os.Exit(1)
		}
		if err := resolveTargetWithClient(log, solarClient, targetNamespace, targetName); err != nil {
			log.Error(err, "resolving target")
			os.Exit(1)
		}
	}

	collector := &agent.Collector{Client: client, Dynamic: dyn, Namespace: namespace}

	pub := agent.Publisher(agent.LogPublisher{Log: log})
	if solarClient != nil {
		pub = agent.KubePublisher{Client: solarClient}
	}

	a := &agent.Agent{
		Collector:       collector,
		Checks:          []agent.Check{&agent.FluxCrds{Discovery: client.Discovery()}},
		Publisher:       pub,
		Interval:        interval,
		Log:             log,
		TargetNamespace: targetNamespace,
		TargetName:      targetName,
		MaxReportAge:    maxReportAge,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("starting solar-agent", "interval", interval, "namespace", namespace)
	a.Run(ctx)
}

func loadSolarClient(kubeconfigPath string) (solarclientset.Interface, error) {
	apiserverCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	solarClient, err := solarclientset.NewForConfig(apiserverCfg)
	if err != nil {
		return nil, err
	}

	return solarClient, nil
}

func resolveTargetWithClient(log logr.Logger, solarClient solarclientset.Interface, namespace, name string) error {
	resolver := &agent.TargetResolver{Client: solarClient, Namespace: namespace, Name: name}

	target, err := resolver.ResolveTarget(context.Background())
	if err != nil {
		return err
	}

	log.Info("target resolved", "namespace", target.Namespace, "name", target.Name)

	return nil
}
