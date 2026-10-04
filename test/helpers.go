// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/onsi/ginkgo/v2"
)

// getProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)

	basepath = strings.ReplaceAll(basepath, "test", "")

	return basepath, nil
}

func Logf(format string, a ...any) {
	_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, format, a...)
}

// run executes the provided command within this context
func Run(cmd *exec.Cmd) (string, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		Logf("chdir dir: %q\n", err)
	}

	// Preserve an environment the caller already set, rather than replacing it:
	// TransferDemo uses it to pass SSL_CERT_FILE.
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	Logf("running: %q\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%q failed with error %q: %w", command, string(output), err)
	}

	return string(output), nil
}

// EnvName gets the content of an upper snake case environment variable and falling back on the name itself
func EnvName(name string) string {
	content, ok := os.LookupEnv(toUpperSnakeCase(name))
	if !ok || content == "" {
		return name
	}

	return content
}

func toUpperSnakeCase(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}

	return strings.ToUpper(b.String())
}

// Demo component coordinates, shared by every suite that seeds a registry from
// the ocm-demo CTF fixture.
const (
	// DemoComponent is the OCM component name of the demo fixture.
	DemoComponent = "opendefense.cloud/ocm-demo"
	// DemoVersion is the version the fixture CTF is built with. It must match
	// the version in test/fixtures/ocm-demo/component-constructor.yaml.
	DemoVersion = "v26.4.2"
	// DemoCTF is the CTF directory built by `make ocm-build-demo`.
	DemoCTF = "test/fixtures/ocm-demo-ctf"
)

// TransferDemo transfers the ocm-demo CTF at ctfPath into target using the OCM
// v2 CLI.
//
// Resources are copied by value and uploaded as OCI artifacts, so an image lands
// in the target registry as a real tagged artifact a workload can pull. The
// default localBlob mode would instead leave it addressable only by digest inside
// the component's own repository: it records a referenceName but publishes
// nothing at it, and a pod using that reference fails with "not found".
//
// The absolute reference this records carries whichever host the transfer was
// pushed through; SolAr re-anchors it to the registry it reads the component
// from (see ocmv2.ResolveOCIReference).
//
// ocmConfig may be empty, in which case no --config flag is passed.
//
// caFile optionally points at a CA bundle to trust for the target registry.
// OCM v2 has no rootcerts config type, so a private certificate is trusted via
// SSL_CERT_FILE rather than through the OCM config.
func TransferDemo(ctx context.Context, ctfPath, target, ocmConfig string, caFile ...string) (string, error) {
	args := []string{}
	if ocmConfig != "" {
		args = append(args, "--config", ocmConfig)
	}
	args = append(args, "transfer", "cv", "--copy-resources", "--upload-as", "ociArtifact",
		fmt.Sprintf("ctf::%s//%s:%s", ctfPath, DemoComponent, DemoVersion), target)

	cmd := exec.CommandContext(ctx, EnvName("ocm"), args...)
	if len(caFile) > 0 && caFile[0] != "" {
		cmd.Env = append(os.Environ(), "SSL_CERT_FILE="+caFile[0])
	}

	return Run(cmd)
}

// WriteOCMConfig writes an OCM v2 config granting basic-auth credentials for
// hostPort, and returns its path. The file is created under a Ginkgo-managed temp
// directory.
//
// It exists because OCM v2 matches a consumer identity on hostname AND port,
// defaulting an omitted port to 80 or 443 rather than treating it as a wildcard
// (see runtime.IdentityMatchesURL). Test registries listen on an ephemeral port,
// so the config cannot be a static fixture — it has to name the port the
// registry actually got.
func WriteOCMConfig(dir, hostPort, username, password string) (string, error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		// No port to split: use the value as the hostname.
		host, port = hostPort, ""
	}

	cfg := fmt.Sprintf(`type: generic.config.ocm.software/v1
configurations:
  - type: credentials.config.ocm.software/v1
    consumers:
      - identity:
          type: OCIRegistry
          hostname: %s
          port: %q
        credentials:
          - type: Credentials/v1
            properties:
              username: %s
              password: %s
`, host, port, username, password)

	path := filepath.Join(dir, "ocmconfig.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		return "", fmt.Errorf("failed to write ocm config: %w", err)
	}

	return path, nil
}
