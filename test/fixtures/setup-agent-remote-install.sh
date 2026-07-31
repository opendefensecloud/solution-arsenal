#!/usr/bin/env bash
set -euo pipefail

KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
KUBECTL="${KUBECTL:-kubectl} --context kind-${KIND_CLUSTER_DEV}"

NAMESPACE="${NAMESPACE:-tenant-demo}"

$KUBECTL get namespace "$NAMESPACE" >/dev/null 2>&1 || \
  $KUBECTL create namespace "$NAMESPACE"

echo -e "\nSETTING UP AGENT REMOTE INSTALL (workflow B):\n"

echo "Applying remote-installer ServiceAccount/ClusterRole to namespace '$NAMESPACE'"
$KUBECTL apply -n "$NAMESPACE" -f test/fixtures/e2e/agent-remote-install-rbac.yaml

echo "Binding the ClusterRole to the ServiceAccount in '$NAMESPACE'"
$KUBECTL create clusterrolebinding solar-agent-remote-installer \
  --clusterrole=solar-agent-remote-installer \
  --serviceaccount="$NAMESPACE:solar-agent-remote-installer" \
  --dry-run=client -o yaml | $KUBECTL apply -f -

echo "Minting a scoped token and building the remote-access kubeconfig Secret"
# NOTE: this demo is self-referential, the "remote" cluster is the same
# kind-solar-dev cluster solar-apiserver/solar-controller-manager run in, so the
# in-cluster DNS name is used instead of the host-mapped API server port (which
# isn't reachable from inside a Pod). Against a real target cluster, this Secret
# would instead hold that cluster's own externally-reachable kubeconfig.
CA=$($KUBECTL get configmap kube-root-ca.crt -n "$NAMESPACE" -o jsonpath='{.data.ca\.crt}' | base64 | tr -d '\n')
TOKEN=$($KUBECTL create token solar-agent-remote-installer -n "$NAMESPACE" --duration=2h)

REMOTE_KUBECONFIG=$(cat <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://kubernetes.default.svc
    certificate-authority-data: ${CA}
  name: solar
contexts:
- context:
    cluster: solar
    user: remote-installer
  name: solar
current-context: solar
users:
- name: remote-installer
  user:
    token: ${TOKEN}
EOF
)

$KUBECTL create secret generic solar-agent-remote-kubeconfig -n "$NAMESPACE" \
  --from-literal=kubeconfig="$REMOTE_KUBECONFIG" \
  --dry-run=client -o yaml | $KUBECTL apply -f -

echo "Applying Target with agentAccessSecretRef set to namespace '$NAMESPACE'"
$KUBECTL apply -n "$NAMESPACE" -f test/fixtures/e2e/agent-remote-install-target.yaml

echo -e "\nsolar-controller-manager reacts on its own; the first attempt can take up to 30s"
echo -e "(the reconcile backoff interval). Watch it:\n"
echo "  kubectl --context kind-${KIND_CLUSTER_DEV} get target agent-remote-install -n $NAMESPACE -w"
echo
echo "Once AgentInstalled=True, the placeholder install (MarkerInstaller -- see"
echo "pkg/controller/agent_installer.go) leaves a marker behind:"
echo
echo "  kubectl --context kind-${KIND_CLUSTER_DEV} get configmap solar-agent-installed -n solar-system -o yaml"
