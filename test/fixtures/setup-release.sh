#!/usr/bin/env bash
set -euo pipefail

KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
KUBECTL="${KUBECTL:-kubectl} --context kind-${KIND_CLUSTER_DEV}"

NAMESPACE="${NAMESPACE:-default}"

$KUBECTL get namespace "$NAMESPACE" >/dev/null 2>&1 || \
  $KUBECTL create namespace "$NAMESPACE"

echo -e "\nSETTING UP RELEASE:\n"
echo "Applying Component and Release resources to namespace '$NAMESPACE'..."

$KUBECTL apply -n "$NAMESPACE" -f test/fixtures/e2e/componentversion.yaml
$KUBECTL apply -n "$NAMESPACE" -f test/fixtures/e2e/release.yaml

echo "Done. Resources applied:"
echo "  - Component: test-ocm-software-toi-demo-helmdemo"
echo "  - ComponentVersion: test-ocm-software-toi-demo-helmdemo-0-12-0"
echo "  - Release: test-ocm-software-toi-demo-helmdemo-0-12-0-release"
