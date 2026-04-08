// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"oras.land/oras-go/v2/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// namespace where the project is deployed in
const controllerNamespace = "solar-system"

var _ = Describe("solar", Ordered, func() {
	var controllerPodName string
	var testns string
	testStart := time.Now()

	SetDefaultEventuallyTimeout(10 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)

	dir, err := getProjectDir()
	Expect(err).NotTo(HaveOccurred())

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating solar-system namespace")
		cmd := exec.Command(kubectlBinary, "create", "ns", controllerNamespace)
		_, err := run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling solar-system namespace for trust-manager")
		cmd = exec.Command(kubectlBinary, "label", "ns", controllerNamespace, "trust=enabled", "--overwrite")
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace")

		// NOTE: etcd runs as root uid, so unfortunately we can not enforce this yet
		// By("labeling the namespace to enforce the restricted security policy")
		// cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
		// 	"pod-security.kubernetes.io/enforce=restricted")
		// _, err = run(cmd)
		// Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("deploying renderer secret")
		applyResource(controllerNamespace, filepath.Join(dir, "test", "fixtures", "e2e", "zot-deploy-auth.yaml"))

		By("deploying apiserver and controller-manager")
		cmd = exec.Command(helmBinary, "upgrade", "--install",
			"--namespace", controllerNamespace, "solar", filepath.Join(dir, "charts", "solar"),
			"--values", filepath.Join(dir, "test", "fixtures", "solar.values.yaml"),
			"--set", "apiserver.image.tag=e2e",
			"--set", "controller.image.tag=e2e",
			"--set", "renderer.image.tag=e2e",
			"--set", "discovery.image.tag=e2e")
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred())

		testns = setupTestNS()

		// update discovery webhook pointer service to point to the actual discovery webhook address which has been
		// determined once the name of the test namespace has been defined
		svc := patchYAMLFile(
			filepath.Join(dir, "test", "fixtures", "discovery-webhook-ptr-svc.yaml"),
			fmt.Sprintf(`[{"op": "replace", "path": "/spec/externalName", "value":"discovery-zot-webhook.%s.svc.cluster.local"}]`, testns),
		)
		defer func() { _ = os.Remove(svc) }()
		applyResource("zot", svc)
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespaces.
	AfterAll(func() {
		By("undeploying the apiserver and controller-manager")
		cmd := exec.Command(helmBinary, "uninstall", "-n", controllerNamespace, "solar")
		_, _ = run(cmd)

		By("removing manager namespace")
		cmd = exec.Command(kubectlBinary, "delete", "ns", controllerNamespace)
		_, _ = run(cmd)
	})

	BeforeEach(func() {
		testStart = time.Now()
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command(kubectlBinary, "logs", controllerPodName, "-n", controllerNamespace, "--since", time.Since(testStart).String())
			controllerLogs, err := run(cmd)
			if err == nil {
				logf("Controller logs:\n %s", controllerLogs)
			} else {
				logf("Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command(kubectlBinary, "get", "events", "-n", controllerNamespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := run(cmd)
			if err == nil {
				logf("Kubernetes events:\n%s", eventsOutput)
			} else {
				logf("Failed to get Kubernetes events: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command(kubectlBinary, "describe", "pod", controllerPodName, "-n", controllerNamespace)
			podDescription, err := run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	// ------------------------------- E2E Test -------------------------------------

	Context("SolAr E2E", func() {
		It("should start api extension server and controller-manager successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command(kubectlBinary, "get",
					"pods", "-l", "app.kubernetes.io/component=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", controllerNamespace,
				)

				podOutput, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := getNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command(kubectlBinary, "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", controllerNamespace,
				)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())

			cmd := exec.Command(kubectlBinary, "wait", "apiservices/v1alpha1.solar.opendefense.cloud",
				"--for", "condition=Available",
				"--timeout", waitTimeout)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create a component version", func() {
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "discovery-webhook.yaml"))

			// wait for discovery webhook to be ready to handle requests
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "endpointslice", "-l", "kubernetes.io/service-name=discovery-zot-webhook", "-n", testns, "-o", "jsonpath='{.items[0].endpoints[0].conditions.ready}'")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("true"))
			}).Should(Succeed())

			// set up port forwarding for Zot registry to upload OCM package
			localport := getFreePort()
			stop := portForward("service/zot-discovery", localport, 443, "-n", "zot")
			defer stop()

			ocmconfig := filepath.Join(dir, "test", "fixtures", "e2e", "ocmconfig")
			ocmDemoCtf := filepath.Join(dir, "test", "fixtures", "ocm-demo-ctf")
			caCrt := filepath.Join(dir, "test", "fixtures", "ca.crt")
			cmd := exec.Command(ocmBinary, "--config", ocmconfig, "transfer", "ctf", ocmDemoCtf, fmt.Sprintf("localhost:%d/test", localport))
			cmd.Env = append(cmd.Env, "SSL_CERT_FILE="+caCrt)
			_, err := run(cmd)
			Expect(err).NotTo(HaveOccurred())

			verifyComp := func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "comp", "-n", testns, "opendefense-cloud-ocm-demo", "-o", "jsonpath='{.spec.registry}'")
				_, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}

			verifyCompVers := func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "cv", "-n", testns, "opendefense-cloud-ocm-demo-v26-4-0", "-o", "jsonpath='{.spec.componentRef.name}'")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("opendefense-cloud-ocm-demo"))
			}

			Eventually(func(g Gomega) {
				verifyComp(g)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				verifyCompVers(g)
			}).Should(Succeed())

			// --- Delete test: remove OCI tag while webhook discovery is active ---
			// The webhook-based discovery receives Zot events on tag deletion,
			// so this must run before the webhook discovery is torn down.

			By("starting port-forward to zot-discovery for tag deletion")
			deletePort := getFreePort()
			stopDelete := portForward("service/zot-discovery", deletePort, 443, "-n", "zot")
			defer stopDelete()

			By("deleting the OCI tag from zot-discovery")
			zotDiscovery := newZotClient(deletePort)
			ociRepoPath := "test/component-descriptors/opendefense.cloud/ocm-demo"
			deleteCtx := context.Background()
			deleteRepo, repoErr := zotDiscovery.Repository(deleteCtx, ociRepoPath)
			Expect(repoErr).NotTo(HaveOccurred())
			desc, resolveErr := deleteRepo.Resolve(deleteCtx, "v26.4.0")
			Expect(resolveErr).NotTo(HaveOccurred())
			Expect(deleteRepo.Delete(deleteCtx, desc)).To(Succeed())

			By("verifying the ComponentVersion was deleted")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "wait", "--for=delete", "cv/opendefense-cloud-ocm-demo-v26-4-0", "-n", testns, "--timeout=0")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "ComponentVersion should be NotFound, got: %s", output)
			}).Should(Succeed())

			By("verifying the parent Component was also cleaned up")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "wait", "--for=delete", "comp/opendefense-cloud-ocm-demo", "-n", testns, "--timeout=0")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Component should be NotFound when last CV is removed, got: %s", output)
			}).Should(Succeed())

			// Clean up webhook discovery
			cmd = exec.Command(kubectlBinary, "delete", "discovery", "zot-webhook", "-n", testns)
			output, err := run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete discovery resource, got: %s", output)

			By("confirming the webhook discovery resource was deleted")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "wait", "--for=delete", "discovery/zot-webhook", "-n", testns, "--timeout=0")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Discovery resource should be NotFound, got: %s", output)
			}).Should(Succeed())

			// re-push OCM package, re-create via scan for subsequent tests
			By("re-pushing the OCM package after tag deletion")
			cmd = exec.Command(ocmBinary, "--config", ocmconfig, "transfer", "ctf", ocmDemoCtf, fmt.Sprintf("localhost:%d/test", localport))
			cmd.Env = append(cmd.Env, "SSL_CERT_FILE="+caCrt)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())

			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "discovery-scan.yaml"))

			Eventually(func(g Gomega) {
				verifyComp(g)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				verifyCompVers(g)
			}).Should(Succeed())
		})

		It("should render a Helm chart when a Release is created for a ComponentVersion", func() {
			By("creating a Release for the ComponentVersion")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "release.yaml"))

			By("waiting for the rendered chart URL to be set")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "release", "-n", testns,
					"test-opendefense-cloud-ocm-demo-v26-4-0-release",
					"-o", `jsonpath={.status.chartURL}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "chartURL should be set after rendering")
			}).Should(Succeed())

			By("verifying the rendered Helm chart exists in the OCI registry")
			localport := getFreePort()
			stop := portForward("service/zot-deploy", localport, 443, "-n", "zot")
			defer stop()

			zotDeploy := newZotClient(localport)

			ctx := context.Background()
			var repo registry.Repository
			Eventually(func() error {
				var err error
				repo, err = zotDeploy.Repository(ctx,
					fmt.Sprintf("%s/release-test-opendefense-cloud-ocm-demo-v26-4-0-release", testns))
				return err
			}).Should(Succeed())

			_, _, err := repo.FetchReference(ctx, "v0.0.0")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should render a target when a target gets registered", func() {
			By("creating a target")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "target.yaml"))

			// Verify Target creation
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "targets", "-n", testns, "cluster-1", "-o", "jsonpath=\"{.spec.releases}\"")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("test-release"))
			}).Should(Succeed())

			By("verifying HydratedTarget gets created")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "hydratedtargets", "-n", testns, "cluster-1")
				_, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}).Should(Succeed())

			By("verifying RenderTask gets created")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "rendertasks", testns+"-test-opendefense-cloud-ocm-demo-v26-4-0-release-0")
				_, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}).Should(Succeed())

			By("verifying the rendered Helm chart exists in the OCI registry")
			localport := getFreePort()
			stop := portForward("service/zot-deploy", localport, 443, "-n", "zot")
			defer stop()

			zotDeploy := newZotClient(localport)

			ctx := context.Background()
			var repo registry.Repository
			Eventually(func() error {
				var err error
				repo, err = zotDeploy.Repository(ctx, fmt.Sprintf("%s/ht-cluster-1", testns))

				return err
			}).Should(Succeed())

			_, _, err = repo.FetchReference(ctx, "v0.0.0")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should add matching profiles to a hydrated target", func() {
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "profile.yaml"))

			// Verify that the profile has been added to the hydrated target
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "-n", testns, "hydratedtarget", "cluster-1", "-o", "jsonpath='{.spec.profiles.*}'")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("production"))
			}).Should(Succeed())
		})

		It("should bootstrap a cluster successfully", func() {
			By("creating regcred secret, ocirepository and helmrelease")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "regcred.yaml"))
			ocirepo := patchYAMLFile(
				filepath.Join(dir, "test", "fixtures", "e2e", "bootstrap-ocirepository.yaml"),
				fmt.Sprintf(`[{"op": "replace", "path": "/spec/url", "value":"oci://zot-deploy.zot.svc.cluster.local/%s/ht-cluster-1"}]`, testns),
			)
			defer func() { _ = os.Remove(ocirepo) }()
			applyResource(testns, ocirepo)
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "bootstrap-helmrelease.yaml"))

			By("verifying successful reconciliation of flux resources")
			Eventually(func() bool {
				return getStatusCondition(
					testns,
					"ocirepositories.source.toolkit.fluxcd.io/solar-bootstrap",
					"Ready")
			}).Should(BeTrue())
			Eventually(func() bool {
				return getStatusCondition(
					testns,
					"helmreleases.helm.toolkit.fluxcd.io/solar-bootstrap",
					"Ready")
			}).Should(BeTrue())

			By("verifying release was rolled out")
			Eventually(func() error {
				cmd := exec.Command(kubectlBinary, "get", "-n", testns, "helmreleases.helm.toolkit.fluxcd.io/solar-bootstrap-test-release")
				_, err := run(cmd)
				return err
			}).Should(Succeed())

			By("verifying inner release reaches ready")
			Eventually(func() bool {
				return getStatusCondition(
					testns,
					"helmreleases.helm.toolkit.fluxcd.io/solar-bootstrap-test-release",
					"Ready")
			}).Should(BeTrue())
		})
	})
})
