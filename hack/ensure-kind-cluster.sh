#!/usr/bin/env bash
# Ensure a Kind cluster with a specific node image exists, then exit 0.
#
# Three outcomes:
#   1. Cluster doesn't exist               → create with the requested image.
#   2. Cluster exists with the right image → reuse silently (idempotent).
#   3. Cluster exists with a different image:
#      - KIND_RECREATE=1  → delete + recreate with the requested image.
#      - otherwise        → fail loudly with an actionable message.
#
# Required args:
#   $1   Kind cluster name (e.g. solar-test-e2e)
#   $2   Kind node image   (e.g. kindest/node:v1.33.0)
#
# Optional env:
#   KIND            path to kind binary    (default: kind)
#   DOCKER          path to docker binary  (default: docker)
#   KIND_RECREATE   set to "1" to delete + recreate on image mismatch
#                   instead of failing

set -euo pipefail

CLUSTER_NAME="${1:?Usage: $0 <cluster-name> <node-image>}"
NODE_IMAGE="${2:?Usage: $0 <cluster-name> <node-image>}"
KIND="${KIND:-kind}"
DOCKER="${DOCKER:-docker}"
KIND_RECREATE="${KIND_RECREATE:-}"

# Outcome 1 — cluster doesn't exist. Exact-line match (-qx) avoids false
# positives when one cluster name is a substring of another.
if ! "$KIND" get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  echo "Creating Kind cluster '$CLUSTER_NAME' with image $NODE_IMAGE..."
  "$KIND" create cluster --name "$CLUSTER_NAME" --image "$NODE_IMAGE"
  exit 0
fi

# kind names the control-plane container "<cluster-name>-control-plane" by
# convention; Config.Image reflects the original --image argument verbatim,
# so a direct string compare works without parsing.
existing=$("$DOCKER" inspect "${CLUSTER_NAME}-control-plane" \
  --format '{{.Config.Image}}' 2>/dev/null || true)

# Outcome 2 — match, reuse.
if [ "$existing" = "$NODE_IMAGE" ]; then
  echo "Kind cluster '$CLUSTER_NAME' already exists with $NODE_IMAGE; reusing."
  exit 0
fi

# Outcome 3a — mismatch, KIND_RECREATE=1, delete + recreate.
if [ "$KIND_RECREATE" = "1" ]; then
  echo "Kind cluster '$CLUSTER_NAME' has image '$existing' but '$NODE_IMAGE' requested;" \
       "KIND_RECREATE=1, deleting + recreating..."
  "$KIND" delete cluster --name "$CLUSTER_NAME"
  "$KIND" create cluster --name "$CLUSTER_NAME" --image "$NODE_IMAGE"
  exit 0
fi

# Outcome 3b — mismatch, no opt-in, fail loud.
cat >&2 <<EOF
ERROR: Kind cluster '$CLUSTER_NAME' has image '$existing' but '$NODE_IMAGE' was requested.
       Delete the cluster first ('$KIND delete cluster --name $CLUSTER_NAME')
       then retry, or pass KIND_RECREATE=1 to delete + recreate automatically.
EOF
exit 1
