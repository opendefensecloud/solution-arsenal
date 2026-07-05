package trust

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const cosignBinary = "cosign"

// Verifier verifies cosign signatures on OCI artifacts.
type Verifier struct {
	cosignPath string
}

// NewVerifier creates a new Verifier that uses the cosign binary.
// It looks for cosign in PATH or at the specified path.
func NewVerifier(cosignPath string) (*Verifier, error) {
	if cosignPath == "" {
		path, err := exec.LookPath(cosignBinary)
		if err != nil {
			return nil, fmt.Errorf("cosign not found in PATH: %w", err)
		}
		cosignPath = path
	}
	return &Verifier{cosignPath: cosignPath}, nil
}

// VerifyOpts holds options for signature verification.
type VerifyOpts struct {
	// ImageRef is the OCI image reference to verify (e.g. "registry.example.com/repo/component:v1").
	ImageRef string
	// PublicKeyPEM is the PEM-encoded cosign public key.
	// If empty, keyless verification (Sigstore/Fulcio) is attempted.
	PublicKeyPEM string
}

// Result holds the outcome of a signature verification.
type Result struct {
	// Verified is true if the signature was successfully verified.
	Verified bool
	// Error contains the verification error, if any.
	Error error
}

// Verify verifies the cosign signature on the given OCI artifact.
func (v *Verifier) Verify(ctx context.Context, opts VerifyOpts) Result {
	args := []string{"verify"}

	if opts.PublicKeyPEM != "" {
		args = append(args, "--key", "/dev/stdin")
	} else {
		args = append(args, "--insecure-ignore-tlog=true")
	}

	args = append(args, opts.ImageRef)

	cmd := exec.CommandContext(ctx, v.cosignPath, args...)
	if opts.PublicKeyPEM != "" {
		cmd.Stdin = strings.NewReader(opts.PublicKeyPEM)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{
			Verified: false,
			Error:    fmt.Errorf("cosign verify failed: %w\noutput: %s", err, string(output)),
		}
	}

	return Result{
		Verified: true,
	}
}

// VerifyKeyless verifies a cosign keyless (Sigstore/Fulcio) signature.
func (v *Verifier) VerifyKeyless(ctx context.Context, imageRef string) Result {
	return v.Verify(ctx, VerifyOpts{
		ImageRef: imageRef,
	})
}

// VerifyWithPublicKey verifies a cosign signature against a specific public key.
func (v *Verifier) VerifyWithPublicKey(ctx context.Context, imageRef, publicKeyPEM string) Result {
	return v.Verify(ctx, VerifyOpts{
		ImageRef:    imageRef,
		PublicKeyPEM: publicKeyPEM,
	})
}
