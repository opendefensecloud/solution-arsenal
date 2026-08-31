// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	ggcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	testregistry "go.opendefense.cloud/solar/test/registry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSolarRenderer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Solar Renderer Suite")
}

var _ = Describe("solar-renderer command", func() {
	var (
		tmpConfigFile   *os.File
		tmpDockerConfig *os.File
		testRegistry    *testregistry.Registry
		testServer      *http.Server
		registryURL     string

		username = "myusername"
		password = "mypassword"
	)

	cmdOutput := func(cmd *cobra.Command) *bytes.Buffer {
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)

		return &output
	}

	validReleaseConfig := func() solarv1alpha1.RendererConfig {
		return solarv1alpha1.RendererConfig{
			Type: solarv1alpha1.RendererConfigTypeRelease,
			ReleaseConfig: solarv1alpha1.ReleaseConfig{
				Chart: solarv1alpha1.ChartConfig{
					Name:        "test-chart",
					Description: "Test Chart",
					Version:     "1.0.0",
					AppVersion:  "1.0.0",
				},
				Input: solarv1alpha1.ReleaseInput{
					Component: solarv1alpha1.ReleaseComponent{
						Name: "test-component",
					},
					Resources: map[string]solarv1alpha1.ResolvedResourceAccess{
						"resource1": {
							Repository: "oci://example.com/resource1",
							Tag:        "v1.0.0",
						},
						"resource2": {
							Repository: "oci://example.com/resource2",
							Tag:        "v1.0.0",
						},
					},
					Entrypoint: solarv1alpha1.Entrypoint{
						ResourceName: "resource1",
						Type:         solarv1alpha1.EntrypointTypeHelm,
					},
				},
				Values: runtime.RawExtension{},
			},
		}
	}

	writeToTmpConfig := func(config solarv1alpha1.RendererConfig) {
		configData, err := yaml.Marshal(config)
		Expect(err).NotTo(HaveOccurred())

		_, err = tmpConfigFile.Write(configData)
		Expect(err).NotTo(HaveOccurred())
		_ = tmpConfigFile.Close()
	}

	writeTmpDockerConfig := func() {
		var err error
		tmpDockerConfig, err = os.CreateTemp("", "dockerconfig-*.json")
		Expect(err).NotTo(HaveOccurred())

		auth := base64.StdEncoding.EncodeToString(fmt.Appendf([]byte{}, "%s:%s", username, password))
		url := strings.TrimPrefix(registryURL, "oci://")

		config := map[string]any{
			"auths": map[string]any{
				url: map[string]string{
					"auth": auth,
				},
			},
		}
		dockerconfig, err := json.Marshal(config)
		Expect(err).NotTo(HaveOccurred())
		_, err = tmpDockerConfig.Write(dockerconfig)
		Expect(err).NotTo(HaveOccurred())
		_ = tmpDockerConfig.Close()
	}

	BeforeEach(func() {
		var err error
		tmpConfigFile, err = os.CreateTemp("", "renderer-config-*.yaml")
		Expect(err).NotTo(HaveOccurred())

		// Start test registry
		testRegistry = testregistry.New(
			registry.Logger(log.New(GinkgoWriter, "registry", log.Flags())),
		).WithAuth(username, password)

		// Find an available port
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		registryAddr := listener.Addr().String()
		_ = listener.Close()

		// Create HTTP server for the registry
		testServer = &http.Server{
			Addr:              registryAddr,
			Handler:           testRegistry.HandleFunc(),
			ReadHeaderTimeout: 5 * time.Second,
			ErrorLog:          log.New(GinkgoWriter, "test-server", log.Flags()),
		}

		// Start server in background
		go func() {
			_ = testServer.ListenAndServe()
		}()

		// Give server time to start
		Eventually(func() error {
			conn, err := net.Dial("tcp", registryAddr)
			if err != nil {
				return err
			}
			_ = conn.Close()

			return nil
		}, "5s").Should(Succeed())

		registryURL = fmt.Sprintf("oci://%s", registryAddr)
	})

	AfterEach(func() {
		if tmpConfigFile != nil {
			if err := os.Remove(tmpConfigFile.Name()); err != nil {
				GinkgoLogr.Info("Failed to cleanup temporary config file", "path", tmpConfigFile.Name())
			}
			tmpConfigFile = nil
		}
		if tmpDockerConfig != nil {
			if err := os.Remove(tmpDockerConfig.Name()); err != nil {
				GinkgoLogr.Info("Failed to cleanup temporary dockerconfig file", "path", tmpDockerConfig.Name())
			}
			tmpDockerConfig = nil
		}
		_ = testServer.Shutdown(context.TODO())
	})

	Describe("render-only mode", func() {
		It("should render a release from config file", func() {
			writeToTmpConfig(validReleaseConfig())

			// Execute command
			cmd := newRootCmd()
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--skip-push"})
			output := cmdOutput(cmd)

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Verify output mentions rendering
			Expect(output.String()).To(ContainSubstring("Rendered release"))
		})

		It("should fail with invalid config file", func() {
			cmd := newRootCmd()
			cmd.SetArgs([]string{"/nonexistent/config.yaml", "--skip-push"})
			_ = cmdOutput(cmd)

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read config-file"))
		})

		It("should fail with malformed YAML", func() {
			config := "invalid: yaml: content: ["
			_, err := tmpConfigFile.WriteString(config)
			Expect(err).NotTo(HaveOccurred())
			_ = tmpConfigFile.Close()

			cmd := newRootCmd()
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--skip-push"})
			_ = cmdOutput(cmd)

			err = cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse config-file"))
		})

		It("should fail with invalid TMPDIR", func() {
			oldTmp := os.Getenv("TMPDIR")
			defer func() { _ = os.Setenv("TMPDIR", oldTmp) }()
			err := os.Setenv("TMPDIR", "/nonexistent")
			Expect(err).NotTo(HaveOccurred())

			writeToTmpConfig(validReleaseConfig())

			cmd := newRootCmd()
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--skip-push"})
			_ = cmdOutput(cmd)

			err = cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no such file or directory"))
		})

		It("should fail with unknown type", func() {
			writeToTmpConfig(solarv1alpha1.RendererConfig{
				Type: "unknown",
			})

			cmd := newRootCmd()
			cmd.SetArgs([]string{tmpConfigFile.Name(), "--skip-push"})
			_ = cmdOutput(cmd)

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown type specified"))
		})
	})

	Describe("render and push mode", func() {
		It("should render and push a release to OCI registry", func() {
			writeToTmpConfig(validReleaseConfig())

			// Execute command
			cmd := newRootCmd()
			cmd.SetArgs([]string{
				"--plain-http",
				"--url=" + registryURL + "/test-chart:1.0.0",
				"--username=" + username,
				"--password=" + password,
				tmpConfigFile.Name(),
			})
			output := cmdOutput(cmd)

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Verify output mentions both rendering and pushing
			Expect(output.String()).To(ContainSubstring("Rendered release"))
			Expect(output.String()).To(ContainSubstring("Pushed result to"))
		})

		It("should render and push a release to OCI registry with dockerconfig", func() {
			writeTmpDockerConfig()
			oldDockerConfig := os.Getenv("DOCKER_CONFIG")
			defer func() { _ = os.Setenv("DOCKER_CONFIG", oldDockerConfig) }()
			err := os.Setenv("DOCKER_CONFIG", tmpDockerConfig.Name())
			Expect(err).NotTo(HaveOccurred())

			writeToTmpConfig(validReleaseConfig())

			// Execute command
			cmd := newRootCmd()
			cmd.SetArgs([]string{
				"--plain-http",
				"--url=" + registryURL + "/test-chart:1.0.0",
				tmpConfigFile.Name(),
			})
			output := cmdOutput(cmd)

			err = cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Verify output mentions both rendering and pushing
			Expect(output.String()).To(ContainSubstring("Rendered release"))
			Expect(output.String()).To(ContainSubstring("Pushed result to"))
		})

		It("should fail push with invalid registry credentials", func() {
			writeToTmpConfig(validReleaseConfig())

			cmd := newRootCmd()
			cmd.SetArgs([]string{
				tmpConfigFile.Name(),
				"--url=" + registryURL + "/test-chart:1.0.0",
				"--plain-http",
				"--username=" + username,
				"--password=wrong-password",
			})
			_ = cmdOutput(cmd)

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to push result"))
		})
	})
	Describe("signing mode", func() {
		// signingConfig returns a release config wired to a freshly generated
		// cosign keypair, plus the public half for verification.
		signingConfig := func(pass string) (solarv1alpha1.RendererConfig, []byte) {
			GinkgoHelper()

			keys, err := cosign.GenerateKeyPair(func(bool) ([]byte, error) { return []byte(pass), nil })
			Expect(err).NotTo(HaveOccurred())

			keyPath := filepath.Join(GinkgoT().TempDir(), "cosign.key")
			Expect(os.WriteFile(keyPath, keys.PrivateBytes, 0o600)).To(Succeed())

			cfg := validReleaseConfig()
			cfg.Signing = &solarv1alpha1.SigningConfig{KeyPath: keyPath}

			return cfg, keys.PublicBytes
		}

		// rewriteConfig replaces the config file contents; writeToTmpConfig
		// closes the handle, so it can only be used once per spec.
		rewriteConfig := func(config solarv1alpha1.RendererConfig) {
			GinkgoHelper()

			configData, err := yaml.Marshal(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(tmpConfigFile.Name(), configData, 0o600)).To(Succeed())
		}

		runRenderer := func(chartRef string) (*bytes.Buffer, error) {
			GinkgoHelper()

			cmd := newRootCmd()
			cmd.SetArgs([]string{
				"--plain-http",
				"--url=" + chartRef,
				"--username=" + username,
				"--password=" + password,
				tmpConfigFile.Name(),
			})
			output := cmdOutput(cmd)

			return output, cmd.Execute()
		}

		// signatureCount reports how many cosign signatures are attached to ref.
		signatureCount := func(chartRef string) int {
			GinkgoHelper()

			parsed, err := name.ParseReference(strings.TrimPrefix(chartRef, "oci://"), name.Insecure)
			Expect(err).NotTo(HaveOccurred())

			remoteOpt := ggcrremote.WithAuth(&authn.Basic{Username: username, Password: password})

			desc, err := ggcrremote.Get(parsed, remoteOpt)
			Expect(err).NotTo(HaveOccurred())

			digest := parsed.Context().Digest(desc.Digest.String())
			opts := ociremote.WithRemoteOptions(remoteOpt)

			sigTag, err := ociremote.SignatureTag(digest, opts)
			Expect(err).NotTo(HaveOccurred())

			sigs, err := ociremote.Signatures(sigTag, opts)
			Expect(err).NotTo(HaveOccurred())

			list, err := sigs.Get()
			Expect(err).NotTo(HaveOccurred())

			return len(list)
		}

		It("signs the artifact after pushing it", func() {
			cfg, pubKey := signingConfig("passw0rd")
			writeToTmpConfig(cfg)
			GinkgoT().Setenv("COSIGN_PASSWORD", "passw0rd")

			chartRef := registryURL + "/test-chart:1.0.0"
			output, err := runRenderer(chartRef)
			Expect(err).NotTo(HaveOccurred())

			Expect(output.String()).To(ContainSubstring("Pushed result to"))
			Expect(output.String()).To(ContainSubstring("Signed"))
			Expect(signatureCount(chartRef)).To(Equal(1))
			Expect(pubKey).NotTo(BeEmpty())
		})

		It("fails when the signing key is missing", func() {
			cfg, _ := signingConfig("passw0rd")
			cfg.Signing.KeyPath = filepath.Join(GinkgoT().TempDir(), "absent.key")
			writeToTmpConfig(cfg)
			GinkgoT().Setenv("COSIGN_PASSWORD", "passw0rd")

			_, err := runRenderer(registryURL + "/test-chart:1.0.0")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to sign artifact"))
		})

		It("fails when the key password is wrong", func() {
			cfg, _ := signingConfig("passw0rd")
			writeToTmpConfig(cfg)
			GinkgoT().Setenv("COSIGN_PASSWORD", "not-the-password")

			_, err := runRenderer(registryURL + "/test-chart:1.0.0")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to sign artifact"))
		})

		It("skips an artifact already signed with the same key", func() {
			cfg, _ := signingConfig("passw0rd")
			writeToTmpConfig(cfg)
			GinkgoT().Setenv("COSIGN_PASSWORD", "passw0rd")

			chartRef := registryURL + "/test-chart:1.0.0"

			_, err := runRenderer(chartRef)
			Expect(err).NotTo(HaveOccurred())

			output, err := runRenderer(chartRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(output.String()).To(ContainSubstring("already exists"))
			Expect(output.String()).NotTo(ContainSubstring("Signed"))
			Expect(signatureCount(chartRef)).To(Equal(1))
		})

		It("re-renders an existing artifact that carries no signature from this key", func() {
			cfg, _ := signingConfig("passw0rd")
			writeToTmpConfig(cfg)
			GinkgoT().Setenv("COSIGN_PASSWORD", "passw0rd")

			chartRef := registryURL + "/test-chart:1.0.0"

			_, err := runRenderer(chartRef)
			Expect(err).NotTo(HaveOccurred())

			// A second target signs with its own key, so the existing-chart
			// early return must not apply, otherwise that target never gets a
			// signature it can verify.
			second, _ := signingConfig("passw0rd")
			rewriteConfig(second)

			output, err := runRenderer(chartRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(output.String()).NotTo(ContainSubstring("already exists"))
			Expect(output.String()).To(ContainSubstring("Signed"))
		})

		It("skips signing and keeps the existing-chart short-circuit when unconfigured", func() {
			writeToTmpConfig(validReleaseConfig())

			chartRef := registryURL + "/test-chart:1.0.0"

			output, err := runRenderer(chartRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(output.String()).To(ContainSubstring("Pushed result to"))
			Expect(output.String()).NotTo(ContainSubstring("Signed"))
			Expect(signatureCount(chartRef)).To(Equal(0))

			output, err = runRenderer(chartRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(output.String()).To(ContainSubstring("already exists"))
		})
	})
})
