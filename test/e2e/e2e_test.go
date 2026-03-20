// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"context"
	"fmt"
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
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespaces.
	AfterAll(func() {
		By("removing test namespace")
		cmd := exec.Command(kubectlBinary, "delete", "ns", testns)
		_, _ = run(cmd)

		By("undeploying the apiserver and controller-manager")
		cmd = exec.Command(helmBinary, "uninstall", "-n", controllerNamespace, "solar")
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

			// set up port fowarding for Zot registry to upload OCM package
			localport := getFreePort()
			stop := portForward("service/zot-discovery", localport, 443, "-n", "zot")
			defer stop()

			ocmconfig := filepath.Join(dir, "test", "fixtures", "e2e", "ocmconfig")
			helmdemoCtf := filepath.Join(dir, "test", "fixtures", "helmdemo-ctf")
			caCrt := filepath.Join(dir, "test", "fixtures", "ca.crt")
			cmd := exec.Command(ocmBinary, "--config", ocmconfig, "transfer", "ctf", helmdemoCtf, fmt.Sprintf("localhost:%d/test", localport))
			cmd.Env = append(cmd.Env, "SSL_CERT_FILE="+caCrt)
			_, err := run(cmd)
			Expect(err).NotTo(HaveOccurred())

			verifyComp := func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "comp", "-n", testns, "ocm-software-toi-demo-helmdemo", "-o", "jsonpath='{.spec.registry}'")
				_, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}

			verifyCompVers := func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "cv", "-n", testns, "ocm-software-toi-demo-helmdemo-0-12-0", "-o", "jsonpath='{.spec.componentRef.name}'")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("ocm-software-toi-demo-helmdemo"))
			}

			Eventually(func(g Gomega) {
				verifyComp(g)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				verifyCompVers(g)
			}).Should(Succeed())

			cmd = exec.Command(kubectlBinary, "delete", "discovery", "zot-webhook", "-n", testns)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())
			cmd = exec.Command(kubectlBinary, "delete", "cv", "ocm-software-toi-demo-helmdemo-0-12-0", "-n", testns)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())
			cmd = exec.Command(kubectlBinary, "delete", "comp", "ocm-software-toi-demo-helmdemo", "-n", testns)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "cv", "ocm-software-toi-demo-helmdemo-0-12-0", "-n", testns)
				_, err := run(cmd)
				g.Expect(err).To(HaveOccurred())
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "comp", "ocm-software-toi-demo-helmdemo", "-n", testns)
				_, err := run(cmd)
				g.Expect(err).To(HaveOccurred())
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "discovery", "zot-webhook", "-n", testns)
				_, err := run(cmd)
				g.Expect(err).To(HaveOccurred())
			}).Should(Succeed())

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
					"test-ocm-software-toi-demo-helmdemo-0-12-0-release",
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
					fmt.Sprintf("%s/release-test-ocm-software-toi-demo-helmdemo-0-12-0-release", testns))
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
				cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "test-ocm-software-toi-demo-helmdemo-0-12-0-release-0")
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
	})
})
