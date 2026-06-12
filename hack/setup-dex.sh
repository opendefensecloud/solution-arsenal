#!/usr/bin/env bash

set -euo pipefail

KUBECTL="${KUBECTL:-kubectl}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CERT_DIR="${CERT_DIR:-$PROJECT_DIR/test/fixtures}"

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
