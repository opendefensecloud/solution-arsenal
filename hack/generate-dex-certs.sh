#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CERT_DIR="${CERT_DIR:-$PROJECT_DIR/test/fixtures}"
# Project-local working dir for generated dev/e2e state. Kept out of the system
# /tmp on purpose: /tmp is reaped (tmpfiles/reboot), and a missing bind-mount
# source gets recreated by Docker as a directory, breaking the next run.
WORK_DIR="${WORK_DIR:-$PROJECT_DIR/tmp/ui}"
mkdir -p "$WORK_DIR"

DEX_CA_CERT="$CERT_DIR/dex-ca.crt"
DEX_CA_KEY="$CERT_DIR/dex-ca.key"
DEX_TLS_CERT="$CERT_DIR/dex-tls.crt"
DEX_TLS_KEY="$CERT_DIR/dex-tls.key"
AUTH_CONFIG="$WORK_DIR/dex-auth-config.yaml"
KIND_CONFIG_TMPL="$PROJECT_DIR/test/fixtures/e2e/kind-config-oidc.yaml.tmpl"
KIND_CONFIG_OUT="$WORK_DIR/kind-config-oidc.yaml"

if [[ -f "$DEX_CA_CERT" && -f "$DEX_CA_KEY" && -f "$DEX_TLS_CERT" && -f "$DEX_TLS_KEY" ]]; then
    echo "Dex certificates already exist, skipping generation."
else
    mkdir -p "$CERT_DIR"
    echo "Generating Dex CA certificate..."
    openssl req -x509 -newkey rsa:2048 -keyout "$DEX_CA_KEY" -out "$DEX_CA_CERT" \
        -days 3650 -nodes -subj "/CN=Dex Test CA" 2>/dev/null

    echo "Generating Dex TLS certificate..."
    openssl req -newkey rsa:2048 -keyout "$DEX_TLS_KEY" -out "$WORK_DIR/dex-tls.csr" \
        -nodes -subj "/CN=localhost" 2>/dev/null

    openssl x509 -req -in "$WORK_DIR/dex-tls.csr" \
        -CA "$DEX_CA_CERT" -CAkey "$DEX_CA_KEY" -CAcreateserial \
        -out "$DEX_TLS_CERT" -days 3650 \
        -extfile <(printf "subjectAltName=DNS:localhost,DNS:dex.dex.svc.cluster.local,DNS:dex.dex.svc,DNS:dex") \
        2>/dev/null

    rm -f "$WORK_DIR/dex-tls.csr" "$CERT_DIR/dex-ca.srl"

    echo "Dex certificates generated:"
    echo "  CA cert:  $DEX_CA_CERT"
    echo "  TLS cert: $DEX_TLS_CERT"
fi

# Generate K8s AuthenticationConfiguration with CA cert inlined.
# This is mounted into the Kind node for the API server to use.
echo "Generating K8s OIDC AuthenticationConfiguration..."
# Defensive: a leftover bind-mount source from a previous run can be a directory.
rm -rf "$AUTH_CONFIG"
CA_CONTENT=$(sed 's/^/        /' "$DEX_CA_CERT")
cat > "$AUTH_CONFIG" <<EOF
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
  - issuer:
      url: https://localhost:5556
      audiences:
        - solar-ui
      certificateAuthority: |
${CA_CONTENT}
    claimMappings:
      username:
        claim: email
        prefix: ""
      groups:
        claim: groups
        prefix: ""
EOF

echo "Auth config written to: $AUTH_CONFIG"

# Render the Kind cluster config with the absolute auth-config path inlined.
# Kind requires an absolute hostPath, so it cannot be checked in directly.
echo "Generating Kind cluster config (OIDC)..."
sed "s|@@AUTH_CONFIG@@|$AUTH_CONFIG|g" "$KIND_CONFIG_TMPL" > "$KIND_CONFIG_OUT"
echo "Kind config written to: $KIND_CONFIG_OUT"
