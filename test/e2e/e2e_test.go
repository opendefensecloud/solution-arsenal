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
	"strings"
	"time"

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
			"--set", "renderer.image.tag=e2e")
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred())

		testns = setupTestNS()

		By("deploying registry credentials to test namespace for per-task push auth")
		applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "zot-deploy-auth.yaml"))

		By("deploying discovery credentials secret")
		applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "zot-discovery-auth.yaml"))

		By("deploying solar-discovery (webhook mode)")
		cmd = exec.Command(helmBinary, "upgrade", "--install",
			"--namespace", testns, "solar-discovery", filepath.Join(dir, "charts", "solar-discovery"),
			"--values", filepath.Join(dir, "test", "fixtures", "solar-discovery-webhook.values.yaml"),
			"--set", "namespace="+testns)
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred())

		// update discovery webhook pointer service to point to the Helm-deployed discovery service
		svc := patchYAMLFile(
			filepath.Join(dir, "test", "fixtures", "discovery-webhook-ptr-svc.yaml"),
			fmt.Sprintf(`[{"op": "replace", "path": "/spec/externalName", "value":"solar-discovery.%s.svc.cluster.local"}]`, testns),
		)
		defer func() { _ = os.Remove(svc) }()
		applyResource("zot", svc)
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespaces.
	AfterAll(func() {
		By("undeploying solar-discovery")
		cmd := exec.Command(helmBinary, "uninstall", "-n", testns, "solar-discovery")
		_, _ = run(cmd)
		cmd = exec.Command(helmBinary, "uninstall", "-n", testns, "solar-discovery-scan")
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

		It("should discover components via webhook", func() {
			By("waiting for discovery deployment to be ready")
			Eventually(func() error {
				cmd := exec.Command(kubectlBinary, "wait", "deployment/solar-discovery",
					"-n", testns, "--for=condition=Available", "--timeout=0")
				_, err := run(cmd)

				return err
			}).Should(Succeed())

			By("pushing OCM package to zot-discovery")
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
				cmd := exec.Command(kubectlBinary, "get", "cv", "-n", testns, "opendefense-cloud-ocm-demo-v26-4-1", "-o", "jsonpath='{.spec.componentRef.name}'")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("opendefense-cloud-ocm-demo"))
			}

			By("verifying Component was created via webhook discovery")
			Eventually(func(g Gomega) {
				verifyComp(g)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				verifyCompVers(g)
			}).Should(Succeed())

			// --- Delete test: remove OCI tag while webhook discovery is active ---
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
			desc, resolveErr := deleteRepo.Resolve(deleteCtx, "v26.4.1")
			Expect(resolveErr).NotTo(HaveOccurred())
			Expect(deleteRepo.Delete(deleteCtx, desc)).To(Succeed())

			By("verifying the ComponentVersion was deleted")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "wait", "--for=delete", "cv/opendefense-cloud-ocm-demo-v26-4-1", "-n", testns, "--timeout=0")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "ComponentVersion should be NotFound, got: %s", output)
			}).Should(Succeed())

			By("verifying the parent Component was also cleaned up")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "wait", "--for=delete", "comp/opendefense-cloud-ocm-demo", "-n", testns, "--timeout=0")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Component should be NotFound when last CV is removed, got: %s", output)
			}).Should(Succeed())

			// --- Scan mode test: uninstall webhook, deploy scan, re-push, verify ---
			By("uninstalling webhook discovery")
			cmd = exec.Command(helmBinary, "uninstall", "-n", testns, "solar-discovery")
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("deploying solar-discovery (scan mode)")
			cmd = exec.Command(helmBinary, "upgrade", "--install",
				"--namespace", testns, "solar-discovery-scan", filepath.Join(dir, "charts", "solar-discovery"),
				"--values", filepath.Join(dir, "test", "fixtures", "solar-discovery-scan.values.yaml"),
				"--set", "namespace="+testns)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("waiting for scan discovery deployment to be ready")
			Eventually(func() error {
				cmd := exec.Command(kubectlBinary, "wait", "deployment/solar-discovery-scan",
					"-n", testns, "--for=condition=Available", "--timeout=0")
				_, err := run(cmd)

				return err
			}).Should(Succeed())

			By("re-pushing the OCM package for scan discovery")
			cmd = exec.Command(ocmBinary, "--config", ocmconfig, "transfer", "ctf", ocmDemoCtf, fmt.Sprintf("localhost:%d/test", localport))
			cmd.Env = append(cmd.Env, "SSL_CERT_FILE="+caCrt)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying Component was created via scan discovery")
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

			By("waiting for ComponentVersionResolved condition to be set")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "release", "-n", testns,
					"test-opendefense-cloud-ocm-demo-v26-4-1-release",
					"-o", `jsonpath={.status.conditions[?(@.type=="ComponentVersionResolved")].status}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}).Should(Succeed())
		})

		It("should render a target when a target gets registered", func() {
			By("creating registry and target")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "registry.yaml"))
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "target.yaml"))

			// Verify Target creation
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "targets", "-n", testns, "cluster-1", "-o", "jsonpath={.spec.renderRegistryRef.name}")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("deploy-registry"))
			}).Should(Succeed())

			By("creating a ReleaseBinding to bind the release to the target")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "releasebinding.yaml"))

			By("verifying release RenderTask gets created for this target")
			Eventually(func(g Gomega) {
				// Use jsonpath to find render-rel-* RenderTasks owned by our target
				cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "-o",
					`jsonpath={range .items[?(@.spec.ownerName=="cluster-1")]}{.metadata.name}{"\n"}{end}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "expected at least one RenderTask owned by target cluster-1")
				g.Expect(output).To(ContainSubstring("render-rel-"))
			}).Should(Succeed())

			By("verifying the rendered bootstrap Helm chart exists in the OCI registry")
			localport := getFreePort()
			stop := portForward("service/zot-deploy", localport, 443, "-n", "zot")
			defer stop()

			zotDeploy := newZotClient(localport)

			ctx := context.Background()
			Eventually(func() error {
				repo, err := zotDeploy.Repository(ctx, fmt.Sprintf("%s/bootstrap-cluster-1", testns))
				if err != nil {
					return err
				}
				_, _, err = repo.FetchReference(ctx, "v0.0.0")

				return err
			}).Should(Succeed())
		})

		It("should create ReleaseBindings when a matching profile exists", func() {
			By("creating a second release for the profile to reference")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "profile-release.yaml"))

			By("creating the profile that matches the target")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "profile.yaml"))

			By("verifying the profile controller created a ReleaseBinding for cluster-1 referencing the profile's release")
			Eventually(func(g Gomega) {
				// Find the ReleaseBinding targeting cluster-1 and verify its releaseRef in one query
				cmd := exec.Command(kubectlBinary, "get", "releasebindings", "-n", testns, "-o",
					`jsonpath={range .items[?(@.spec.targetRef.name=="cluster-1")]}{.spec.releaseRef.name}{"\n"}{end}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("profile-ocm-demo-release"),
					"expected a ReleaseBinding for cluster-1 referencing profile-ocm-demo-release")
			}).Should(Succeed())
		})

		It("should bootstrap a cluster successfully", func() {
			By("creating regcred secret, ocirepository and helmrelease")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "regcred.yaml"))
			ocirepo := patchYAMLFile(
				filepath.Join(dir, "test", "fixtures", "e2e", "bootstrap-ocirepository.yaml"),
				fmt.Sprintf(`[{"op": "replace", "path": "/spec/url", "value":"oci://zot-deploy.zot.svc.cluster.local/%s/bootstrap-cluster-1"}]`, testns),
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

			By("waiting for the OCI repository to pick up the latest bootstrap chart version")
			// The profile test created a second ReleaseBinding which caused
			// bootstrapVersion to increment and a v0.0.1 chart to be pushed.
			// Force FluxCD to reconcile so it picks up v0.0.1 instead of v0.0.0.
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "annotate", "ocirepository", "solar-bootstrap",
					"-n", testns, "reconcile.fluxcd.io/requestedAt="+time.Now().Format(time.RFC3339Nano),
					"--overwrite")
				_, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				cmd = exec.Command(kubectlBinary, "get", "ocirepository", "solar-bootstrap",
					"-n", testns, "-o", "jsonpath={.status.artifact.revision}")
				out, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(ContainSubstring("v0.0.1"), "OCI repository has not picked up v0.0.1 yet: %s", out)
			}).Should(Succeed())

			Eventually(func() bool {
				return getStatusCondition(
					testns,
					"helmreleases.helm.toolkit.fluxcd.io/solar-bootstrap",
					"Ready")
			}).Should(BeTrue())

			By("verifying inner releases were rolled out")
			// The bootstrap chart creates one inner HelmRelease per bound release.
			// We expect two: one from the directly assigned ReleaseBinding and one
			// from the Profile-created ReleaseBinding.
			innerSelector := "helm.toolkit.fluxcd.io/name=solar-bootstrap"
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "-n", testns,
					"helmreleases.helm.toolkit.fluxcd.io",
					"-l", innerSelector,
					"-o", "jsonpath={.items[*].metadata.name}")
				out, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				names := strings.Fields(out)
				g.Expect(names).To(HaveLen(2), "expected 2 inner HelmReleases (direct + profile), got: %v", names)
			}).Should(Succeed())

			By("verifying inner releases reach ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "-n", testns,
					"helmreleases.helm.toolkit.fluxcd.io",
					"-l", innerSelector,
					"-o", "jsonpath={.items[*].status.conditions[?(@.type=='Ready')].status}")
				out, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				statuses := strings.Fields(out)
				g.Expect(statuses).To(HaveLen(2), "expected 2 ready statuses, got: %v", statuses)
				for _, status := range statuses {
					g.Expect(status).To(Equal("True"))
				}
			}).Should(Succeed())

			By("verifying workload deployments from both releases become available")
			// Each inner HelmRelease deploys its own workload. We expect two
			// deployments: one from the directly assigned release and one from
			// the profile-assigned release.
			deploySelector := "helm.toolkit.fluxcd.io/namespace=" + testns
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "deployments", "-n", testns,
					"-l", deploySelector,
					"-o", "jsonpath={.items[*].metadata.name}")
				out, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				deployments := strings.Fields(out)
				g.Expect(deployments).To(HaveLen(2),
					"expected 2 workload deployments (direct + profile release), got: %v", deployments)
			}).Should(Succeed())

			cmd := exec.Command(kubectlBinary, "wait", "-n", testns, "deployments",
				"-l", deploySelector,
				"--for=condition=Available", "--timeout=5m")
			_, err := run(cmd)
			Expect(err).NotTo(HaveOccurred(), "workload deployments did not become Available")
		})
	})
})
