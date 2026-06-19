#!/usr/bin/env bash

set -euo pipefail

KUBECTL="${KUBECTL:-kubectl}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CERT_DIR="${CERT_DIR:-$PROJECT_DIR/test/fixtures}"

# Guard against accidentally applying cluster-admin RBAC to a non-local cluster.
# This script is for local Kind-based dev/test only.
CURRENT_CONTEXT="$($KUBECTL config current-context 2>/dev/null || true)"
EXPECTED_CONTEXT="${KIND_CLUSTER:+kind-${KIND_CLUSTER}}"
if [[ "${ALLOW_NON_LOCAL_CLUSTER:-false}" != "true" ]]; then
    if [[ -n "$EXPECTED_CONTEXT" && "$CURRENT_CONTEXT" != "$EXPECTED_CONTEXT" ]]; then
        echo "Refusing to run against context '${CURRENT_CONTEXT:-<none>}' (expected '$EXPECTED_CONTEXT')." >&2
        echo "Switch context or set ALLOW_NON_LOCAL_CLUSTER=true to override intentionally." >&2
        exit 1
    fi
    if [[ -z "$EXPECTED_CONTEXT" && ! "$CURRENT_CONTEXT" =~ ^kind- ]]; then
        echo "Refusing to apply Dex test fixtures to non-kind context: ${CURRENT_CONTEXT:-<none>}" >&2
        echo "Set ALLOW_NON_LOCAL_CLUSTER=true to override intentionally." >&2
        exit 1
    fi
fi

echo -e "\nSETTING UP DEX (OIDC IdP):\n"

# Create namespace
$KUBECTL create namespace dex 2>/dev/null || true

# Create TLS secret from generated certificates
echo "Creating Dex TLS secret..."
$KUBECTL create secret tls dex-tls -n dex \
    --cert="$CERT_DIR/dex-tls.crt" \
    --key="$CERT_DIR/dex-tls.key" \
    --dry-run=client -o yaml | $KUBECTL apply -f -

# Deploy Dex config and deployment
$KUBECTL apply -f "$PROJECT_DIR/test/fixtures/e2e/dex/dex-config.yaml"
$KUBECTL apply -f "$PROJECT_DIR/test/fixtures/e2e/dex/dex-deployment.yaml"

echo "Waiting for Dex deployment to be available..."
$KUBECTL wait deployment/dex -n dex --for=condition=Available --timeout=120s

# Grant the static Dex user cluster-admin for dev/test
$KUBECTL apply -f "$PROJECT_DIR/test/fixtures/e2e/dex/dex-rbac.yaml"

echo "Dex setup complete."
