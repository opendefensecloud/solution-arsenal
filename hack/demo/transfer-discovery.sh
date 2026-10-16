#!/usr/bin/env bash
#
# Transfer the ocm-demo CTF into an already-reachable OCI registry. This is the
# one place the OCM transfer invocation and its config live, so the dev/demo
# scripts cannot drift from each other. The caller is responsible for making the
# target reachable (e.g. a port-forward); this script only runs the transfer.
#
# Usage: transfer-discovery.sh <target-registry-url>
#   e.g. transfer-discovery.sh https://localhost:4443/test
set -euo pipefail

OCM="${OCM:-ocm}"
# Credentials for the target registry. OCM v2 has no rootcerts config type, so
# trust for the cluster's self-signed certificate is supplied through
# SSL_CERT_FILE instead.
OCM_CONFIG="${OCM_CONFIG:-./test/fixtures/e2e/ocmconfig}"
OCM_UPLOADERS="${OCM_UPLOADERS:-./test/fixtures/ocm-uploaders.yaml}"
OCM_DEMO_DIR="${OCM_DEMO_DIR:-$(pwd)/test/fixtures/ocm-demo-ctf}"
OCM_DEMO_COMPONENT="${OCM_DEMO_COMPONENT:-opendefense.cloud/ocm-demo}"
OCM_DEMO_VERSION="${OCM_DEMO_VERSION:-v26.4.2}"
# hack/dev-cluster.sh writes the cluster CA here.
export SSL_CERT_FILE="${SSL_CERT_FILE:-$(pwd)/test/fixtures/ca.crt}"

target="${1:?usage: transfer-discovery.sh <target-registry-url>}"

# The uploader configuration publishes each image and chart as a real tagged
# artifact in the target registry so workloads can pull it. Without it a resource
# stays addressable only by digest inside the component's own repository.
#
# The absolute reference this records carries whichever host the transfer was
# pushed through (a port-forward, typically). SolAr re-anchors it to the address
# it reads the component from, so that is not baked into the deployment.
exec "$OCM" --config "$OCM_UPLOADERS" --config "$OCM_CONFIG" transfer cv \
    "ctf::${OCM_DEMO_DIR}//${OCM_DEMO_COMPONENT}:${OCM_DEMO_VERSION}" "$target"
