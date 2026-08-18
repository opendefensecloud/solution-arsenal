#!/usr/bin/env bash
# Test phase of the catalog-chaining e2e. Sourced by e2e-chaining.sh, which
# provides the shared configuration and helpers (hack/e2e-chaining.sh).
#
# Runs the actual scenario against an already-provisioned topology:
#   1. push the OCM package to the source registry
#   2. wait for the source discovery to pick it up
#   3. trigger the ARC transfer workflow
#   4. wait for the workflow to succeed
#   5. verify the destination discovery picked it up
#
# Cluster-agnostic: all API access goes through the context-aware kubectl
# wrappers, so SOURCE_CONTEXT/DEST_CONTEXT/ARC_CONTEXT may point at the same
# cluster (single-cluster e2e) or at different clusters (multi-cluster e2e).

# --- scenario ----------------------------------------------------------------

push_ocm_package() {
    info "PUSHING OCM PACKAGE TO SOURCE REGISTRY (${SRC_REGISTRY}):"
    (
        cd "${REPO_ROOT}" || exit 1
        KIND_CLUSTER_DEV="${KIND_CLUSTER}" \
        LOCAL_PORT=$((20000 + RANDOM % 10000)) \
            bash test/fixtures/setup-discovery.sh
    )
}

wait_source_discovery() {
    info "WAITING FOR SOURCE DISCOVERY (${SOURCE_NS}):"
    wait_for "Component to be present in source" \
        kubectl_source get components.solar.opendefense.cloud -n "${SOURCE_NS}" "${COMPONENT_NAME}"
    wait_for "ComponentVersion to be present in source " \
        kubectl_source get componentversions.solar.opendefense.cloud -n "${SOURCE_NS}" "${CV_NAME}"

    local registry
    registry="$(kubectl_source get components.solar.opendefense.cloud -n "${SOURCE_NS}" "${COMPONENT_NAME}" \
        -o jsonpath='{.spec.registry}')"
    [[ "${registry}" == "${SRC_REGISTRY}" ]] \
        || fail "unexpected source component registry '${registry}' (expected ${SRC_REGISTRY})"
    log "source discovery has ${COMPONENT_NAME} @ ${registry}"
}

trigger_transfer() {
    info "TRIGGERING CATALOG TRANSFER:"
    kubectl_arc create --namespace "${WORKFLOW_NS}" -f "${CHAINING_FIXTURES}/transfer-workflow.yaml"
    wait_for "transfer workflow created" \
      [ -n "$(latest_chaining_workflow)" ]
}

# latest_chaining_workflow prints the most recently created chaining workflow.
latest_chaining_workflow() {
    kubectl_arc get workflows.argoproj.io -n "${WORKFLOW_NS}" \
        -l solar.opendefense.cloud/e2e=chaining \
        --sort-by=.metadata.creationTimestamp \
        -o jsonpath='{.items[-1:].metadata.name}'
}

dump_workflow_logs() {
    local wf="$1"
    log "--- workflow ${wf} nodes ---"
    kubectl_arc get workflows.argoproj.io -n "${WORKFLOW_NS}" "${wf}" \
        -o jsonpath='{range .status.nodes.*}- Name: {.displayName}{"\n"}  Phase: {.phase}{"\n"}  Message:{.message}{"\n"}{end}'
    log "--- artifactworkflows ---"
    kubectl_arc get artifactworkflows.arc.opendefense.cloud -n "${WORKFLOW_NS}" \
        -o jsonpath='{range .items[*]}{.metadata.name} {.status.phase}{"\n"}{end}' || true
    log "--- workflow pod logs ---"
    while IFS= read -r pod; do
        [[ -n "${pod}" ]] || continue
        log "### ${pod}"
        kubectl_arc logs -n "${WORKFLOW_NS}" "${pod}" --tail=50 --all-containers || true
    done < <(kubectl_arc get pods -n "${WORKFLOW_NS}" -l workflows.argoproj.io/workflow="${wf}" -o name 2>/dev/null || true)
}

wait_workflow() {
    info "WAITING FOR TRANSFER WORKFLOW TO SUCCEED:"
    local wf
    wf="$(latest_chaining_workflow)"
    [[ -n "${wf}" ]] || fail "no chaining workflow found"
    log "tracking workflow ${wf} (timeout: ${WORKFLOW_TIMEOUT}s)"

    local phase
    local deadline=$((SECONDS + WORKFLOW_TIMEOUT))
    while :; do
        phase="$(kubectl_arc get workflows.argoproj.io -n "${WORKFLOW_NS}" "${wf}" \
            -o jsonpath='{.status.phase}' 2>/dev/null || true)"
        if [[ "${phase}" == "Succeeded" ]]; then
            log "workflow ${wf} Succeeded"
            return 0
        fi
        if [[ "${phase}" == "Failed" || "${phase}" == "Error" ]]; then
            dump_workflow_logs "${wf}"
            fail "workflow ${wf} ended with phase ${phase}"
        fi
        if ((SECONDS > deadline)); then
            dump_workflow_logs "${wf}"
            fail "timed out after ${WORKFLOW_TIMEOUT}s waiting for workflow ${wf} (phase: ${phase:-unknown})"
        fi
        sleep 10
    done
}

verify_dest_discovery() {
    info "VERIFYING DESTINATION DISCOVERY (${DEST_NS}):"

    wait_for "Component to be present in destination" \
        kubectl_dest get components.solar.opendefense.cloud -n "${DEST_NS}" "${COMPONENT_NAME}"
    wait_for "ComponentVersion to be present in destination" \
        kubectl_dest get componentversions.solar.opendefense.cloud -n "${DEST_NS}" "${CV_NAME}"

    local registry tag component
    registry="$(kubectl_dest get components.solar.opendefense.cloud -n "${DEST_NS}" "${COMPONENT_NAME}" \
        -o jsonpath='{.spec.registry}')"
    component="$(kubectl_dest get componentversions.solar.opendefense.cloud -n "${DEST_NS}" "${CV_NAME}" \
        -o jsonpath='{.spec.componentRef.name}')"
    tag="$(kubectl_dest get componentversions.solar.opendefense.cloud -n "${DEST_NS}" "${CV_NAME}" \
        -o jsonpath='{.spec.tag}')"

    [[ "${registry}" == "${DST_REMOTE_URL}" ]] \
        || fail "unexpected destination component registry '${registry}' (expected ${DST_REMOTE_URL})"
    [[ "${component}" == "${COMPONENT_NAME}" ]] \
        || fail "unexpected destination ComponentVersion ref '${component}' (expected ${COMPONENT_NAME})"
    [[ "${tag}" == "${CV_TAG}" ]] \
        || fail "unexpected destination ComponentVersion tag '${tag}' (expected ${CV_TAG})"
    log "destination discovery has ${COMPONENT_NAME} @ ${registry}"
}

summary() {
    local wf
    wf="$(latest_chaining_workflow)"
    info "CHAINING E2E PASSED"
    log "  source (${SOURCE_NS}):       ${COMPONENT_NAME} (${SRC_REGISTRY})"
    log "  destination (${DEST_NS}):    ${COMPONENT_NAME} (${DST_REMOTE_URL})"
    log "  transfer workflow:           ${wf}"
}

cmd_test() {
    push_ocm_package
    wait_source_discovery
    trigger_transfer
    wait_workflow
    verify_dest_discovery
    summary
}
