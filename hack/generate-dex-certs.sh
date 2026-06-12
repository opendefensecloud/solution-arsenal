#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CERT_DIR="${CERT_DIR:-$PROJECT_DIR/test/fixtures}"

DEX_CA_CERT="$CERT_DIR/dex-ca.crt"
DEX_CA_KEY="$CERT_DIR/dex-ca.key"
DEX_TLS_CERT="$CERT_DIR/dex-tls.crt"
DEX_TLS_KEY="$CERT_DIR/dex-tls.key"
AUTH_CONFIG="/tmp/solar-dex-auth-config.yaml"

if [[ -f "$DEX_CA_CERT" && -f "$DEX_TLS_CERT" ]]; then
    echo "Dex certificates already exist, skipping generation."
else
    echo "Generating Dex CA certificate..."
    openssl req -x509 -newkey rsa:2048 -keyout "$DEX_CA_KEY" -out "$DEX_CA_CERT" \
        -days 3650 -nodes -subj "/CN=Dex Test CA" 2>/dev/null

    echo "Generating Dex TLS certificate..."
    openssl req -newkey rsa:2048 -keyout "$DEX_TLS_KEY" -out /tmp/dex-tls.csr \
        -nodes -subj "/CN=localhost" 2>/dev/null

    openssl x509 -req -in /tmp/dex-tls.csr \
        -CA "$DEX_CA_CERT" -CAkey "$DEX_CA_KEY" -CAcreateserial \
        -out "$DEX_TLS_CERT" -days 3650 \
        -extfile <(printf "subjectAltName=DNS:localhost,DNS:dex.dex.svc.cluster.local,DNS:dex.dex.svc,DNS:dex") \
        2>/dev/null

    rm -f /tmp/dex-tls.csr "$CERT_DIR/dex-ca.srl"

    echo "Dex certificates generated:"
    echo "  CA cert:  $DEX_CA_CERT"
    echo "  TLS cert: $DEX_TLS_CERT"
fi

# Generate K8s AuthenticationConfiguration with CA cert inlined.
# This is mounted into the Kind node for the API server to use.
echo "Generating K8s OIDC AuthenticationConfiguration..."
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
