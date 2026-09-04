#!/usr/bin/env bash
set -euo pipefail

# Sets up the ADR-018 agent flow on the dev cluster: a user creates the Target,
# the agent is given a scoped credential for it and only verifies on startup.

KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
KUBECTL="${KUBECTL:-kubectl} --context kind-${KIND_CLUSTER_DEV}"

NAMESPACE="${NAMESPACE:-solar-system}"
TARGET_NAME="${TARGET_NAME:-cluster-1}"
RENDER_REGISTRY="${RENDER_REGISTRY:-deploy-registry}"
REGISTRY_NAMESPACE="${REGISTRY_NAMESPACE:-$NAMESPACE}"
OUT_KUBECONFIG="${OUT_KUBECONFIG:-/tmp/solar-agent.kubeconfig}"

for ns in "$NAMESPACE" "$REGISTRY_NAMESPACE"; do
  $KUBECTL get namespace "$ns" >/dev/null 2>&1 || $KUBECTL create namespace "$ns"
done

echo -e "\nSETTING UP SOLAR-AGENT:\n"

echo "Applying the agent's solar-cluster ServiceAccount/Role/RoleBinding to namespace '$NAMESPACE'"
$KUBECTL apply -n "$NAMESPACE" -f test/fixtures/e2e/agent-rbac.yaml

echo "Applying the agent's local-cluster ClusterRole (nodes, pods, ocirepositories, helmreleases -- all read-only)"
sed "s/AGENT_NAMESPACE/$NAMESPACE/" test/fixtures/e2e/agent-local-rbac.yaml | $KUBECTL apply -n "$NAMESPACE" -f -

echo "Ensuring Registry '$RENDER_REGISTRY' exists in namespace '$REGISTRY_NAMESPACE'"
$KUBECTL apply -n "$REGISTRY_NAMESPACE" -f test/fixtures/e2e/zot-deploy-auth.yaml
$KUBECTL apply -n "$REGISTRY_NAMESPACE" -f test/fixtures/e2e/registry.yaml

if [ "$NAMESPACE" != "$REGISTRY_NAMESPACE" ]; then
  echo "Granting Targets in '$NAMESPACE' access to Registries in '$REGISTRY_NAMESPACE' (ReferenceGrant, ADR-012 Pattern 2)"
  sed "s/TARGET_NAMESPACE/$NAMESPACE/" test/fixtures/e2e/cross-ns-registry-grant.yaml | \
    $KUBECTL apply -n "$REGISTRY_NAMESPACE" -f -
fi

echo "Creating Target '$TARGET_NAME' (this is the user's job, not the agent's)"
cat <<EOF | $KUBECTL apply -n "$NAMESPACE" -f -
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Target
metadata:
  name: ${TARGET_NAME}
spec:
  renderRegistryRef:
    name: ${RENDER_REGISTRY}
$(if [ "$NAMESPACE" != "$REGISTRY_NAMESPACE" ]; then echo "    namespace: ${REGISTRY_NAMESPACE}"; fi)
EOF

# a kubeconfig stands in for the OAuth client credential ADR-018
# specifies, which SolAr will render into the agent's manifests. Swap this block
# for a client-id/client-secret once the issuer exists.
echo "Minting a scoped token for solar-agent (may only get/list Targets in '$NAMESPACE')"
SERVER=$($KUBECTL config view --minify --raw -o jsonpath='{.clusters[0].cluster.server}')
CA=$($KUBECTL config view --minify --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')
TOKEN=$($KUBECTL create token solar-agent -n "$NAMESPACE" --duration=2h)

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
    user: agent
  name: solar
current-context: solar
users:
- name: agent
  user:
    token: ${TOKEN}
EOF
echo "Wrote agent kubeconfig to $OUT_KUBECONFIG"

echo -e "\nRun the agent; it resolves the Target above and then reports:\n"
echo "  go run ./cmd/solar-agent \\"
echo "    --apiserver-kubeconfig=$OUT_KUBECONFIG \\"
echo "    --target-namespace=$NAMESPACE \\"
echo "    --target-name=$TARGET_NAME \\"
echo "    --interval=1h"

echo -e "\nTo see the failure mode, run it against a Target name that does not exist:\n"
echo "  go run ./cmd/solar-agent --apiserver-kubeconfig=$OUT_KUBECONFIG \\"
echo "    --target-namespace=$NAMESPACE --target-name=nope --interval=1h"
