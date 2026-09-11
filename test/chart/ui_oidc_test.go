// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package chart_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// helmTemplate renders the solar chart's UI deployment with the given --set
// overrides and returns the rendered manifest, or the helm error output.
func helmTemplate(sets ...string) (string, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	chartDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "charts", "solar")

	args := []string{
		"template", "solar", chartDir,
		"--show-only", "templates/ui/deployment.yaml",
		"--set", "ui.enabled=true",
	}
	for _, s := range sets {
		args = append(args, "--set", s)
	}

	out, err := exec.CommandContext(context.Background(), "helm", args...).CombinedOutput()

	return string(out), err
}

var _ = Describe("UI deployment OIDC arguments", func() {
	BeforeEach(func() {
		if _, err := exec.LookPath("helm"); err != nil {
			Skip("helm not found in PATH")
		}
	})

	Context("without an OIDC issuer", func() {
		It("renders without requiring any OIDC value", func() {
			out, err := helmTemplate()
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("--auth-mode=token"))
			Expect(out).NotTo(ContainSubstring("--oidc-"))
		})

		It("omits the client secret env var even if a secret is configured", func() {
			out, err := helmTemplate("ui.oidc.existingSecret=solar-ui-oidc")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).NotTo(ContainSubstring("SOLAR_UI_OIDC_CLIENT_SECRET"))
		})
	})

	Context("with an OIDC issuer", func() {
		It("renders the OIDC arguments", func() {
			out, err := helmTemplate(
				"ui.oidc.issuer=https://dex.example.com",
				"ui.oidc.redirectURL=https://solar.example.com/api/auth/callback",
			)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("--oidc-issuer=https://dex.example.com"))
			Expect(out).To(ContainSubstring("--oidc-client-id=solar-ui"))
			Expect(out).To(ContainSubstring("--oidc-redirect-url=https://solar.example.com/api/auth/callback"))
		})

		It("requires a redirect URL", func() {
			out, err := helmTemplate("ui.oidc.issuer=https://dex.example.com")
			Expect(err).To(HaveOccurred())
			Expect(out).To(ContainSubstring("ui.oidc.redirectURL is required"))
		})

		It("wires the client secret from the existing secret", func() {
			out, err := helmTemplate(
				"ui.oidc.issuer=https://dex.example.com",
				"ui.oidc.redirectURL=https://solar.example.com/api/auth/callback",
				"ui.oidc.existingSecret=solar-ui-oidc",
			)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("--oidc-client-secret=$(SOLAR_UI_OIDC_CLIENT_SECRET)"))
			Expect(strings.Count(out, "SOLAR_UI_OIDC_CLIENT_SECRET")).To(Equal(2))
		})
	})
})
