#!/usr/bin/env bash
#
# Remove the resources seeded by demo-app.sh and the demo application namespace,
# while leaving the cluster, the discovery setup and the populated catalog in
# place. Re-run demo-app.sh afterwards to seed again.
set -euo pipefail

NS="${NS:-solar-system}"
APP_NS="${APP_NS:-demo}"
FIXTURES="test/fixtures/e2e"

# kubectl pinned to the dev cluster context, so call sites read as plain
# `kubectl ...`. Honors a KUBECTL override (e.g. from the Makefile) for the binary.
KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
kubectl() { command "${KUBECTL:-kubectl}" --context "kind-${KIND_CLUSTER_DEV}" "$@"; }

step() { printf '\n>>> %s\n' "$*"; }

remove_bootstrap() {
    # Delete the Flux resources first so the workload is uninstalled before the
    # SolAr resources that produced its chart go away.
    step "Removing the bootstrap Flux resources..."
    kubectl delete -n "$NS" helmrelease/solar-bootstrap ocirepository/solar-bootstrap \
        --ignore-not-found --timeout=2m
}

remove_solar_resources() {
    # Deleting the ReleaseBinding first lets the render GC controller clean up the
    # RenderBinding and RenderArtifact.
    step "Removing the demo SolAr resources..."
    local fixture
    for fixture in releasebinding release registrybinding target registry; do
        kubectl delete -n "$NS" -f "$FIXTURES/$fixture.yaml" --ignore-not-found
    done
    kubectl delete -n "$NS" secret regcred --ignore-not-found
}

remove_app_namespace() {
    step "Removing the application namespace '$APP_NS'..."
    kubectl delete namespace "$APP_NS" --ignore-not-found --timeout=2m
}

main() {
    remove_bootstrap
    remove_solar_resources
    remove_app_namespace
    echo
    echo "Demo resources removed. The cluster, discovery and catalog are kept."
}

main "$@"
