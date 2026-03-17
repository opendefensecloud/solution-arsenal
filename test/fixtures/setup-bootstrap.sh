#!/usr/bin/env bash
set -euo pipefail

KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
KUBECTL="${KUBECTL:-kubectl} --context kind-${KIND_CLUSTER_DEV}"

NAMESPACE="${NAMESPACE:-default}"

echo -e "\nSETTING UP BOOTSTRAP:\n"

echo "Registering Target in namespace '$NAMESPACE'"
$KUBECTL apply -n "$NAMESPACE" -f test/fixtures/e2e/target.yaml

echo "Applying OCIRepository and HelmRelease for bootstrap to namespace '$NAMESPACE'"
$KUBECTL label namespace "$NAMESPACE" trust=enabled --overwrite
$KUBECTL apply -n "$NAMESPACE" -f test/fixtures/e2e/regcred.yaml
$KUBECTL apply -n "$NAMESPACE" -f test/fixtures/e2e/bootstrap-helmrelease.yaml
