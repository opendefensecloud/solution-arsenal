#!/usr/bin/env bash
#
# Seed the ocm-demo application end to end on an existing dev cluster:
#
#   transfer -> discover -> release -> render -> bootstrap -> running workload
#
# It mirrors the happy path the e2e suite exercises (test/e2e/e2e_test.go) and
# reuses the same fixtures under test/fixtures/e2e, so the demo and the e2e tests
# cannot drift. It is idempotent: re-running applies the same resources and waits
# on the same conditions.
#
# Assumes `make dev-cluster` has already run (SolAr + discovery + both Zots up).
set -euo pipefail

YQ="${YQ:-yq}"

# SolAr resources and the render pipeline live in NS. It defaults to
# solar-system: that is where dev-cluster's discovery writes the
# ComponentVersion and where the discovery Registry (zot-scan) and the
# zot-deploy pull secret already live, so every reference resolves in-namespace
# without ReferenceGrants. The application itself is deployed into APP_NS (the
# Release's targetNamespace).
NS="${NS:-solar-system}"
APP_NS="${APP_NS:-demo}"

FIXTURES="test/fixtures/e2e"
DEPLOY_REGISTRY_HOST="zot-deploy.zot.svc.cluster.local"

COMPONENT_VERSION="opendefense-cloud-ocm-demo-v26-4-2"
TARGET="cluster-1"
RELEASE="test-opendefense-cloud-ocm-demo-v26-4-2-release"

POLL_ATTEMPTS="${POLL_ATTEMPTS:-60}"
POLL_DELAY="${POLL_DELAY:-5}"

# kubectl pinned to the dev cluster context, so call sites read as plain
# `kubectl ...`. Honors a KUBECTL override (e.g. from the Makefile) for the binary.
KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
kubectl() { command "${KUBECTL:-kubectl}" --context "kind-${KIND_CLUSTER_DEV}" "$@"; }

step() { printf '\n>>> %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

# Poll a command until it succeeds. Usage: wait_until "<description>" <cmd> [args...]
wait_until() {
    local description="$1"
    shift
    local attempt
    for (( attempt = 1; attempt <= POLL_ATTEMPTS; attempt++ )); do
        "$@" && return 0
        sleep "$POLL_DELAY"
    done
    die "timed out after $(( POLL_ATTEMPTS * POLL_DELAY ))s waiting for ${description}"
}

# --- conditions we wait on ---------------------------------------------------

component_version_exists() {
    kubectl get componentversion "$COMPONENT_VERSION" -n "$NS" >/dev/null 2>&1
}

registry_bindings_exist() {
    kubectl get registrybinding cluster-1-deploy-registry cluster-1-discovery-registry \
        -n "$NS" >/dev/null 2>&1
}

target_exists() {
    kubectl get target "$TARGET" -n "$NS" >/dev/null 2>&1
}

release_resolved() {
    local status
    status="$(kubectl get release "$RELEASE" -n "$NS" \
        -o jsonpath='{.status.conditions[?(@.type=="ComponentVersionResolved")].status}' 2>/dev/null)"
    [ "$status" = "True" ]
}

render_artifact_exists() {
    [ -n "$(kubectl get renderartifacts -n "$NS" -o name 2>/dev/null)" ]
}

# --- phases ------------------------------------------------------------------

ensure_catalog() {
    if component_version_exists; then
        step "ComponentVersion already in the catalog, skipping the transfer."
        return
    fi
    step "Transferring the ocm-demo component into the discovery registry..."
    bash test/fixtures/setup-discovery.sh
    step "Waiting for discovery to add the ComponentVersion to the catalog..."
    wait_until "ComponentVersion $COMPONENT_VERSION" component_version_exists
}

prepare_app_namespace() {
    step "Ensuring the application namespace '$APP_NS' exists..."
    kubectl get namespace "$APP_NS" >/dev/null 2>&1 || kubectl create namespace "$APP_NS"
    kubectl label namespace "$APP_NS" trust=enabled --overwrite
}

deploy_credentials() {
    step "Deploying registry pull credentials..."
    kubectl apply -n "$NS" -f "$FIXTURES/regcred.yaml"
    kubectl apply -n "$APP_NS" -f "$FIXTURES/regcred.yaml"
}

create_registry_and_bindings() {
    step "Creating the deploy Registry and RegistryBindings..."
    kubectl apply -n "$NS" -f "$FIXTURES/registry.yaml"
    kubectl apply -n "$NS" -f "$FIXTURES/registrybinding.yaml"
    wait_until "the RegistryBindings" registry_bindings_exist
    # Give the controller's informer cache a moment to see the RegistryBindings
    # before the ReleaseBinding triggers rendering (mirrors the e2e ordering).
    sleep 5
}

register_target() {
    step "Registering the Target..."
    kubectl apply -n "$NS" -f "$FIXTURES/target.yaml"
    wait_until "the Target $TARGET" target_exists
}

create_release() {
    step "Creating the Release and waiting for it to resolve the ComponentVersion..."
    kubectl apply -n "$NS" -f "$FIXTURES/release.yaml"
    wait_until "the Release to resolve its ComponentVersion" release_resolved
}

trigger_render() {
    step "Binding the Release to the Target to trigger the render pipeline..."
    kubectl apply -n "$NS" -f "$FIXTURES/releasebinding.yaml"
    step "Waiting for the render pipeline to produce a RenderArtifact..."
    wait_until "a RenderArtifact" render_artifact_exists
}

bootstrap() {
    step "Bootstrapping the cluster so Flux pulls the rendered chart and deploys it..."
    # The bootstrap OCIRepository fixture is namespaced to 'default'; point it at
    # the namespace we actually rendered into.
    local url="oci://${DEPLOY_REGISTRY_HOST}/${NS}/bootstrap-${TARGET}"
    "$YQ" ".spec.url = \"$url\"" "$FIXTURES/bootstrap-ocirepository.yaml" \
        | kubectl apply -n "$NS" -f -
    kubectl apply -n "$NS" -f "$FIXTURES/bootstrap-helmrelease.yaml"

    step "Waiting for the bootstrap OCIRepository and HelmRelease to become ready..."
    kubectl wait --for=condition=Ready -n "$NS" ocirepository/solar-bootstrap --timeout=300s
    kubectl wait --for=condition=Ready -n "$NS" helmrelease/solar-bootstrap --timeout=300s
}

print_summary() {
    cat <<EOF

Demo app seeded. What to look at next:

  # catalog, release and render state
  kubectl -n $NS get components,componentversions,releases,rendertasks,renderartifacts

  # bootstrap and the deployed workload
  kubectl -n $NS get ocirepository,helmrelease
  kubectl -n $APP_NS get helmrelease,pods
EOF
}

main() {
    ensure_catalog
    prepare_app_namespace
    deploy_credentials
    create_registry_and_bindings
    register_target
    create_release
    trigger_render
    bootstrap
    print_summary
}

main "$@"
