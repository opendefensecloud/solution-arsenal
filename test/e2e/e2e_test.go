// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"oras.land/oras-go/v2/errdef"
)

// namespace where the project is deployed in
const controllerNamespace = "solar-system"

var _ = Describe("solar", Ordered, func() {
	var controllerPodName string
	var testns string
	var deployns string
	var registryns string
	var imageTag string
	var imageRepo string
	var ciMode bool
	var ghcrToken string
	var solarValuesFile string
	testStart := time.Now()

	SetDefaultEventuallyTimeout(10 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)

	dir, err := getProjectDir()
	Expect(err).NotTo(HaveOccurred())

	// redeploySolar upgrades the solar Helm release and waits for the
	// controller-manager rollout to complete. Optional extra --set flags are
	// appended after the base args, e.g. "--set", "controller.args.registryBindingStrict=true".
	// Must only be called from within a Ginkgo node (BeforeAll, AfterAll, It)
	// because it reads imageTag, ciMode, and solarValuesFile which are set by the outer BeforeAll.
	redeploySolar := func(extraArgs ...string) {
		GinkgoHelper()
		args := []string{
			"upgrade", "--install",
			"--namespace", controllerNamespace, "solar", filepath.Join(dir, "charts", "solar"),
			"--values", solarValuesFile,
			"--set", "apiserver.image.tag=" + imageTag,
			"--set", "controller.image.tag=" + imageTag,
			"--set", "renderer.image.tag=" + imageTag,
		}
		if ciMode {
			args = append(args, "--set", "global.imagePullSecrets[0].name=ghcr-pull-secret")
		}
		args = append(args, extraArgs...)
		cmd := exec.Command(helmBinary, args...)
		_, err := run(cmd)
		Expect(err).NotTo(HaveOccurred())

		cmd = exec.Command(kubectlBinary, "rollout", "status",
			"deployment/solar-controller-manager",
			"-n", controllerNamespace, "--timeout", waitTimeout)
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred())

		cmd = exec.Command(kubectlBinary, "wait", "apiservices/v1alpha1.solar.opendefense.cloud",
			"--for", "condition=Available",
			"--timeout", waitTimeout)
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred())

		cmd = exec.Command(kubectlBinary, "get",
			"pods", "-l", "app.kubernetes.io/component=controller-manager",
			"-o", "go-template={{ range .items }}"+
				"{{ if not .metadata.deletionTimestamp }}"+
				"{{ .metadata.name }}"+
				"{{ \"\\n\" }}{{ end }}{{ end }}",
			"-n", controllerNamespace,
		)
		podOutput, err := run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
		podNames := getNonEmptyLines(podOutput)
		Expect(podNames).To(HaveLen(1), "expected 1 controller pod running after redeploy")
		controllerPodName = podNames[0]
	}

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

		Eventually(func() error {
			cmd := exec.Command(kubectlBinary, "get", "configmap", "-n", controllerNamespace, "root-bundle")
			_, err := run(cmd)

			return err
		}).Should(Succeed())

		// NOTE: etcd runs as root uid, so unfortunately we can not enforce this yet
		// By("labeling the namespace to enforce the restricted security policy")
		// cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
		// 	"pod-security.kubernetes.io/enforce=restricted")
		// _, err = run(cmd)
		// Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("deploying renderer secret")
		applyResource(controllerNamespace, filepath.Join(dir, "test", "fixtures", "e2e", "zot-deploy-auth.yaml"))

		imageTag = os.Getenv("IMAGE_TAG")
		imageRepo = os.Getenv("REGISTRY")
		ghcrToken = os.Getenv("GHCR_TOKEN")
		ciMode = os.Getenv("E2E_IMAGE_SOURCE") == "ghcr"

		solarValuesFile = filepath.Join(dir, "test", "fixtures", "solar.values.yaml")
		if ciMode {
			solarValuesFile = filepath.Join(dir, "test", "fixtures", "solar-ci.values.yaml")
		}

		if ciMode {
			By("creating ghcr.io imagePullSecret in " + controllerNamespace)
			Expect(createPullSecret(controllerNamespace, ghcrToken)).To(Succeed())
		}

		By("deploying apiserver and controller-manager")
		redeploySolar()

		By("creating test namespaces")
		testns = setupTestNS()
		deployns = fmt.Sprintf("%s-deploy", testns)
		cmd = exec.Command(kubectlBinary, "create", "namespace", deployns)
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred())
		registryns = fmt.Sprintf("%s-registry", testns)
		cmd = exec.Command(kubectlBinary, "create", "ns", registryns)
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred())

		if ciMode {
			By("creating ghcr.io imagePullSecret in " + testns)
			Expect(createPullSecret(testns, ghcrToken)).To(Succeed())
		}

		By("deploying registry credentials to test namespace for per-task push auth")
		applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "zot-deploy-auth.yaml"))

		By("deploying discovery credentials secret")
		applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "zot-discovery-auth.yaml"))

		By("creating discovery Registry object (webhook mode)")
		applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "zot-discovery-registry-webhook.yaml"))

		By("deploying solar-discovery (webhook mode)")
		discoveryArgs := []string{
			"upgrade", "--install",
			"--namespace", testns, "solar-discovery", filepath.Join(dir, "charts", "solar-discovery"),
			"--values", filepath.Join(dir, "test", "fixtures", "solar-discovery-webhook.values.yaml"),
			"--set", "namespace=" + testns,
			"--set", "image.repository=" + imageRepo + "/solar-discovery",
			"--set", "image.tag=" + imageTag,
		}
		if ciMode {
			discoveryArgs = append(discoveryArgs, "--set", "imagePullSecrets[0].name=ghcr-pull-secret")
		}
		cmd = exec.Command(helmBinary, discoveryArgs...)
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred())

		// #623: a solar-discovery release installed without .Values.registries
		// must not own any Registry CR. Together with the "should create and
		// delete a Helm-managed Registry from .Values.registries" spec below,
		// this fixes the invariant "chart owns exactly the registries the
		// caller declared".
		By("verifying no Helm-managed Registry CR exists in an empty-registries install")
		regCmd := exec.Command(kubectlBinary, "get", "registry",
			"-n", testns,
			"-l", "app.kubernetes.io/instance=solar-discovery,app.kubernetes.io/managed-by=Helm",
			"-o", "name")
		out, err := run(regCmd)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(out)).To(BeEmpty(),
			"solar-discovery release without .Values.registries must not own any Registry CR")

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

		By("deleting test namesspaces")
		cmd = exec.Command(kubectlBinary, "delete", "ns", "--timeout", "2m", testns)
		_, _ = run(cmd)
		cmd = exec.Command(kubectlBinary, "delete", "ns", "--timeout", "2m", deployns)
		_, _ = run(cmd)
		cmd = exec.Command(kubectlBinary, "delete", "ns", "--timeout", "2m", registryns)
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
			Expect(controllerPodName).To(ContainSubstring("controller-manager"))
		})

		It("should discover components via webhook", func() {
			By("waiting for discovery deployment to be ready")
			Eventually(func() error {
				cmd := exec.Command(kubectlBinary, "wait", "deployment/solar-discovery",
					"-n", testns, "--for=condition=Available", "--timeout="+waitTimeout)
				_, err := run(cmd)

				return err
			}).Should(Succeed())

			By("pushing OCM package to zot-discovery")
			localport := getFreePort()
			stop := portForward("service/zot-discovery", localport, 443, "-n", "zot")
			defer stop()

			ocmconfig := filepath.Join(dir, "test", "fixtures", "e2e", "ocmconfig")
			ocmDemoCtf := filepath.Join(dir, "test", "fixtures", "ocm-demo-ctf")
			cmd := exec.Command(ocmBinary, "--config", ocmconfig, "transfer", "ctf", ocmDemoCtf, fmt.Sprintf("localhost:%d/test", localport))
			_, err := run(cmd)
			Expect(err).NotTo(HaveOccurred())

			verifyComp := func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "comp", "-n", testns, "opendefense-cloud-ocm-demo", "-o", "jsonpath='{.spec.registry}'")
				_, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}

			verifyCompVers := func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "cv", "-n", testns, "opendefense-cloud-ocm-demo-v26-4-2", "-o", "jsonpath='{.spec.componentRef.name}'")
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
			desc, resolveErr := deleteRepo.Resolve(deleteCtx, "v26.4.2")
			Expect(resolveErr).NotTo(HaveOccurred())
			Expect(deleteRepo.Delete(deleteCtx, desc)).To(Succeed())

			By("verifying the ComponentVersion was deleted")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "wait", "--for=delete", "cv/opendefense-cloud-ocm-demo-v26-4-2", "-n", testns, "--timeout=0")
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

			By("removing webhook discovery Registry object")
			cmd = exec.Command(kubectlBinary, "delete", "registry",
				"-n", testns, "--selector", "app=zot-discovery-webhook")
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("creating discovery Registry object (scan mode)")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "zot-discovery-registry-scan.yaml"))

			By("deploying solar-discovery (scan mode)")
			discoveryScanArgs := []string{
				"upgrade", "--install",
				"--namespace", testns, "solar-discovery-scan", filepath.Join(dir, "charts", "solar-discovery"),
				"--values", filepath.Join(dir, "test", "fixtures", "solar-discovery-scan.values.yaml"),
				"--set", "namespace=" + testns,
				"--set", "image.repository=" + imageRepo + "/solar-discovery",
				"--set", "image.tag=" + imageTag,
			}
			if ciMode {
				discoveryScanArgs = append(discoveryScanArgs, "--set", "imagePullSecrets[0].name=ghcr-pull-secret")
			}
			cmd = exec.Command(helmBinary, discoveryScanArgs...)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("waiting for scan discovery deployment to be ready")
			Eventually(func() error {
				cmd := exec.Command(kubectlBinary, "wait", "deployment/solar-discovery-scan",
					"-n", testns, "--for=condition=Available", "--timeout="+waitTimeout)
				_, err := run(cmd)

				return err
			}).Should(Succeed())

			By("re-pushing the OCM package for scan discovery")
			cmd = exec.Command(ocmBinary, "--config", ocmconfig, "transfer", "ctf", ocmDemoCtf, fmt.Sprintf("localhost:%d/test", localport))
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

		// #623 / #644: the discovery chart's .Values.registries used to be a
		// dangling reference — mentioned in NOTES.txt but no template rendered
		// anything. This spec exercises the completed implementation:
		// setting the value produces a Registry CR labelled as Helm-managed,
		// and `helm uninstall` removes it (the lifecycle coupling documented
		// under "Registry lifecycle: embedded vs standalone").
		It("should create and delete a Helm-managed Registry from .Values.registries (#623)", func() {
			embeddedRelease := "solar-discovery-embedded-test"
			embeddedNs := testns
			// A probe registry that no worker needs to actually reach — this
			// spec exercises the chart template, not the discovery loop.
			registriesJSON := `[{"hostname":"probe.example.local","scanInterval":"1h"}]`

			// Register cleanup before the install so a mid-install failure
			// still triggers it, and defensively delete the ClusterRole the
			// chart renders — it's cluster-scoped and survives namespace
			// deletion, so a prior run's aborted install can leave it
			// orphaned with stale Helm-ownership annotations that block a
			// reinstall.
			DeferCleanup(func() {
				cmd := exec.Command(helmBinary, "uninstall", "-n", embeddedNs, embeddedRelease)
				_, _ = run(cmd)
				cmd = exec.Command(kubectlBinary, "delete", "clusterrole",
					embeddedRelease, "--ignore-not-found")
				_, _ = run(cmd)
				cmd = exec.Command(kubectlBinary, "delete", "clusterrolebinding",
					embeddedRelease, "--ignore-not-found")
				_, _ = run(cmd)
			})

			By("preflight: remove any orphan cluster-scoped resources from prior runs")
			// Same cleanup as DeferCleanup above but proactive — protects
			// against a previous run's DeferCleanup that didn't fire (e.g.
			// aborted with Ctrl-C).
			cmd := exec.Command(kubectlBinary, "delete", "clusterrole",
				embeddedRelease, "--ignore-not-found")
			_, _ = run(cmd)
			cmd = exec.Command(kubectlBinary, "delete", "clusterrolebinding",
				embeddedRelease, "--ignore-not-found")
			_, _ = run(cmd)

			By("installing solar-discovery with an embedded registry entry")
			embeddedArgs := []string{
				"upgrade", "--install",
				"--namespace", embeddedNs, embeddedRelease,
				filepath.Join(dir, "charts", "solar-discovery"),
				"--set", "namespace=" + embeddedNs,
				"--set", "image.repository=" + imageRepo + "/solar-discovery",
				"--set", "image.tag=" + imageTag,
				"--set-json", "registries=" + registriesJSON,
			}
			if ciMode {
				embeddedArgs = append(embeddedArgs, "--set", "imagePullSecrets[0].name=ghcr-pull-secret")
			}
			cmd = exec.Command(helmBinary, embeddedArgs...)
			_, err := run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying the Registry CR exists with Helm ownership labels")
			// Name defaults to the sanitised hostname (non-alnum chars → `-`)
			// per templates/registries.yaml, so "probe.example.local" becomes
			// "probe-example-local".
			expectedName := "probe-example-local"
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "registry",
					"-n", embeddedNs,
					"-l", "app.kubernetes.io/instance="+embeddedRelease+",app.kubernetes.io/managed-by=Helm",
					"-o", "jsonpath={.items[*].metadata.name}")
				out, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.Fields(out)).To(ContainElement(expectedName),
					"expected Registry named %q labelled as owned by release %q, got: %q", expectedName, embeddedRelease, out)
			}).Should(Succeed())

			By("uninstalling and verifying the Registry is deleted with the release")
			cmd = exec.Command(helmBinary, "uninstall", "-n", embeddedNs, embeddedRelease)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "registry",
					"-n", embeddedNs, expectedName, "--ignore-not-found",
					"-o", "name")
				out, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(out)).To(BeEmpty(),
					"chart-managed Registry must be deleted on helm uninstall")
			}).Should(Succeed())
		})

		It("should surface the required-check message when a registry entry omits hostname (#700)", func() {
			// Regression guard for the review on #700: a missing .hostname
			// used to crash line 2 of registries.yaml with a lower/wrong-type
			// error before our required-check could fire. The template now
			// extracts $hostname with `required` at the top of the range.
			cmd := exec.Command(helmBinary, "template", "probe",
				filepath.Join(dir, "charts", "solar-discovery"),
				"--set", "namespace="+testns,
				"--set-json", `registries=[{"scanInterval":"1h"}]`,
			)
			out, err := run(cmd)
			Expect(err).To(HaveOccurred(),
				"helm template must fail when registries[].hostname is missing")
			Expect(out).To(ContainSubstring("registries[].hostname is required"),
				"failure must surface our required-check message, not an internal type error")
		})

		It("should render a Helm chart when a Release is created for a ComponentVersion", func() {
			By("creating a Release for the ComponentVersion")
			release := patchYAMLFile(
				filepath.Join(dir, "test", "fixtures", "e2e", "release.yaml"),
				fmt.Sprintf(`[{"op": "replace", "path": "/spec/targetNamespace", "value":"%s"}]`, deployns),
			)
			defer func() { _ = os.Remove(release) }()
			applyResource(testns, release)

			By("waiting for ComponentVersionResolved condition to be set")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "release", "-n", testns,
					"test-opendefense-cloud-ocm-demo-v26-4-2-release",
					"-o", `jsonpath={.status.conditions[?(@.type=="ComponentVersionResolved")].status}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}).Should(Succeed())
		})

		It("should resolve a cross-namespace ComponentVersion via ReferenceGrant", func() {
			By("creating a catalog namespace to hold the shared ComponentVersion")
			catalogNs := fmt.Sprintf("%s-catalog", testns)
			cmd := exec.Command(kubectlBinary, "create", "ns", catalogNs)
			_, err := run(cmd)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				cmd := exec.Command(kubectlBinary, "delete", "ns", "--timeout", "2m", catalogNs)
				_, _ = run(cmd)
			})

			By("creating a Component and ComponentVersion in the catalog namespace")
			applyResource(catalogNs, filepath.Join(dir, "test", "fixtures", "e2e", "componentversion.yaml"))

			By("creating a Release with a cross-namespace ComponentVersion reference (no grant yet)")
			crossRelease := patchYAMLFile(
				filepath.Join(dir, "test", "fixtures", "e2e", "release.yaml"),
				fmt.Sprintf(`[
					{"op": "replace", "path": "/metadata/name", "value": "cross-ns-cv-release"},
					{"op": "replace", "path": "/spec/componentVersionRef/name", "value": "test-opendefense-cloud-ocm-demo-v26-4-2"},
					{"op": "replace", "path": "/spec/targetNamespace", "value": %q},
					{"op": "add", "path": "/spec/componentVersionNamespace", "value": %q}
				]`, deployns, catalogNs),
			)
			defer func() { _ = os.Remove(crossRelease) }()
			applyResource(testns, crossRelease)
			DeferCleanup(func() {
				cmd := exec.Command(kubectlBinary, "delete", "release", "cross-ns-cv-release", "-n", testns, "--ignore-not-found")
				_, _ = run(cmd)
			})

			By("verifying ComponentVersionResolved=False reason=NotGranted without a ReferenceGrant")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "release", "cross-ns-cv-release", "-n", testns,
					"-o", `jsonpath={.status.conditions[?(@.type=="ComponentVersionResolved")].reason}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("NotGranted"))
			}).Should(Succeed())

			By("creating a ReferenceGrant in the catalog namespace permitting testns Releases")
			grantFile := patchYAMLFile(
				filepath.Join(dir, "test", "fixtures", "e2e", "cross-ns-cv-grant.yaml"),
				fmt.Sprintf(`[{"op": "replace", "path": "/spec/from/0/namespace", "value": %q}]`, testns),
			)
			defer func() { _ = os.Remove(grantFile) }()
			applyResource(catalogNs, grantFile)

			By("verifying ComponentVersionResolved=True once the ReferenceGrant is in place")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "release", "cross-ns-cv-release", "-n", testns,
					"-o", `jsonpath={.status.conditions[?(@.type=="ComponentVersionResolved")].status}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}).Should(Succeed())

			By("deleting the ReferenceGrant and verifying access is revoked")
			cmd = exec.Command(kubectlBinary, "delete", "referencegrant", "allow-release-cv-access", "-n", catalogNs)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "release", "cross-ns-cv-release", "-n", testns,
					"-o", `jsonpath={.status.conditions[?(@.type=="ComponentVersionResolved")].reason}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("NotGranted"))
			}).Should(Succeed())
		})

		It("should render a target when a target gets registered", func() {
			By("creating registry")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "registry.yaml"))

			By("creating RegistryBindings for pull credentials")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "registrybinding.yaml"))

			// Wait for RegistryBindings to be visible so the informer cache
			// is warm before the ReleaseBinding triggers rendering.
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "rb", "-n", testns,
					"cluster-1-deploy-registry", "cluster-1-discovery-registry",
					"-o", "jsonpath={.items[*].metadata.name}")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("cluster-1-deploy-registry"))
				g.Expect(output).To(ContainSubstring("cluster-1-discovery-registry"))
			}).Should(Succeed())

			By("creating target")
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

			By("verifying the renderer pushed the updated bootstrap chart (v0.0.1) to zot-deploy")
			profileLocalPort := getFreePort()
			profileStop := portForward("service/zot-deploy", profileLocalPort, 443, "-n", "zot")
			defer profileStop()
			zotProfileDeploy := newZotClient(profileLocalPort)
			profileCtx := context.Background()
			Eventually(func() error {
				repo, err := zotProfileDeploy.Repository(profileCtx, fmt.Sprintf("%s/bootstrap-cluster-1", testns))
				if err != nil {
					return err
				}
				_, _, err = repo.FetchReference(profileCtx, "v0.0.1")
				return err
			}).Should(Succeed())
		})

		It("should match a Target in another namespace via ReferenceGrant", func() {
			By("creating a secondary namespace to host the cross-namespace target")
			crossNs := fmt.Sprintf("%s-cross", testns)
			cmd := exec.Command(kubectlBinary, "create", "ns", crossNs)
			_, err := run(cmd)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				cmd := exec.Command(kubectlBinary, "delete", "ns", "--timeout", "2m", crossNs)
				_, _ = run(cmd)
			})

			By("deploying registry credentials into the secondary namespace")
			applyResource(crossNs, filepath.Join(dir, "test", "fixtures", "e2e", "zot-deploy-auth.yaml"))

			By("creating a Registry in the secondary namespace — cluster-2's renderRegistryRef has no namespace set, so it resolves locally")
			applyResource(crossNs, filepath.Join(dir, "test", "fixtures", "e2e", "registry.yaml"))

			By("creating a target with env=prod in the secondary namespace")
			applyResource(crossNs, filepath.Join(dir, "test", "fixtures", "e2e", "cross-ns-target.yaml"))

			By("creating a ReferenceGrant in the secondary namespace granting testns Profile and ReleaseBinding access to Targets")
			grantFile := patchYAMLFile(
				filepath.Join(dir, "test", "fixtures", "e2e", "cross-ns-referencegrant.yaml"),
				fmt.Sprintf(`[
					{"op": "replace", "path": "/spec/from/0/namespace", "value": %q},
					{"op": "replace", "path": "/spec/from/1/namespace", "value": %q}
				]`, testns, testns),
			)
			defer func() { _ = os.Remove(grantFile) }()
			applyResource(crossNs, grantFile)

			By("verifying the profile controller created a ReleaseBinding in testns for cluster-2 from the secondary namespace")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "releasebindings", "-n", testns, "-o",
					`jsonpath={range .items[?(@.spec.targetRef.name=="cluster-2")]}{.spec.targetNamespace}{"\n"}{end}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring(crossNs),
					"expected a ReleaseBinding for cluster-2 with targetNamespace=%s", crossNs)
			}).Should(Succeed())

			By("verifying the target controller found the cross-namespace ReleaseBinding and created a RenderTask for cluster-2")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", crossNs, "-o",
					`jsonpath={range .items[?(@.spec.ownerName=="cluster-2")]}{.metadata.name}{"\n"}{end}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(),
					"expected at least one RenderTask owned by cluster-2 in %s", crossNs)
			}).Should(Succeed())
		})

		It("should bootstrap a cluster using a Registry in another namespace via ReferenceGrant", func() {
			By("deploying registry credentials into the registry namespace")
			applyResource(registryns, filepath.Join(dir, "test", "fixtures", "e2e", "zot-deploy-auth.yaml"))

			By("creating the shared Registry in the registry namespace")
			applyResource(registryns, filepath.Join(dir, "test", "fixtures", "e2e", "cross-ns-registry.yaml"))

			By("creating a ReferenceGrant in the registry namespace to allow Targets from testns")
			grantFile := patchYAMLFile(
				filepath.Join(dir, "test", "fixtures", "e2e", "cross-ns-registry-grant.yaml"),
				fmt.Sprintf(`[{"op": "replace", "path": "/spec/from/0/namespace", "value": %q}]`, testns),
			)
			defer func() { _ = os.Remove(grantFile) }()
			applyResource(registryns, grantFile)

			By("creating a Target referencing the cross-namespace Registry")
			targetFile := patchYAMLFile(
				filepath.Join(dir, "test", "fixtures", "e2e", "target.yaml"),
				fmt.Sprintf(`[
					{"op": "replace", "path": "/metadata/name", "value": "cluster-cross-reg"},
					{"op": "add", "path": "/spec/renderRegistryNamespace", "value": %q},
					{"op": "replace", "path": "/spec/renderRegistryRef/name", "value": "shared-deploy-registry"}
				]`, registryns),
			)
			defer func() { _ = os.Remove(targetFile) }()
			applyResource(testns, targetFile)
			DeferCleanup(func() {
				cmd := exec.Command(kubectlBinary, "delete", "target", "cluster-cross-reg", "-n", testns, "--ignore-not-found")
				_, _ = run(cmd)
				cmd = exec.Command(kubectlBinary, "delete", "releasebinding", "cluster-cross-reg-binding", "-n", testns, "--ignore-not-found")
				_, _ = run(cmd)
			})

			By("verifying the Target has RegistryResolved=True")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "target", "cluster-cross-reg", "-n", testns,
					"-o", `jsonpath={.status.conditions[?(@.type=="RegistryResolved")].status}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"),
					"expected RegistryResolved=True for target using cross-namespace registry")
			}).Should(Succeed())

			By("creating a ReleaseBinding to trigger rendering via the cross-namespace registry")
			bindingFile := patchYAMLFile(
				filepath.Join(dir, "test", "fixtures", "e2e", "releasebinding.yaml"),
				`[
					{"op": "replace", "path": "/metadata/name", "value": "cluster-cross-reg-binding"},
					{"op": "replace", "path": "/spec/targetRef/name", "value": "cluster-cross-reg"}
				]`,
			)
			defer func() { _ = os.Remove(bindingFile) }()
			applyResource(testns, bindingFile)

			By("verifying a RenderTask is created for the cross-namespace registry target")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "-o",
					`jsonpath={range .items[?(@.spec.ownerName=="cluster-cross-reg")]}{.metadata.name}{"\n"}{end}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(),
					"expected at least one RenderTask owned by cluster-cross-reg")
			}).Should(Succeed())

			By("verifying the RenderTask uses the shared registry hostname")
			Eventually(func(g Gomega) {
				cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "-o",
					`jsonpath={range .items[?(@.spec.ownerName=="cluster-cross-reg")]}{.spec.baseURL}{"\n"}{end}`)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("zot-deploy.zot.svc.cluster.local"),
					"expected RenderTask to use the shared registry hostname")
			}).Should(Succeed())

			By("verifying the bootstrap chart is pushed to the shared OCI registry")
			localport := getFreePort()
			stop := portForward("service/zot-deploy", localport, 443, "-n", "zot")
			defer stop()

			zotDeploy := newZotClient(localport)
			ctx := context.Background()
			Eventually(func() error {
				repo, err := zotDeploy.Repository(ctx, fmt.Sprintf("%s/bootstrap-cluster-cross-reg", testns))
				if err != nil {
					return err
				}
				_, _, err = repo.FetchReference(ctx, "v0.0.0")

				return err
			}).Should(Succeed())
		})

		It("should bootstrap a cluster successfully", func() {
			By("creating regcred secret, ocirepository and helmrelease")
			applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "regcred.yaml"))
			applyResource(deployns, filepath.Join(dir, "test", "fixtures", "e2e", "regcred.yaml"))
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
			// The profile test already waited for v0.0.1 to be pushed by the renderer.
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

			// Get the HelmReleases created by the ocm-demo component
			cmd := exec.Command(kubectlBinary, "get", "helmreleases.helm.toolkit.fluxcd.io", "-n", testns,
				"-l", "solar.opendefense.cloud/component=opendefense-cloud-ocm-demo",
				"-o", "jsonpath={.items[*].metadata.name}")
			out, err := run(cmd)
			Expect(err).NotTo(HaveOccurred())

			deploymentHelmReleases := strings.Fields(out)
			Expect(deploymentHelmReleases).To(HaveLen(2),
				"did not find expected 2 HelmReleases, got %d", len(deploymentHelmReleases))

			type foundDeployment struct {
				Name      string
				Namespace string
			}

			foundDeployments := make(map[string]foundDeployment)
			Eventually(func(g Gomega) {
				for _, helmrelease := range deploymentHelmReleases {
					// Determine in which namespace to look for the deployment
					cmd := exec.Command(kubectlBinary, "get", "helmreleases.helm.toolkit.fluxcd.io", "-n", testns, helmrelease,
						"-o", "jsonpath={.spec.targetNamespace}")
					out, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())

					// `.spec.targetNamespace` is an optional field used to specify the namespace to which the Helm release is made.
					// It defaults to the namespace of the HelmRelease.
					// Source: <https://fluxcd.io/flux/components/helm/helmreleases/#target-namespace>
					targetns := out
					if targetns == "" {
						targetns = testns
					}

					// Get the deployment from the target namespace
					deploySelector := fmt.Sprintf("helm.toolkit.fluxcd.io/name=%s", helmrelease)
					cmd = exec.Command(kubectlBinary, "get", "deployments", "-n", targetns,
						"-l", deploySelector,
						"-o", "jsonpath={.items[*].metadata.name}")
					out, err = run(cmd)
					g.Expect(err).NotTo(HaveOccurred())

					deployments := strings.Fields(out)
					g.Expect(deployments).To(HaveLen(1),
						"did not find exactly one deployment for %s, got %d", helmrelease, len(deployments))

					foundDeployments[helmrelease] = foundDeployment{
						Name:      deployments[0],
						Namespace: targetns,
					}
				}
				g.Expect(foundDeployments).To(HaveLen(2), "did not find expected 2 deployments, got %d", len(foundDeployments))
			}).Should(Succeed())

			// Verify Deployments become available
			for _, deploy := range foundDeployments {
				cmd := exec.Command(kubectlBinary, "wait", "deployments",
					"-n", deploy.Namespace, deploy.Name,
					"--for=condition=Available", "--timeout=5m")
				_, err := run(cmd)
				Expect(err).NotTo(HaveOccurred(), "workload deployment %s/%s did not become Available", deploy.Namespace, deploy.Name)
			}
		})

		// ------ RenderArtifact / RenderBinding lifecycle ------
		// These ordered specs verify the entire artifact ref-counting and GC cycle:
		//   target-1 release -> rendered -> 1 artifact + 1 binding
		//   target-2 same release -> same artifact + 2 bindings
		//   remove binding-1 -> artifact alive, 1 binding remains
		//   remove target-2 -> 0 bindings -> artifact GC'd + OCI tag deleted

		Context("RenderArtifact and RenderBinding lifecycle", Ordered, func() {
			// artName is the deterministic RenderArtifact name discovered after the first render.
			// artTag is the OCI tag captured before the last cleanup, used for the OCI assertion.
			var artName string
			var artTag string

			// releaseRepo is the OCI repository path for the art-lifecycle-release chart.
			// Evaluated lazily because testns is only set during test execution.
			releaseRepo := func() string {
				return fmt.Sprintf("%s/%s/release-art-lifecycle-release", testns, testns)
			}

			AfterAll(func() {
				// cleanup
				for _, res := range [][]string{
					{"releasebinding", "art-rb-1"},
					{"releasebinding", "art-rb-2"},
					{"releasebinding", "art-rb-3"},
					{"target", "art-tgt-1"},
					{"target", "art-tgt-2"},
					{"target", "art-tgt-3"},
					{"release", "art-lifecycle-release"},
					{"registry", "art-deploy-registry"},
				} {
					cmd := exec.Command(kubectlBinary, "delete", res[0], res[1],
						"-n", testns, "--ignore-not-found", "--timeout=2m")
					_, _ = run(cmd)
				}
			})

			It("should create 1 RenderArtifact and 1 RenderBinding when a Release renders for the first target", func() {
				By("creating art-deploy-registry Registry backed by zot-deploy")
				artRegFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "registry.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "art-deploy-registry"},
						{"op": "replace", "path": "/spec/solarSecretRef/name", "value": "zot-deploy-auth"}
					]`,
				)
				defer func() { _ = os.Remove(artRegFile) }()
				applyResource(testns, artRegFile)

				By("creating art-lifecycle-release pointing to the already-discovered ComponentVersion")
				artRelFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "release.yaml"),
					fmt.Sprintf(`[
						{"op": "replace", "path": "/metadata/name", "value": "art-lifecycle-release"},
						{"op": "replace", "path": "/spec/uniqueName", "value": "art-lifecycle"},
						{"op": "replace", "path": "/spec/targetNamespace", "value": %q}
					]`, testns),
				)
				defer func() { _ = os.Remove(artRelFile) }()
				applyResource(testns, artRelFile)

				By("creating art-tgt-1 Target using art-deploy-registry")
				tgt1File := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "target.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "art-tgt-1"},
						{"op": "replace", "path": "/spec/renderRegistryRef/name", "value": "art-deploy-registry"}
					]`,
				)
				defer func() { _ = os.Remove(tgt1File) }()
				applyResource(testns, tgt1File)

				By("creating art-rb-1 ReleaseBinding (art-tgt-1 -> art-lifecycle-release)")
				rb1File := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "releasebinding.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "art-rb-1"},
						{"op": "replace", "path": "/spec/targetRef/name", "value": "art-tgt-1"},
						{"op": "replace", "path": "/spec/releaseRef/name", "value": "art-lifecycle-release"}
					]`,
				)
				defer func() { _ = os.Remove(rb1File) }()
				applyResource(testns, rb1File)

				By("waiting for the release RenderTask for art-tgt-1 to succeed (renderer pushes chart)")
				// The jsonpath query selects RenderTasks owned by art-tgt-1 and emits
				// "repository=condition_status" so we can match the right task.
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "-o",
						`jsonpath={range .items[?(@.spec.ownerName=="art-tgt-1")]}{.spec.repository}{"="}{.status.conditions[?(@.type=="JobSucceeded")].status}{"\n"}{end}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("release-art-lifecycle-release=True"),
						"release RenderTask for art-tgt-1 not yet succeeded; output: %s", output)
				}).Should(Succeed())

				By("verifying exactly 1 RenderArtifact exists for the art-lifecycle-release chart")
				Eventually(func(g Gomega) {
					arts := getRenderArtifactsByRepo(testns, releaseRepo())
					g.Expect(arts).To(HaveLen(1),
						"expected 1 RenderArtifact for %s, got: %v", releaseRepo(), arts)
					artName = arts[0]
				}).Should(Succeed())

				By("verifying exactly 1 RenderBinding references the artifact (art-tgt-1's binding)")
				Eventually(func(g Gomega) {
					bindings := getRenderBindingsByArtifact(testns, artName)
					g.Expect(bindings).To(HaveLen(1),
						"expected 1 RenderBinding for artifact %s, got: %v", artName, bindings)
				}).Should(Succeed())

				By("verifying the RenderArtifact carries a populated status.chartURL")
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "renderartifact", artName,
						"-n", testns, "-o", "jsonpath={.status.chartURL}")
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(HavePrefix("oci://"),
						"expected chartURL to start with oci://, got: %s", output)
				}).Should(Succeed())
			})

			It("should reuse the same RenderArtifact and add a second RenderBinding for a second target", func() {
				By("creating art-tgt-2 Target using art-deploy-registry")
				tgt2File := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "target.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "art-tgt-2"},
						{"op": "replace", "path": "/spec/renderRegistryRef/name", "value": "art-deploy-registry"}
					]`,
				)
				defer func() { _ = os.Remove(tgt2File) }()
				applyResource(testns, tgt2File)

				By("creating art-rb-2 ReleaseBinding (art-tgt-2 -> same art-lifecycle-release)")
				rb2File := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "releasebinding.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "art-rb-2"},
						{"op": "replace", "path": "/spec/targetRef/name", "value": "art-tgt-2"},
						{"op": "replace", "path": "/spec/releaseRef/name", "value": "art-lifecycle-release"}
					]`,
				)
				defer func() { _ = os.Remove(rb2File) }()
				applyResource(testns, rb2File)

				By("waiting for the release RenderTask for art-tgt-2 to succeed")
				// The renderer detects the chart already exists in the registry (same coordinates
				// as art-tgt-1's render) and skips the actual push, marking JobSucceeded quickly.
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "-o",
						`jsonpath={range .items[?(@.spec.ownerName=="art-tgt-2")]}{.spec.repository}{"="}{.status.conditions[?(@.type=="JobSucceeded")].status}{"\n"}{end}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("release-art-lifecycle-release=True"),
						"release RenderTask for art-tgt-2 not yet succeeded; output: %s", output)
				}).Should(Succeed())

				By("verifying the artifact is shared: still exactly 1 RenderArtifact")
				Eventually(func(g Gomega) {
					arts := getRenderArtifactsByRepo(testns, releaseRepo())
					g.Expect(arts).To(HaveLen(1),
						"expected 1 shared RenderArtifact, but got: %v", arts)
					g.Expect(arts[0]).To(Equal(artName),
						"expected the same artifact %s to be reused, got %s", artName, arts[0])
				}).Should(Succeed())

				By("verifying 2 RenderBindings now reference the shared artifact (one per target)")
				Eventually(func(g Gomega) {
					bindings := getRenderBindingsByArtifact(testns, artName)
					g.Expect(bindings).To(HaveLen(2),
						"expected 2 RenderBindings for artifact %s (art-tgt-1 + art-tgt-2), got: %v",
						artName, bindings)
				}).Should(Succeed())
			})

			It("should retain the RenderArtifact when only one binding is removed", func() {
				By("deleting art-rb-1 — target controller should clean up art-tgt-1's stale RenderBinding")
				cmd := exec.Command(kubectlBinary, "delete", "releasebinding", "art-rb-1", "-n", testns)
				_, err := run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("waiting for art-tgt-1's RenderBinding to be removed (stale binding GC by target reconciler)")
				Eventually(func(g Gomega) {
					bindings := getRenderBindingsByArtifact(testns, artName)
					g.Expect(bindings).To(HaveLen(1),
						"expected 1 remaining RenderBinding after art-rb-1 deletion, got: %v", bindings)
				}).Should(Succeed())

				By("verifying the RenderArtifact persists — art-tgt-2's binding keeps it alive")
				Consistently(func(g Gomega) {
					arts := getRenderArtifactsByRepo(testns, releaseRepo())
					g.Expect(arts).To(ContainElement(artName),
						"RenderArtifact %s should still exist while art-tgt-2 holds a binding", artName)
				}, "15s", "3s").Should(Succeed())
			})

			It("should GC the RenderArtifact and remove the OCI tag when the last binding is gone", func() {
				By("capturing the artifact's OCI tag before cleanup")
				cmd := exec.Command(kubectlBinary, "get", "renderartifact", artName,
					"-n", testns, "-o", "jsonpath={.spec.tag}")
				output, err := run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).NotTo(BeEmpty(), "artifact spec.tag must not be empty")
				artTag = output

				By("deleting art-tgt-2 (exercises Target finalizer path: deleteOwnedRenderBindings)")
				cmd = exec.Command(kubectlBinary, "delete", "target", "art-tgt-2", "-n", testns)
				_, err = run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("waiting for all RenderBindings to be removed (finalizer cleans them up)")
				Eventually(func(g Gomega) {
					bindings := getRenderBindingsByArtifact(testns, artName)
					g.Expect(bindings).To(BeEmpty(),
						"all RenderBindings should be gone after art-tgt-2 deletion, remaining: %v", bindings)
				}).Should(Succeed())

				// The RenderArtifactReconciler sees 0 bindings -> calls Delete on the artifact
				// (sets DeletionTimestamp) -> finalizer path runs cleanupOCIArtifact -> removes
				// the OCI tag -> removes the finalizer -> k8s deletes the object.
				By("waiting for the RenderArtifact to be fully deleted from k8s (finalizer + OCI GC complete)")
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "wait", "renderartifact", artName,
						"-n", testns, "--for=delete", "--timeout=0")
					_, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred(),
						"RenderArtifact %s should be deleted; if stuck check OCICleanup=False condition "+
							"which indicates a registry auth/TLS issue in the controller-manager", artName)
				}).Should(Succeed())

				By("verifying the OCI tag is gone from the registry (belt-and-suspenders check)")
				ociPort := getFreePort()
				stopOCI := portForward("service/zot-deploy", ociPort, 443, "-n", "zot")
				defer stopOCI()
				zotClient := newZotClient(ociPort)
				ociCtx := context.Background()
				Eventually(func(g Gomega) {
					repo, err := zotClient.Repository(ociCtx, releaseRepo())
					g.Expect(err).NotTo(HaveOccurred(), "should connect to zot-deploy")
					_, _, err = repo.FetchReference(ociCtx, artTag)
					g.Expect(errors.Is(err, errdef.ErrNotFound)).To(BeTrue(),
						"OCI tag %s in %s should no longer exist after GC", artTag, releaseRepo())
				}).Should(Succeed())
			})

			It("should GC the RenderArtifact when the last ReleaseBinding is deleted directly (stale-binding path)", func() {
				// This exercises the alternative GC entry point: deleteStaleRenderBindings, called
				// during the normal reconcile loop when a Target's ReleaseBindings drop to zero.
				// This is distinct from the Target-finalizer path (deleteOwnedRenderBindings) tested
				// in the previous spec.
				var artName3 string
				var artTag3 string

				DeferCleanup(func() {
					for _, res := range [][]string{
						{"releasebinding", "art-rb-3"},
						{"target", "art-tgt-3"},
					} {
						cmd := exec.Command(kubectlBinary, "delete", res[0], res[1],
							"-n", testns, "--ignore-not-found", "--timeout=2m")
						_, _ = run(cmd)
					}
				})

				By("creating art-tgt-3 Target using art-deploy-registry")
				tgt3File := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "target.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "art-tgt-3"},
						{"op": "replace", "path": "/spec/renderRegistryRef/name", "value": "art-deploy-registry"}
					]`,
				)
				defer func() { _ = os.Remove(tgt3File) }()
				applyResource(testns, tgt3File)

				By("creating art-rb-3 ReleaseBinding (art-tgt-3 -> art-lifecycle-release)")
				rb3File := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "releasebinding.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "art-rb-3"},
						{"op": "replace", "path": "/spec/targetRef/name", "value": "art-tgt-3"},
						{"op": "replace", "path": "/spec/releaseRef/name", "value": "art-lifecycle-release"}
					]`,
				)
				defer func() { _ = os.Remove(rb3File) }()
				applyResource(testns, rb3File)

				By("waiting for art-tgt-3's release RenderTask to succeed (renderer pushes or detects existing chart)")
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "-o",
						`jsonpath={range .items[?(@.spec.ownerName=="art-tgt-3")]}{.spec.repository}{"="}{.status.conditions[?(@.type=="JobSucceeded")].status}{"\n"}{end}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("release-art-lifecycle-release=True"),
						"release RenderTask for art-tgt-3 not yet succeeded; output: %s", output)
				}).Should(Succeed())

				By("capturing the RenderArtifact name created for art-tgt-3")
				Eventually(func(g Gomega) {
					arts := getRenderArtifactsByRepo(testns, releaseRepo())
					g.Expect(arts).To(HaveLen(1),
						"expected 1 RenderArtifact for %s, got: %v", releaseRepo(), arts)
					artName3 = arts[0]
				}).Should(Succeed())

				By("capturing the OCI tag for the post-GC registry assertion")
				cmd := exec.Command(kubectlBinary, "get", "renderartifact", artName3,
					"-n", testns, "-o", "jsonpath={.spec.tag}")
				output, err := run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).NotTo(BeEmpty(), "artifact spec.tag must not be empty")
				artTag3 = output

				By("verifying exactly 1 RenderBinding exists for art-tgt-3")
				Eventually(func(g Gomega) {
					bindings := getRenderBindingsByArtifact(testns, artName3)
					g.Expect(bindings).To(HaveLen(1),
						"expected 1 RenderBinding for artifact %s, got: %v", artName3, bindings)
				}).Should(Succeed())

				By("deleting art-rb-3 directly (exercises deleteStaleRenderBindings in target reconciler)")
				cmd = exec.Command(kubectlBinary, "delete", "releasebinding", "art-rb-3", "-n", testns)
				_, err = run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("waiting for the target reconciler to remove the now-stale RenderBinding for art-tgt-3")
				Eventually(func(g Gomega) {
					bindings := getRenderBindingsByArtifact(testns, artName3)
					g.Expect(bindings).To(BeEmpty(),
						"stale RenderBinding should be removed when all ReleaseBindings are deleted, remaining: %v", bindings)
				}).Should(Succeed())

				By("waiting for the RenderArtifact to be fully GC'd (0 bindings -> OCI cleanup -> k8s deletion)")
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "wait", "renderartifact", artName3,
						"-n", testns, "--for=delete", "--timeout=0")
					_, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred(),
						"RenderArtifact %s should be deleted after last stale binding removed", artName3)
				}).Should(Succeed())

				By("verifying the OCI tag is gone from the registry")
				ociPort := getFreePort()
				stopOCI := portForward("service/zot-deploy", ociPort, 443, "-n", "zot")
				defer stopOCI()
				zotClient := newZotClient(ociPort)
				ociCtx := context.Background()
				Eventually(func(g Gomega) {
					repo, err := zotClient.Repository(ociCtx, releaseRepo())
					g.Expect(err).NotTo(HaveOccurred(), "should connect to zot-deploy")
					_, _, err = repo.FetchReference(ociCtx, artTag3)
					g.Expect(errors.Is(err, errdef.ErrNotFound)).To(BeTrue(),
						"OCI tag %s in %s should no longer exist after GC via stale-binding path", artTag3, releaseRepo())
				}).Should(Succeed())
			})

			It("(edge case) should surface MissingDependencies when a Release references a non-existent ComponentVersion", func() {
				DeferCleanup(func() {
					for _, res := range [][]string{
						{"releasebinding", "art-edge-rb"},
						{"target", "art-edge-tgt"},
						{"release", "art-edge-release"},
					} {
						cmd := exec.Command(kubectlBinary, "delete", res[0], res[1],
							"-n", testns, "--ignore-not-found", "--timeout=2m")
						_, _ = run(cmd)
					}
				})

				By("creating art-edge-release pointing to a ComponentVersion that does not exist")
				edgeRelFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "release.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "art-edge-release"},
						{"op": "replace", "path": "/spec/componentVersionRef/name", "value": "cv-does-not-exist"},
						{"op": "replace", "path": "/spec/uniqueName", "value": "art-edge"}
					]`,
				)
				defer func() { _ = os.Remove(edgeRelFile) }()
				applyResource(testns, edgeRelFile)

				// Use env=edge label so the existing 'production' Profile (env=prod) does not
				// create an additional ReleaseBinding that would obscure the condition check.
				By("creating art-edge-tgt Target (env=edge, not matched by the production Profile)")
				edgeTgtFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "target.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "art-edge-tgt"},
						{"op": "replace", "path": "/spec/renderRegistryRef/name", "value": "art-deploy-registry"},
						{"op": "replace", "path": "/metadata/labels/env", "value": "edge"}
					]`,
				)
				defer func() { _ = os.Remove(edgeTgtFile) }()
				applyResource(testns, edgeTgtFile)

				By("creating art-edge-rb ReleaseBinding (art-edge-tgt -> art-edge-release)")
				edgeRbFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "releasebinding.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "art-edge-rb"},
						{"op": "replace", "path": "/spec/targetRef/name", "value": "art-edge-tgt"},
						{"op": "replace", "path": "/spec/releaseRef/name", "value": "art-edge-release"}
					]`,
				)
				defer func() { _ = os.Remove(edgeRbFile) }()
				applyResource(testns, edgeRbFile)

				By("verifying Target art-edge-tgt gets ReleasesRendered=False reason=MissingDependencies")
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "target", "art-edge-tgt",
						"-n", testns,
						"-o", `jsonpath={.status.conditions[?(@.type=="ReleasesRendered")].reason}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("MissingDependencies"),
						"expected ReleasesRendered reason=MissingDependencies, got: %q", output)
				}).Should(Succeed())

				By("verifying no RenderTask for the art-edge-release chart was created")
				Consistently(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "-o",
						`jsonpath={range .items[?(@.spec.ownerName=="art-edge-tgt")]}{.spec.repository}{"\n"}{end}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).NotTo(ContainSubstring("art-edge-release"),
						"expected no RenderTask for art-edge-release to be created, but found: %s", output)
				}, "30s", "5s").Should(Succeed())
			})
		})

		Context("registry-binding relaxed mode (default)", Ordered, func() {
			AfterAll(func() {
				for _, res := range [][]string{
					{"releasebinding", "relax-rb"},
					{"release", "relax-release"},
					{"target", "relax-tgt"},
					{"componentversion", "strict-source-cv"},
				} {
					cmd := exec.Command(kubectlBinary, "delete", res[0], res[1],
						"-n", testns, "--ignore-not-found", "--timeout=2m")
					_, _ = run(cmd)
				}
			})

			It("should create a RenderTask even when no RegistryBinding exists for a source host", func() {
				By("creating a ComponentVersion whose resources reference an unbound source registry")
				applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "strict-mode-cv.yaml"))

				By("creating a Release referencing strict-source-cv")
				relaxRelFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "release.yaml"),
					fmt.Sprintf(`[
						{"op": "replace", "path": "/metadata/name", "value": "relax-release"},
						{"op": "replace", "path": "/spec/componentVersionRef/name", "value": "strict-source-cv"},
						{"op": "replace", "path": "/spec/uniqueName", "value": "relax-unique"},
						{"op": "replace", "path": "/spec/targetNamespace", "value": %q}
					]`, testns),
				)
				defer os.Remove(relaxRelFile)
				applyResource(testns, relaxRelFile)

				By("creating a Target with env=relax (not matched by any Profile)")
				relaxTgtFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "target.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "relax-tgt"},
						{"op": "replace", "path": "/metadata/labels/env", "value": "relax"}
					]`,
				)
				defer os.Remove(relaxTgtFile)
				applyResource(testns, relaxTgtFile)

				By("creating a ReleaseBinding from relax-tgt to relax-release (no RegistryBinding added)")
				relaxRbFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "releasebinding.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "relax-rb"},
						{"op": "replace", "path": "/spec/targetRef/name", "value": "relax-tgt"},
						{"op": "replace", "path": "/spec/releaseRef/name", "value": "relax-release"}
					]`,
				)
				defer os.Remove(relaxRbFile)
				applyResource(testns, relaxRbFile)

				By("verifying a RenderTask is created without any RegistryBinding in relaxed mode")
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "-o",
						`jsonpath={range .items[?(@.spec.ownerName=="relax-tgt")]}{.spec.repository}{"\n"}{end}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("release-relax-release"),
						"expected a RenderTask for relax-tgt in relaxed mode, got: %s", output)
				}).Should(Succeed())

				By("verifying rendering succeeds and MissingRegistryBinding is never set in relaxed mode")
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "target", "relax-tgt",
						"-n", testns,
						"-o", `jsonpath={.status.conditions[?(@.type=="ReleasesRendered")].reason}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("AllRendered"),
						"expected ReleasesRendered reason=AllRendered in relaxed mode, got: %q", output)
				}).Should(Succeed())
				Consistently(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "target", "relax-tgt",
						"-n", testns,
						"-o", `jsonpath={.status.conditions[?(@.type=="ReleasesRendered")].reason}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("AllRendered"),
						"ReleasesRendered must stay AllRendered in relaxed mode, got: %q", output)
				}, "20s", "3s").Should(Succeed())
			})
		})

		Context("with --registry-binding-strict enabled", Ordered, func() {
			BeforeAll(func() {
				By("redeploying controller-manager with --registry-binding-strict")
				redeploySolar("--set", "controller.args.registryBindingStrict=true")
			})

			AfterAll(func() {
				for _, res := range [][]string{
					{"releasebinding", "strict-rb"},
					{"release", "strict-release"},
					{"target", "strict-tgt"},
					{"componentversion", "strict-source-cv"},
					{"registrybinding", "strict-rb-source"},
					{"registry", "strict-source-registry"},
				} {
					cmd := exec.Command(kubectlBinary, "delete", res[0], res[1],
						"-n", testns, "--ignore-not-found", "--timeout=2m")
					_, _ = run(cmd)
				}

				By("restoring controller-manager to relaxed registry binding mode")
				redeploySolar()
			})

			It("should block rendering and set MissingRegistryBinding when no binding exists for a source host", func() {
				By("creating a ComponentVersion whose resources reference an unbound source registry")
				applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "strict-mode-cv.yaml"))

				By("creating a Release referencing strict-source-cv")
				strictRelFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "release.yaml"),
					fmt.Sprintf(`[
						{"op": "replace", "path": "/metadata/name", "value": "strict-release"},
						{"op": "replace", "path": "/spec/componentVersionRef/name", "value": "strict-source-cv"},
						{"op": "replace", "path": "/spec/uniqueName", "value": "strict-unique"},
						{"op": "replace", "path": "/spec/targetNamespace", "value": %q}
					]`, testns),
				)
				defer os.Remove(strictRelFile)
				applyResource(testns, strictRelFile)

				By("creating a Target with env=strict (not matched by any Profile)")
				strictTgtFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "target.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "strict-tgt"},
						{"op": "replace", "path": "/metadata/labels/env", "value": "strict"}
					]`,
				)
				defer os.Remove(strictTgtFile)
				applyResource(testns, strictTgtFile)

				By("creating a ReleaseBinding from strict-tgt to strict-release")
				strictRbFile := patchYAMLFile(
					filepath.Join(dir, "test", "fixtures", "e2e", "releasebinding.yaml"),
					`[
						{"op": "replace", "path": "/metadata/name", "value": "strict-rb"},
						{"op": "replace", "path": "/spec/targetRef/name", "value": "strict-tgt"},
						{"op": "replace", "path": "/spec/releaseRef/name", "value": "strict-release"}
					]`,
				)
				defer os.Remove(strictRbFile)
				applyResource(testns, strictRbFile)

				By("verifying Target strict-tgt gets ReleasesRendered=False reason=MissingRegistryBinding")
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "target", "strict-tgt",
						"-n", testns,
						"-o", `jsonpath={.status.conditions[?(@.type=="ReleasesRendered")].reason}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("MissingRegistryBinding"),
						"expected ReleasesRendered reason=MissingRegistryBinding, got: %q", output)
				}).Should(Succeed())

				By("verifying no RenderTask for strict-tgt was created while binding is absent")
				Consistently(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "-o",
						`jsonpath={range .items[?(@.spec.ownerName=="strict-tgt")]}{.spec.repository}{"\n"}{end}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(BeEmpty(),
						"expected no RenderTask for strict-tgt in strict mode, got: %s", output)
				}, "30s", "5s").Should(Succeed())

				By("adding the missing RegistryBinding for strict-source.example.com")
				applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "strict-mode-source-registry.yaml"))
				applyResource(testns, filepath.Join(dir, "test", "fixtures", "e2e", "strict-mode-registrybinding.yaml"))

				By("verifying the MissingRegistryBinding condition clears after RegistryBinding is added")
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "target", "strict-tgt",
						"-n", testns,
						"-o", `jsonpath={.status.conditions[?(@.type=="ReleasesRendered")].reason}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).NotTo(Equal("MissingRegistryBinding"),
						"MissingRegistryBinding condition should be cleared once RegistryBinding is present")
				}).Should(Succeed())

				By("verifying a RenderTask for strict-release is created once the binding is in place")
				Eventually(func(g Gomega) {
					cmd := exec.Command(kubectlBinary, "get", "rendertasks", "-n", testns, "-o",
						`jsonpath={range .items[?(@.spec.ownerName=="strict-tgt")]}{.spec.repository}{"\n"}{end}`)
					output, err := run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("release-strict-release"),
						"expected a RenderTask referencing release-strict-release, got: %s", output)
				}).Should(Succeed())
			})
		})
	})
})
