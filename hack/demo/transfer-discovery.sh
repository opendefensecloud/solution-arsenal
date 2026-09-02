#!/usr/bin/env bash
#
# Transfer the ocm-demo CTF into an already-reachable OCI registry, using the
# single ca-trusting ocmconfig. This is the one place the OCM transfer
# invocation and its config live, so the dev/demo scripts cannot drift from each
# other. The caller is responsible for making the target reachable (e.g. a
# port-forward); this script only runs the transfer.
#
# Usage: transfer-discovery.sh <target-registry-url>
#   e.g. transfer-discovery.sh https://localhost:4443/test
set -euo pipefail

OCM="${OCM:-ocm}"
# ocmconfig with the rootcerts block that trusts the cluster CA. The
# credentials-only variant fails TLS against the self-signed zot cert.
OCM_CONFIG="${OCM_CONFIG:-./test/fixtures/e2e/ocmconfig}"
OCM_DEMO_DIR="${OCM_DEMO_DIR:-$(pwd)/test/fixtures/ocm-demo-ctf}"

target="${1:?usage: transfer-discovery.sh <target-registry-url>}"

exec "$OCM" --config "$OCM_CONFIG" transfer ctf "$OCM_DEMO_DIR" "$target"
