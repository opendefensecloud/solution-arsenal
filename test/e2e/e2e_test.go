// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// namespace where the project is deployed in
const namespace = "solar-system"

var _ = Describe("solar", Ordered, func() {
	var controllerPodName string
	dir, err := getProjectDir()
	Expect(err).NotTo(HaveOccurred())

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating solar-system namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		// NOTE: etcd runs as root uid, so unfortunately we can not enforce this yet
		// By("labeling the namespace to enforce the restricted security policy")
		// cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
		// 	"pod-security.kubernetes.io/enforce=restricted")
		// _, err = run(cmd)
		// Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("deploying renderer secret")
		dir, err := getProjectDir()
		Expect(err).NotTo(HaveOccurred())
		cmd = exec.Command("kubectl", "apply", "-f", filepath.Join(dir, "test", "fixtures", "e2e", "zot-deploy-auth.yaml"))
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred())

		By("deploying apiserver and controller-manager")
		cmd = exec.Command(helmBinary, "upgrade", "--install",
			"--namespace", namespace, "solar", filepath.Join(dir, "charts", "solar"),
			"--values", filepath.Join(dir, "test", "fixtures", "solar.values.yaml"),
			"--set", "apiserver.image.tag=e2e",
			"--set", "controller.image.tag=e2e",
			"--set", "renderer.image.tag=e2e",
			"--set", "discovery.image.tag=e2e")
		_, err = run(cmd)
		Expect(err).NotTo(HaveOccurred())
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("undeploying the apiserver and controller-manager")
		cmd := exec.Command(helmBinary, "uninstall", "-n", namespace, "solar")
		_, _ = run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := run(cmd)
			if err == nil {
				logf("Controller logs:\n %s", controllerLogs)
			} else {
				logf("Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := run(cmd)
			if err == nil {
				logf("Kubernetes events:\n%s", eventsOutput)
			} else {
				logf("Failed to get Kubernetes events: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(10 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)

	Context("Extension API server and Controller Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "app.kubernetes.io/component=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := getNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())

			cmd := exec.Command("kubectl", "wait", "apiservices/v1alpha1.solar.opendefense.cloud",
				"--for", "condition=Available",
				"--timeout", waitTimeout)
			_, err = run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create targets", func() {
			Expect(applyResource("default", filepath.Join(dir, "test", "fixtures", "e2e", "target.yaml"))).To(Succeed())

			// Verify Target creation
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "targets", "-n", "default", "cluster-1", "-o", "jsonpath=\"{.spec.releases}\"")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("test-release"))
			}).Should(Succeed())

			// Verify HydratedTarget creation
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "hydratedtargets", "-n", "default", "cluster-1")
				_, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}).Should(Succeed())

			// Verify HydratedTarget Chart was pushed to the Registry
			// TODO
		})

		It("should create profiles in hydrated target", func() {
			Expect(applyResource("default", filepath.Join(dir, "test", "fixtures", "e2e", "profile.yaml"))).To(Succeed())

			// Verify that the profile has been added to the hydrated target
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "-n", "default", "hydratedtarget", "cluster-1", "-o", "jsonpath='{.spec.profiles.*}'")
				output, err := run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("production"))
			}).Should(Succeed())
		})
	})
})

func applyResource(namespace, file string) error {
	cmd := exec.Command("kubectl", "apply", "-n", namespace, "-f", file)
	_, err := run(cmd)
	return err
}
