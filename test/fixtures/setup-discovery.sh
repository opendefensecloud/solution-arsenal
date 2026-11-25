#!/usr/bin/env bash
set -euo pipefail

KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
KUBECTL="${KUBECTL:-kubectl} --context kind-${KIND_CLUSTER_DEV}"
OCM="${OCM:-ocm}"
OCM_DEMO_DIR="${OCM_DEMO_DIR:-$(pwd)/test/fixtures/ocm-demo-ctf}"
# Registry credentials, the same config the e2e suite uses. OCM v2 has no
# rootcerts config type; transfer-discovery.sh points SSL_CERT_FILE at the
# cluster CA so TLS against the self-signed zot certificate verifies.
OCM_CONFIG="${OCM_CONFIG:-./test/fixtures/e2e/ocmconfig}"

echo -e "\nSETTING UP DISCOVERY:\n"
echo "Waiting for zot-discovery rollout (timeout: 5m)..."
$KUBECTL rollout status statefulset/zot-discovery \
	-n zot \
	--timeout 5m

$KUBECTL wait service/zot-discovery -n zot \
	--for=jsonpath='{.status.loadBalancer.ingress[0].ip}'=172.30.255.200 --timeout=5m

echo "Transferring ocm-demo component via OCM..."
OCM="$OCM" OCM_CONFIG="$OCM_CONFIG" OCM_DEMO_DIR="$OCM_DEMO_DIR" \
	hack/demo/transfer-discovery.sh "https://172.30.255.200:443/test"

echo "Transfer done. Discovery scans the registry on its interval and creates"
echo "the Component/ComponentVersion in the solar-system namespace shortly after."
