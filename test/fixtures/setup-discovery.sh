#!/usr/bin/env bash
set -euo pipefail

KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
KUBECTL="${KUBECTL:-kubectl} --context kind-${KIND_CLUSTER_DEV}"
OCM="${OCM:-ocm}"
OCM_DEMO_DIR="${OCM_DEMO_DIR:-$(pwd)/test/fixtures/ocm-demo-ctf}"
# ocmconfig with the rootcerts block that trusts the cluster CA, the same config
# the e2e suite uses. A credentials-only config fails TLS against the
# self-signed zot cert.
OCM_CONFIG="${OCM_CONFIG:-./test/fixtures/e2e/ocmconfig}"
LOCAL_PORT="${LOCAL_PORT:-4443}"

echo -e "\nSETTING UP DISCOVERY:\n"
echo "Waiting for zot-discovery rollout (timeout: 5m)..."
$KUBECTL rollout status statefulset/zot-discovery \
    -n zot \
    --timeout 5m

echo "Starting port-forward for zot-discovery service..."
$KUBECTL -n zot port-forward "svc/zot-discovery" "${LOCAL_PORT}:443" &
pf_pid=$!
# Always tear the port-forward down, including when the transfer fails. The
# previous version left it running on any non-zero exit.
trap 'kill "$pf_pid" 2>/dev/null || true' EXIT

echo "Waiting for the port-forward to accept connections..."
ready=false
for _ in $(seq 1 30); do
    if (exec 3<>"/dev/tcp/localhost/${LOCAL_PORT}") 2>/dev/null; then
        ready=true
        break
    fi
    sleep 1
done
if [ "$ready" != "true" ]; then
    echo "port-forward to zot-discovery never became ready on localhost:${LOCAL_PORT}" >&2
    exit 1
fi

echo "Transferring ocm-demo component via OCM..."
OCM="$OCM" OCM_CONFIG="$OCM_CONFIG" OCM_DEMO_DIR="$OCM_DEMO_DIR" \
    hack/demo/transfer-discovery.sh "https://localhost:${LOCAL_PORT}/test"

echo "Transfer done. Discovery scans the registry on its interval and creates"
echo "the Component/ComponentVersion in the solar-system namespace shortly after."
