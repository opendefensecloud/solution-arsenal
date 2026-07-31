#!/usr/bin/env bash
set -euo pipefail

KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
KUBECTL="${KUBECTL:-kubectl} --context kind-${KIND_CLUSTER_DEV}"

NAMESPACE="${NAMESPACE:-solar-system}"
TARGET_NAME="${TARGET_NAME:-agent-self-registered}"
RENDER_REGISTRY="${RENDER_REGISTRY:-deploy-registry}"
REGISTRY_NAMESPACE="${REGISTRY_NAMESPACE:-$NAMESPACE}"
OUT_KUBECONFIG="${OUT_KUBECONFIG:-/tmp/solar-agent-bootstrap.kubeconfig}"

for ns in "$NAMESPACE" "$REGISTRY_NAMESPACE"; do
  $KUBECTL get namespace "$ns" >/dev/null 2>&1 || $KUBECTL create namespace "$ns"
done

echo -e "\nSETTING UP AGENT SELF-REGISTRATION (workflow A):\n"

echo "Applying bootstrap ServiceAccount/Role/RoleBinding to namespace '$NAMESPACE'"
$KUBECTL apply -n "$NAMESPACE" -f test/fixtures/e2e/agent-self-register-rbac.yaml

echo "Ensuring Registry '$RENDER_REGISTRY' exists in namespace '$REGISTRY_NAMESPACE'"
$KUBECTL apply -n "$REGISTRY_NAMESPACE" -f test/fixtures/e2e/zot-deploy-auth.yaml
$KUBECTL apply -n "$REGISTRY_NAMESPACE" -f test/fixtures/e2e/registry.yaml

if [ "$NAMESPACE" != "$REGISTRY_NAMESPACE" ]; then
  echo "Granting Targets in '$NAMESPACE' access to Registries in '$REGISTRY_NAMESPACE' (ReferenceGrant, ADR-012 Pattern 2)"
  sed "s/TARGET_NAMESPACE/$NAMESPACE/" test/fixtures/e2e/cross-ns-registry-grant.yaml | \
    $KUBECTL apply -n "$REGISTRY_NAMESPACE" -f -
fi

echo "Minting a scoped token for solar-agent-bootstrap (only allowed to get/list/create Targets in '$NAMESPACE')"
SERVER=$($KUBECTL config view --minify --raw -o jsonpath='{.clusters[0].cluster.server}')
CA=$($KUBECTL config view --minify --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')
TOKEN=$($KUBECTL create token solar-agent-bootstrap -n "$NAMESPACE" --duration=2h)

cat > "$OUT_KUBECONFIG" <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: ${SERVER}
    certificate-authority-data: ${CA}
  name: solar
contexts:
- context:
    cluster: solar
    user: bootstrap
  name: solar
current-context: solar
users:
- name: bootstrap
  user:
    token: ${TOKEN}
EOF
echo "Wrote bootstrap kubeconfig to $OUT_KUBECONFIG"

echo -e "\nRun the agent to self-register its own Target:\n"
echo "  go run ./cmd/solar-agent \\"
echo "    --apiserver-kubeconfig=$OUT_KUBECONFIG \\"
echo "    --target-namespace=$NAMESPACE \\"
echo "    --target-name=$TARGET_NAME \\"
echo "    --render-registry=$RENDER_REGISTRY \\"
if [ "$NAMESPACE" != "$REGISTRY_NAMESPACE" ]; then
  echo "    --render-registry-namespace=$REGISTRY_NAMESPACE \\"
fi
echo "    --interval=1h"

echo -e "\nThen verify (RegistryResolved should be True, since '$RENDER_REGISTRY' actually exists):\n"
echo "  kubectl --context kind-${KIND_CLUSTER_DEV} get target $TARGET_NAME -n $NAMESPACE -o yaml"
