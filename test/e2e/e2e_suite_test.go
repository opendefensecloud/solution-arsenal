// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"sigs.k8s.io/yaml"

	jsonpatch "github.com/evanphx/json-patch/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
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
	kubectlBinary = func() string {
		if v, ok := os.LookupEnv("KUBECTL"); ok {
			return v
		} else {
			return "kubectl"
		}
	}()
	makeBinary = func() string {
		if v, ok := os.LookupEnv("MAKE"); ok {
			return v
		} else {
			return "make"
		}
	}()
	ocmBinary = func() string {
		if v, ok := os.LookupEnv("OCM"); ok {
			return v
		} else {
			return "ocm"
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
	// Setup e2e Cluster
	cmd := exec.Command(makeBinary, "e2e-cluster")
	_, err := run(cmd)
	Expect(err).NotTo(HaveOccurred())

	// Let's retrieve the kubeconfig of the kind cluster
	By("fetching the kubeconfig from kind")
	f, err := os.CreateTemp("", "e2e-kubeconfig")
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	cmd = exec.Command(kindBinary, "get", "kubeconfig", fmt.Sprintf("--name=%s", kindCluster))
	kc, err := run(cmd)
	Expect(err).NotTo(HaveOccurred())
	_, err = f.WriteString(kc)
	Expect(err).NotTo(HaveOccurred())
	f.Sync()
	kubeConfigPath = f.Name()
})

var _ = AfterSuite(func() {
	if kubeConfigPath != "" {
		os.Remove(kubeConfigPath)
	}
})

// ------------------------------- HELPER -------------------------------------

func setCmdContext(cmd *exec.Cmd) error {
	dir, err := getProjectDir()
	if err != nil {
		return err
	}
	cmd.Dir = dir
	env := append(os.Environ(), "GO111MODULE=on", fmt.Sprintf("KUBECONFIG=%s", kubeConfigPath))
	cmd.Env = append(cmd.Env, env...)

	return nil
}

// run executes the provided command within this context
func run(cmd *exec.Cmd) (string, error) {
	if err := setCmdContext(cmd); err != nil {
		return "", err
	}

	command := strings.Join(cmd.Args, " ")
	logf("running: %q\n", command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("%s failed with error: %q", command, err)
	}

	return string(output), err
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

// applyResource applies a resource in the namespace
func applyResource(namespace, file string) {
	GinkgoHelper()

	cmd := exec.Command(kubectlBinary, "apply", "-n", namespace, "-f", file)
	_, err := run(cmd)
	Expect(err).NotTo(HaveOccurred())
}

// portForward temporarily forwards a local port into the cluster
func portForward(typename string, localport int, remoteport int, args ...string) func() {
	GinkgoHelper()

	finalargs := append([]string{"port-forward", typename}, args...)
	finalargs = append(finalargs, fmt.Sprintf("%d:%d", localport, remoteport))
	cmd := exec.Command(kubectlBinary, finalargs...)
	setCmdContext(cmd)
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter

	err := cmd.Start()
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() error {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", localport), time.Second)
		if err != nil {
			return err
		}
		conn.Close()

		return nil
	}).Should(Succeed())

	return func() {
		if cmd.Process != nil {
			err := cmd.Process.Kill()
			Expect(err).NotTo(HaveOccurred())
			err = cmd.Wait()
			Expect(err).To(MatchError(ContainSubstring("signal: killed")))
		}
	}
}

// setupTestNS creates a temporary namespace in the cluster
func setupTestNS() string {
	GinkgoHelper()

	testns := "test"
	cmd := exec.Command(kubectlBinary, "create", "namespace", testns)
	_, err := run(cmd)
	Expect(err).NotTo(HaveOccurred())

	cmd = exec.Command(kubectlBinary, "label", "namespace", testns, "trust=enabled", "--overwrite")
	_, err = run(cmd)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() error {
		cmd := exec.Command(kubectlBinary, "get", "configmap", "-n", testns, "root-bundle")
		_, err := run(cmd)

		return err
	}).Should(Succeed())

	return testns
}

// getFreePort asks the kernel for a free open port that is ready to use.
func getFreePort() int {
	GinkgoHelper()

	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	Expect(err).NotTo(HaveOccurred())

	l, err := net.ListenTCP("tcp", addr)
	Expect(err).NotTo(HaveOccurred())
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port
}

// newZotClient creates an oras-go remote.Registry pointing at the local
// port-forwarded Zot instance, configured with the cluster's self-signed CA
// and admin credentials.
func newZotClient(localport int) *remote.Registry {
	GinkgoHelper()

	cmd := exec.Command(kubectlBinary, "get", "secret", "zot-tls", "-n", "zot", "-o", "jsonpath={.data.ca\\.crt}")
	output, err := run(cmd)
	Expect(err).NotTo(HaveOccurred())

	caCert, err := base64.StdEncoding.DecodeString(output)
	Expect(err).NotTo(HaveOccurred())

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	zotDeploy, err := remote.NewRegistry(fmt.Sprintf("localhost:%d", localport))
	Expect(err).NotTo(HaveOccurred())
	zotDeploy.Client = &auth.Client{
		Credential: auth.StaticCredential(fmt.Sprintf("localhost:%d", localport), auth.Credential{
			Username: "admin",
			Password: "admin",
		}),
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: caCertPool,
				},
			},
		},
	}

	return zotDeploy
}

// getStatusCondition gets a status condition of an object
func getStatusCondition(namespace string, typename string, condition string) bool {
	GinkgoHelper()

	cmd := exec.Command(kubectlBinary, "get", "-n", namespace, typename,
		"-o", fmt.Sprintf("jsonpath={.status.conditions[?(@.type=='%s')].status}", condition))
	output, err := run(cmd)
	if err != nil {
		return false
	}

	return strings.TrimSpace(output) == "True"
}

// patchYAMLFile returns the path to a temporary file containing the yaml file with the applied patch
func patchYAMLFile(path string, patch string) string {
	GinkgoHelper()

	b, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	json, err := yaml.YAMLToJSON(b)
	Expect(err).NotTo(HaveOccurred())

	p, err := jsonpatch.DecodePatch([]byte(patch))
	Expect(err).NotTo(HaveOccurred())

	patchedJSON, err := p.Apply(json)
	Expect(err).NotTo(HaveOccurred())

	patchedYAML, err := yaml.JSONToYAML(patchedJSON)
	Expect(err).NotTo(HaveOccurred())

	f, err := os.CreateTemp("", "patched-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	_, err = f.Write(patchedYAML)
	Expect(err).NotTo(HaveOccurred())

	return f.Name()
}
