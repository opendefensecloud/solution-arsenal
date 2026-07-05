#!/usr/bin/env bash

set -euo pipefail

KUBECTL="${KUBECTL:-kubectl}"
COSIGN="${COSIGN:-cosign}"
NAMESPACE="${NAMESPACE:-solar-system}"
REGISTRY_NAME="${REGISTRY_NAME:-default}"
KEYLESS="${KEYLESS:-false}"
COSIGN_KEY_PASSWORD="${COSIGN_KEY_PASSWORD:-}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
fatal() { echo -e "${RED}[FATAL]${NC} $*"; exit 1; }

check_prerequisites() {
    if ! command -v "$KUBECTL" &>/dev/null; then
        fatal "kubectl not found. Set KUBECTL or install kubectl."
    fi
    if ! command -v "$COSIGN" &>/dev/null; then
        warn "cosign not found. Attempting to install..."
        if command -v brew &>/dev/null; then
            brew install cosign
        elif command -v curl &>/dev/null; then
            COSIGN_VERSION=$(curl -s https://api.github.com/repos/sigstore/cosign/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
            curl -sL "https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}/cosign-linux-amd64" -o /tmp/cosign
            chmod +x /tmp/cosign
            COSIGN=/tmp/cosign
            info "cosign installed to /tmp/cosign"
        else
            fatal "Cannot install cosign. Please install it manually: https://docs.sigstore.dev/cosign/installation/"
        fi
    fi
    if ! $KUBECTL cluster-info &>/dev/null; then
        fatal "Cannot connect to Kubernetes cluster."
    fi
}

setup_keyed_verification() {
    local key_secret_name="cosign-public-key"

    if $KUBECTL get secret "$key_secret_name" -n "$NAMESPACE" &>/dev/null; then
        info "Secret $key_secret_name already exists in namespace $NAMESPACE"
    else
        COSIGN_KEY_PATH=$(mktemp)
        COSIGN_PUB_PATH=$(mktemp)
        trap 'rm -f "$COSIGN_KEY_PATH" "$COSIGN_PUB_PATH"' EXIT

        info "Generating cosign key pair..."
        if [ -n "$COSIGN_KEY_PASSWORD" ]; then
            COSIGN_PASSWORD="$COSIGN_KEY_PASSWORD" $COSIGN generate-key-pair --output-key-prefix /tmp/cosign-key
        else
            COSIGN_PASSWORD="" $COSIGN generate-key-pair --output-key-prefix /tmp/cosign-key
        fi

        mv /tmp/cosign-key-cosign.key "$COSIGN_KEY_PATH" 2>/dev/null || true
        mv /tmp/cosign-key-cosign.pub "$COSIGN_PUB_PATH" 2>/dev/null || true

        if [ ! -f "$COSIGN_PUB_PATH" ]; then
            fatal "Failed to generate cosign key pair. Check cosign installation."
        fi

        info "Creating Kubernetes secret with public key..."
        $KUBECTL create secret generic "$key_secret_name" \
            --namespace "$NAMESPACE" \
            --from-file=cosign.pub="$COSIGN_PUB_PATH" \
            --dry-run=client -o yaml | $KUBECTL apply -f -

        info "Public key saved to secret: $key_secret_name"
        info "Private key saved to: $COSIGN_KEY_PATH"
        warn "Store the private key securely. It is needed to sign OCM packages."
        info "To retrieve the private key: cat $COSIGN_KEY_PATH"
    fi

    patch_registry_verification "$key_secret_name"
}

setup_keyless_verification() {
    info "Configuring keyless (Sigstore/Fulcio) verification..."

    patch_registry_verification ""
}

patch_registry_verification() {
    local key_secret_name="$1"
    local verification_patch

    if [ -n "$key_secret_name" ]; then
        verification_patch='{"spec":{"verification":{"enabled":true,"keySecretRef":{"name":"'"$key_secret_name"'"}}}}'
        info "Enabling key-based verification on registry $REGISTRY_NAME (key secret: $key_secret_name)"
    else
        verification_patch='{"spec":{"verification":{"enabled":true}}}'
        info "Enabling keyless verification on registry $REGISTRY_NAME"
    fi

    if $KUBECTL get registry "$REGISTRY_NAME" -n "$NAMESPACE" &>/dev/null; then
        $KUBECTL patch registry "$REGISTRY_NAME" -n "$NAMESPACE" --type=merge -p "$verification_patch"
        info "Registry $REGISTRY_NAME updated with verification config."
    else
        warn "Registry $REGISTRY_NAME not found in namespace $NAMESPACE."
        warn "After creating the registry, apply this patch:"
        echo "  $KUBECTL patch registry <name> -n $NAMESPACE --type=merge -p '$verification_patch'"
    fi
}

sign_instructions() {
    echo ""
    echo "================================================================================"
    echo "  NEXT STEPS"
    echo "================================================================================"
    echo ""
    echo "1. Sign your OCM packages with cosign:"
    echo ""
    echo "   Using cosign sign (keyless):"
    echo "     cosign sign ghcr.io/your-org/your-component@sha256:... "
    echo ""
    echo "   Using cosign sign (keyed):"
    echo "     cosign sign --key cosign.key ghcr.io/your-org/your-component@sha256:..."
    echo ""
    echo "2. The discovery pipeline will now verify signatures before accepting"
    echo "   component versions into the catalog."
    echo ""
    echo "3. To check verification status on a ComponentVersion:"
    echo "     $KUBECTL get componentversion <name> -n $NAMESPACE -o jsonpath='{.status.conditions}'"
    echo ""
    echo "================================================================================"
}

main() {
    echo ""
    echo "============================================"
    echo "  SolAr Trust Setup"
    echo "============================================"
    echo ""

    check_prerequisites

    if [ "$KEYLESS" = "true" ]; then
        setup_keyless_verification
    else
        setup_keyed_verification
    fi

    sign_instructions
}

main "$@"
