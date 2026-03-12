// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sync/errgroup"
)

const (
	certmanagerVersion  = "v1.19.1"
	certmanagerChart    = "oci://quay.io/jetstack/charts/cert-manager"
	trustmanagerVersion = "v0.20.0"
	trustmanagerChart   = "oci://quay.io/jetstack/charts/trust-manager"
	zotVersion          = "v0.1.102"
	zotRepo             = "https://zotregistry.dev/helm-charts"
	zotChart            = "zot"

	waitTimeout = "5m"
)

var (
	kindBinary = func() string {
		if v, ok := os.LookupEnv("KIND"); ok {
			return v
		} else {
			return "kind"
		}
	}()
	kindCluster = func() string {
		if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
			return v
		} else {
			return "kind"
		}
	}()
	helmBinary = func() string {
		if v, ok := os.LookupEnv("HELM"); ok {
			return v
		} else {
			return "helm"
		}
	}()

	kubeConfigPath = ""
)

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purpose of being used in CI jobs.
// The default setup requires Kind, builds/loads the Manager Docker image locally, and installs
// CertManager.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	logf("Starting project-v4 integration test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	// Let's retrieve the kubeconfig of the kind cluster
	By("fetching the kubeconfig from kind")
	f, err := os.CreateTemp("", "e2e-kubeconfig")
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	cmd := exec.Command(kindBinary, "get", "kubeconfig", fmt.Sprintf("--name=%s", kindCluster))
	kc, err := run(cmd)
	Expect(err).NotTo(HaveOccurred())
	_, err = f.WriteString(kc)
	Expect(err).NotTo(HaveOccurred())
	f.Sync()
	kubeConfigPath = f.Name()

	// Build images
	cmd = exec.Command("make", "docker-build-local-images", "TAG=e2e")
	_, err = run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load images")

	// Load images
	cmd = exec.Command("make", "kind-load-local-images", "TAG=e2e", fmt.Sprintf("KIND_CLUSTER=%s", kindCluster))
	_, err = run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load images")

	logf("Installing CertManager...\n")
	Expect(installCertManager()).To(Succeed(), "Failed to install CertManager")

	logf("Installing TrustManager...\n")
	Expect(installTrustManager()).To(Succeed(), "Failed to install TrustManager")

	logf("Installing Zot...\n")
	Expect(installZot()).To(Succeed(), "Failed to install Zot")
})

var _ = AfterSuite(func() {
	if kubeConfigPath != "" {
		os.Remove(kubeConfigPath)
	}
})

// ------------------------------- HELPER -------------------------------------

// run executes the provided command within this context
func run(cmd *exec.Cmd) (string, error) {
	dir, _ := getProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		logf("chdir dir: %q\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on", fmt.Sprintf("KUBECONFIG=%s", kubeConfigPath))
	command := strings.Join(cmd.Args, " ")
	logf("running: %q\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%q failed with error %q: %w", command, string(output), err)
	}

	return string(output), nil
}

// installCertManager installs cert manager and sets up a CA.
func installCertManager() error {
	cmd := exec.Command(helmBinary, "upgrade", "--install", "cert-manager", certmanagerChart,
		"--version", certmanagerVersion,
		"--namespace", "cert-manager",
		"--create-namespace",
		"--wait",
		"--set", "crds.enabled=true")
	if _, err := run(cmd); err != nil {
		return err
	}
	cmd = exec.Command("kubectl", "wait", "deployments.apps/cert-manager",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", waitTimeout)
	if _, err := run(cmd); err != nil {
		return err
	}

	dir, err := getProjectDir()
	Expect(err).NotTo(HaveOccurred())
	cmd = exec.Command("kubectl", "apply", "-f", filepath.Join(dir, "test", "fixtures", "certmanager.yaml"))
	if _, err := run(cmd); err != nil {
		return err
	}

	cmd = exec.Command("kubectl", "wait", "certificates.cert-manager.io/selfsigned-ca",
		"--for", "condition=Ready",
		"--namespace", "cert-manager",
		"--timeout", waitTimeout)
	_, err = run(cmd)

	return err
}

// installTrustManager installs trust manager.
func installTrustManager() error {
	cmd := exec.Command(helmBinary, "upgrade", "--install", "trust-manager", trustmanagerChart,
		"--version", trustmanagerVersion,
		"--namespace", "cert-manager",
		"--create-namespace",
		"--wait",
		"--set", "crds.enabled=true")
	if _, err := run(cmd); err != nil {
		return err
	}
	cmd = exec.Command("kubectl", "wait", "deployments.apps/trust-manager",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", waitTimeout)
	if _, err := run(cmd); err != nil {
		return err
	}

	dir, err := getProjectDir()
	Expect(err).NotTo(HaveOccurred())
	cmd = exec.Command("kubectl", "apply", "-f", filepath.Join(dir, "test", "fixtures", "trustmanager.yaml"))
	if _, err := run(cmd); err != nil {
		return err
	}

	cmd = exec.Command("kubectl", "label", "namespace", "default", "trust=enabled", "--overwrite")
	_, err = run(cmd)
	return err
}

// installZot installs a zot registry
func installZot() error {
	dir, err := getProjectDir()
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command("kubectl", "create", "namespace", "zot")
	if _, err := run(cmd); err != nil {
		return err
	}

	cmd = exec.Command("kubectl", "apply", "-f", filepath.Join(dir, "test", "fixtures", "zot-cert.yaml"))
	if _, err := run(cmd); err != nil {
		return err
	}
	cmd = exec.Command("kubectl", "wait", "certificates.cert-manager.io/zot-tls",
		"--for", "condition=Ready",
		"--namespace", "zot",
		"--timeout", waitTimeout)
	if _, err := run(cmd); err != nil {
		return err
	}

	installChart := func(name string) func() error {
		return func() error {
			cmd := exec.Command(helmBinary, "upgrade", "--install", name, "--repo", zotRepo, zotChart,
				"--version", zotVersion,
				"--namespace", "zot",
				"--create-namespace",
				"--wait",
				"--values", filepath.Join(dir, "test", "fixtures", name+".values.yaml"))
			_, err := run(cmd)
			return err
		}
	}

	g := errgroup.Group{}
	g.Go(installChart("zot-deploy"))
	g.Go(installChart("zot-discovery"))
	return g.Wait()
}

// getNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func getNonEmptyLines(output string) []string {
	var res []string
	for element := range strings.SplitSeq(output, "\n") {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// getProjectDir will return the directory where the project is
func getProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, fmt.Errorf("failed to get current working directory: %w", err)
	}
	wd = strings.ReplaceAll(wd, "/test/e2e", "")
	return wd, nil
}

func logf(format string, a ...any) {
	_, _ = fmt.Fprintf(GinkgoWriter, format, a...)
}
