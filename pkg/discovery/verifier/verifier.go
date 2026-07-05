package verifier

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/trust"
)

const (
	cosignPublicKeyKey = "cosign.pub"
)

type Verifier struct {
	*discovery.Runner[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent]
	provider       *discovery.RegistryProvider
	secretClient   corev1client.CoreV1Interface
	namespace      string
	trustVerifier  *trust.Verifier
	cosignVerifier *trust.Verifier
	cosignErr      error
}

func NewVerifier(
	provider *discovery.RegistryProvider,
	secretClient corev1client.CoreV1Interface,
	namespace string,
	in <-chan discovery.ComponentVersionEvent,
	out chan<- discovery.ComponentVersionEvent,
	errChan chan<- discovery.ErrorEvent,
	opts ...discovery.RunnerOption[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent],
) *Verifier {
	p := &Verifier{
		provider:     provider,
		secretClient: secretClient,
		namespace:    namespace,
	}
	p.Runner = discovery.NewRunner(p, in, out, errChan)
	for _, opt := range opts {
		opt(p.Runner)
	}

	return p
}

func NewVerifierOptions(opts ...discovery.RunnerOption[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent]) []discovery.RunnerOption[discovery.ComponentVersionEvent, discovery.ComponentVersionEvent] {
	return opts
}

func (v *Verifier) Process(ctx context.Context, ev discovery.ComponentVersionEvent) ([]discovery.ComponentVersionEvent, error) {
	if ev.Source.Type == discovery.EventDeleted {
		return []discovery.ComponentVersionEvent{ev}, nil
	}

	registry := v.provider.Get(ev.Source.Registry)
	if registry == nil {
		return nil, fmt.Errorf("invalid registry: %s", ev.Source.Registry)
	}

	if registry.Spec.Verification == nil || !registry.Spec.Verification.Enabled {
		v.Logger().V(1).Info("verification not enabled for registry, passing through", "registry", ev.Source.Registry)
		return []discovery.ComponentVersionEvent{ev}, nil
	}

	if v.secretClient == nil {
		v.Logger().V(1).Info("no secret client available, cannot read verification keys; passing through", "registry", ev.Source.Registry)
		return []discovery.ComponentVersionEvent{ev}, nil
	}

	// Initialize cosign verifier once, cache it and gracefully handle if missing
	if v.cosignVerifier == nil && v.cosignErr == nil {
		verifier, err := trust.NewVerifier("")
		if err != nil {
			v.cosignErr = err
			v.Logger().V(1).Info("cosign binary not available; passing through without verification", "error", err)
			return []discovery.ComponentVersionEvent{ev}, nil
		}
		v.cosignVerifier = verifier
	}

	if v.cosignErr != nil {
		v.Logger().V(1).Info("cosign not available; passing through without verification")
		return []discovery.ComponentVersionEvent{ev}, nil
	}

	// Strip scheme from URL to get proper OCI reference
	registryURL := registry.GetURL()
	// Remove scheme if present (e.g., "https://registry.example.com" -> "registry.example.com")
	if idx := strings.Index(registryURL, "://"); idx != -1 {
		registryURL = registryURL[idx+3:]
	}
	imageRef := fmt.Sprintf("%s/%s/%s:%s", registryURL, ev.Namespace, ev.Component, ev.Source.Version)

	var publicKeyPEM string
	if registry.Spec.Verification.KeySecretRef != nil {
		secret, err := v.secretClient.Secrets(v.namespace).Get(ctx, registry.Spec.Verification.KeySecretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to read verification key secret %q: %w", registry.Spec.Verification.KeySecretRef.Name, err)
		}
		pubKey, ok := secret.Data[cosignPublicKeyKey]
		if !ok {
			return nil, fmt.Errorf("secret %q is missing key %q", registry.Spec.Verification.KeySecretRef.Name, cosignPublicKeyKey)
		}
		publicKeyPEM = string(pubKey)
	}

	var result trust.Result
	if publicKeyPEM != "" {
		result = v.cosignVerifier.VerifyWithPublicKey(ctx, imageRef, publicKeyPEM)
	} else {
		result = v.cosignVerifier.VerifyKeyless(ctx, imageRef)
	}

	if !result.Verified {
		v.Logger().Error(result.Error, "signature verification failed", "image", imageRef, "registry", ev.Source.Registry)
		return nil, fmt.Errorf("signature verification failed for %s: %w", imageRef, result.Error)
	}

	v.Logger().V(1).Info("signature verified", "image", imageRef, "registry", ev.Source.Registry)
	return []discovery.ComponentVersionEvent{ev}, nil
}
