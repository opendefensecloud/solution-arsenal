#!/usr/bin/env bash
# shellcheck disable=SC1091 # sourced phase scripts are analyzed as separate inputs
# Catalog chaining e2e: verifies that two Solar instances chained through ARC
# transfer a discovered OCM package from a source catalog to a destination one.
#
# This entrypoint holds the shared configuration and helpers, then dispatches
# to the phase scripts. The setup phase (e2e-chaining-setup.sh) provisions the
# topology. The cluster-agnostic test phase (e2e-chaining-test.sh) runs the
# scenario and only talks to clusters through the context-aware kubectl
# wrappers below, so SOURCE_CONTEXT/DEST_CONTEXT/ARC_CONTEXT could point at
# different clusters.
#
# Usage: hack/e2e-chaining.sh {setup|test|cleanup|all}

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${REPO_ROOT:-$(cd "${SCRIPT_DIR}/.." && pwd)}"
CHAINING_FIXTURES="${CHAINING_FIXTURES:-${REPO_ROOT}/test/fixtures/chaining}"

ARGO_WORKFLOWS_VERSION="${ARGO_WORKFLOWS_VERSION:-v4.0.8}"
# arc helm chart version == artifact-conduit release tag (with v prefix).
ARC_VERSION="${ARC_VERSION:-v0.2.2}"
# FIXME: set ARC_EXAMPLES_REF to "$ARC_VERSION" and drop this variable once a
# release carries these fixes.
ARC_EXAMPLES_REF="${ARC_EXAMPLES_REF:-5b8de98654ce81622d0adf794538f41d112993cb}"

# --- tooling -----------------------------------------------------------------
KIND="${KIND:-kind}"
KUBECTL="${KUBECTL:-kubectl}"
HELM="${HELM:-helm}"
OCM="${OCM:-ocm}"
DOCKER="${DOCKER:-docker}"
FLUX="${FLUX:-flux}"
YQ="${YQ:-yq}"

# Mirrors Makefile KIND_NODE_IMAGE (derived from ENVTEST_K8S_VERSION); export
# silences shellcheck since only the setup phase consumes it.
export KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.36.1}"

# --- cluster topology ---------------------------------------------------------
# Single-cluster e2e lives on one kind cluster; every context defaults to it.
# The test phase is written against named contexts so it can later run against
# multiple clusters by setting SOURCE_CONTEXT/DEST_CONTEXT/ARC_CONTEXT.
KIND_CLUSTER="${KIND_CLUSTER:-solar-chaining-e2e}"
SOLAR_CONTEXT="${SOLAR_CONTEXT:-kind-${KIND_CLUSTER}}"   # Solar apiserver/controller cluster
SOURCE_CONTEXT="${SOURCE_CONTEXT:-kind-${KIND_CLUSTER}}" # source discovery cluster
DEST_CONTEXT="${DEST_CONTEXT:-kind-${KIND_CLUSTER}}"     # destination discovery cluster
ARC_CONTEXT="${ARC_CONTEXT:-kind-${KIND_CLUSTER}}"       # ARC + Argo Workflows cluster

# --- namespaces ---------------------------------------------------------------
SOLAR_NS="${SOLAR_NS:-solar-system}"   # solar apiserver + controller
SOURCE_NS="${SOURCE_NS:-solar-a}"      # source discovery
SOURCE2_NS="${SOURCE2_NS:-solar-c}"    # second source discovery (second registry)
DEST_NS="${DEST_NS:-solar-b}"          # destination discovery
ARC_NS="${ARC_NS:-arc-system}"
ARGO_NS="${ARGO_NS:-argo}"
REG_NS="${REG_NS:-zot}"                # source + destination registries
WORKFLOW_NS="${WORKFLOW_NS:-default}"  # workflows + transfer secrets

# --- images -------------------------------------------------------------------
# REGISTRY/TAG/E2E_IMAGE_SOURCE are the make variables; prefer them when run via
# `make e2e-chaining-*`. E2E_IMAGE_SOURCE=ghcr switches the setup into CI mode:
# it skips building/loading local images and relies on pull secrets (GHCR_TOKEN)
# so the pods can pull the images built and pushed by the CI workflow.
SOLAR_IMAGE_REGISTRY="${SOLAR_IMAGE_REGISTRY:-${REGISTRY:-localhost/local}}"
SOLAR_IMAGE_TAG="${SOLAR_IMAGE_TAG:-${TAG:-e2e}}"
E2E_IMAGE_SOURCE="${E2E_IMAGE_SOURCE:-local}"
GHCR_TOKEN="${GHCR_TOKEN:-}"

# --- scenario -----------------------------------------------------------------
# Values for the ocm-demo component (make's OCM_DEMO_VERSION=v26.4.2). The
# destination is the zot-deploy registry provisioned by `make e2e-cluster`.
COMPONENT_NAME="${COMPONENT_NAME:-opendefense-cloud-ocm-demo}"
CV_NAME="${CV_NAME:-opendefense-cloud-ocm-demo-v26-4-2}"
CV_TAG="${CV_TAG:-v26.4.2}"
SRC_REGISTRY="${SRC_REGISTRY:-10.96.200.10:443}"
SRC2_REGISTRY="${SRC2_REGISTRY:-10.96.200.11:443}"
DST_REMOTE_URL="${DST_REMOTE_URL:-zot-deploy.zot.svc.cluster.local:443}"
KUBECONFIG_SECRET="${KUBECONFIG_SECRET:-source-solar-kubeconfig}"
KUBECONFIG_SERVER="${KUBECONFIG_SERVER:-https://10.96.0.1:443}"

# --- timing --------------------------------------------------------------------
WAIT_TIMEOUT="${WAIT_TIMEOUT:-300}"
WORKFLOW_TIMEOUT="${WORKFLOW_TIMEOUT:-900}"

# --- logging -------------------------------------------------------------------
info() { printf '\n==> %s\n' "$*"; }
log() { printf '%s\n' "$*"; }
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

# --- context-aware kubectl ------------------------------------------------------
kubectl_solar() { "${KUBECTL}" --context "${SOLAR_CONTEXT}" "$@"; }
kubectl_source() { "${KUBECTL}" --context "${SOURCE_CONTEXT}" "$@"; }
kubectl_dest() { "${KUBECTL}" --context "${DEST_CONTEXT}" "$@"; }
kubectl_arc() { "${KUBECTL}" --context "${ARC_CONTEXT}" "$@"; }

# --- helpers --------------------------------------------------------------------

# wait_for <description> <command...>
# Polls the command until it exits 0 or the timeout is reached.
wait_for() {
    local description="$1"
    shift 1
    local deadline=$((SECONDS + WAIT_TIMEOUT))

    while ! "$@"; do
        if ((SECONDS >= deadline)); then
            fail "timed out after ${WAIT_TIMEOUT}s waiting for: ${description}"
        fi
        sleep 5
    done
    log "ok: ${description}"
}

# wait_deployment <kubectl-function> <namespace> <deployment-name>
wait_deployment() {
    local kc="$1"
    local ns="$2"
    local deployment="$3"
    info "WAITING FOR ${deployment} DEPLOYMENT (${ns}):"
    "$kc" wait deployment/"${deployment}" --namespace "${ns}" \
        --for condition=Available --timeout 5m
}

# wait_apiservice <kubectl-function> <apiservice-name>
wait_apiservice() {
    local kc="$1"
    local name="$2"
    info "WAITING FOR APISERVICE ${name}:"
    "$kc" wait apiservice/"${name}" --for condition=Available --timeout 5m
}

# create_namespace <kubectl-function> <namespace>
create_namespace() {
    local kc="$1"
    local ns="$2"
    "$kc" get namespace "$ns" >/dev/null 2>&1 && return 0
    "$kc" create namespace "$ns"
}

# --- commands -------------------------------------------------------------------

usage() {
    cat <<EOF
Usage: $(basename "$0") {setup|test|cleanup|all}

Catalog chaining e2e between two Solar instances via ARC.

Commands:
  setup    Provision everything: create the kind cluster, install the base
           infra via hack/dev-cluster.sh, then one Solar instance, three
           solar-discovery workers (two source registries with different
           credentials, one destination), arc, argo and the transfer secrets.
  test     Run the scenario: push the OCM packages to both source registries
           at two sub-namespace depths, wait for source discovery, trigger the
           transfer workflow, verify the destination discovery, then re-run the
           workflow to verify dedup skips everything already transferred.
  cleanup  Delete the kind cluster (single-cluster mode).
  all      setup + test + cleanup.

Configuration is via environment variables; see this file's header.
EOF
}

cmd_cleanup() {
    info "DELETING KIND CLUSTER ${KIND_CLUSTER}:"
    "$KIND" delete cluster --name "${KIND_CLUSTER}" 2>/dev/null || log "cluster ${KIND_CLUSTER} not found"
}

main() {
    local cmd="${1:-}"

    case "${cmd}" in
        setup)
            source "${SCRIPT_DIR}/e2e-chaining-setup.sh"
            cmd_setup
            ;;
        test)
            source "${SCRIPT_DIR}/e2e-chaining-test.sh"
            cmd_test
            ;;
        cleanup)
            cmd_cleanup
            ;;
        all)
            source "${SCRIPT_DIR}/e2e-chaining-setup.sh"
            source "${SCRIPT_DIR}/e2e-chaining-test.sh"
            cmd_setup
            cmd_test
            cmd_cleanup
            ;;
        *)
            usage
            exit 1
            ;;
    esac
}

main "$@"
