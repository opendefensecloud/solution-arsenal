#!/usr/bin/env bash
# Test phase of the catalog-chaining e2e. Sourced by e2e-chaining.sh, which
# provides the shared configuration and helpers (hack/e2e-chaining.sh).
#
# Runs the actual scenario against an already-provisioned topology:
#   1. push the OCM package to both source registries, at different
#      sub-namespace depths ('test' on the first, 'a/b' on the second)
#   2. wait for both source discoveries to pick them up
#   3. trigger the ARC transfer workflow
#   4. wait for the workflow to succeed, and check every source registry
#      produced its own transfer with its own credential
#   5. verify the destination discovery picked it up
#   6. re-run the workflow and verify it fans out to nothing (dedup)
#
# Cluster-agnostic: all API access goes through the context-aware kubectl
# wrappers, so SOURCE_CONTEXT/DEST_CONTEXT/ARC_CONTEXT may point at the same
# cluster (single-cluster e2e) or at different clusters (multi-cluster e2e).

# --- scenario ----------------------------------------------------------------

# Port-forward bookkeeping: one forward at a time, always torn down, including
# when the transfer inside it fails.
PF_PID=""
cleanup_pf() {
    if [ -n "${PF_PID}" ]; then
        kill "${PF_PID}" 2>/dev/null || true
        PF_PID=""
    fi
}
trap cleanup_pf EXIT

# push_to_registry <zot-service-name> <path-prefix>
# Pushes the ocm-demo CTF under the given registry sub-namespace. The shared
# ocmconfig authenticates as admin against localhost, which both registries
# accept
push_to_registry() {
    local svc="$1" prefix="$2" port ready
    port=$((20000 + RANDOM % 10000))

    kubectl_source -n "${REG_NS}" port-forward "svc/${svc}" "${port}:443" &
    PF_PID=$!

    ready=false
    for _ in $(seq 1 30); do
        if (exec 3<>"/dev/tcp/localhost/${port}") 2>/dev/null; then
            ready=true
            break
        fi
        sleep 1
    done
    [ "${ready}" = "true" ] || fail "port-forward to ${svc} never became ready on localhost:${port}"

    (
        cd "${REPO_ROOT}" || exit 1
        OCM="${OCM}" bash hack/demo/transfer-discovery.sh "https://localhost:${port}/${prefix}"
    )
    cleanup_pf
}

# One package per registry, at different sub-namespace depths: single-level on
# the first, multi-level on the second. They must not share a registry: a
# Component's object name derives from the OCM component name alone, so pushing
# the same component under two prefixes in ONE registry produces a single
# Component object whose spec.repository changes with scan order.
push_ocm_package() {
    info "PUSHING OCM PACKAGES TO SOURCE REGISTRIES:"
    kubectl_source rollout status statefulset/zot-discovery -n "${REG_NS}" --timeout 5m
    push_to_registry zot-discovery "test"

    kubectl_source rollout status statefulset/zot-discovery-2 -n "${REG_NS}" --timeout 5m
    push_to_registry zot-discovery-2 "a/b"
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

    local repository
    repository="$(kubectl_source get components.solar.opendefense.cloud -n "${SOURCE_NS}" "${COMPONENT_NAME}" \
        -o jsonpath='{.spec.repository}')"
    [[ "${repository}" == test/* ]] \
        || fail "expected source repository under 'test/', got '${repository}'"
    log "source discovery has ${COMPONENT_NAME} @ ${registry} (${repository})"
}

wait_source2_discovery() {
    info "WAITING FOR SECOND SOURCE DISCOVERY (${SOURCE2_NS}):"
    wait_for "Component to be present in second source" \
        kubectl_source get components.solar.opendefense.cloud -n "${SOURCE2_NS}" "${COMPONENT_NAME}"

    wait_for "ComponentVersion to be present in second source" \
        kubectl_source get componentversions.solar.opendefense.cloud -n "${SOURCE2_NS}" "${CV_NAME}"

    local registry repository
    registry="$(kubectl_source get components.solar.opendefense.cloud -n "${SOURCE2_NS}" "${COMPONENT_NAME}" \
        -o jsonpath='{.spec.registry}')"
    [[ "${registry}" == "${SRC2_REGISTRY}" ]] \
        || fail "unexpected second source registry '${registry}' (expected ${SRC2_REGISTRY})"

    repository="$(kubectl_source get components.solar.opendefense.cloud -n "${SOURCE2_NS}" "${COMPONENT_NAME}" \
        -o jsonpath='{.spec.repository}')"
    [[ "${repository}" == a/b/* ]] \
        || fail "expected second source repository under 'a/b/', got '${repository}'"
    log "second source discovery has ${COMPONENT_NAME} @ ${registry} (${repository})"
}

# Every Secret the transfer workflow names must exist in the workflow namespace
# before we submit. A missing one otherwise surfaces only as an ArtifactWorkflow
# that never gets a status.phase, which the workflow waits on until it times out
verify_referenced_secrets() {
    info "CHECKING REFERENCED SECRETS EXIST (${WORKFLOW_NS}):"
    local names name missing=0
    names="$("${YQ}" -o=json '.' "${CHAINING_FIXTURES}/transfer-workflow.yaml" | jq -r '
      [ .spec.arguments.parameters[]
        | select(.name == "srcSecretName" or .name == "dstSecretName" or .name == "kubeconfigSecret")
        | .value ]
      + ( [ .spec.arguments.parameters[] | select(.name == "srcSecrets") | .value ]
          | map(fromjson | to_entries[].value) )
      | map(select(. != null and . != "")) | unique | .[]')"

    while IFS= read -r name; do
        [ -n "${name}" ] || continue
        if kubectl_arc get secret "${name}" -n "${WORKFLOW_NS}" >/dev/null 2>&1; then
            log "  ok      ${name}"
        else
            log "  MISSING ${name}"
            missing=$((missing + 1))
        fi
    done <<< "${names}"

    [ "${missing}" -eq 0 ] \
        || fail "${missing} Secret(s) referenced by the transfer workflow are missing from ${WORKFLOW_NS}"
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
    log "  source (${SOURCE2_NS}):       ${COMPONENT_NAME} (${SRC2_REGISTRY})"
    log "  destination (${DEST_NS}):    ${COMPONENT_NAME} (${DST_REMOTE_URL})"
    log "  transfer workflow:           ${wf}"
}

count_artifactworkflows() {
    kubectl_arc get artifactworkflows.arc.opendefense.cloud -n "${WORKFLOW_NS}" \
        --no-headers 2>/dev/null | wc -l | tr -d ' '
}

# Each source registry must have produced its own transfer. A wrong credential
# surfaces here as a Failed ArtifactWorkflow, because neither registry accepts
# the other's ARC secret.
verify_multi_registry_transfers() {
    info "VERIFYING PER-REGISTRY TRANSFERS:"
    local count failed
    count="$(count_artifactworkflows)"
    [[ "${count}" -ge 2 ]] \
        || fail "expected at least 2 ArtifactWorkflows (one per source registry), got ${count}"

    failed="$(kubectl_arc get artifactworkflows.arc.opendefense.cloud -n "${WORKFLOW_NS}" \
        -o jsonpath='{range .items[*]}{.status.phase}{"\n"}{end}' \
        | grep -c -E 'Failed|Error' || true)"
    [[ "${failed}" -eq 0 ]] \
        || fail "${failed} ArtifactWorkflow(s) failed; a wrong source credential is the likely cause"
    log "${count} ArtifactWorkflow(s), none failed"
}

# A second run must fan out to nothing: everything is already in the
# destination catalog.
verify_dedup() {
    info "VERIFYING DEDUP ON A SECOND RUN:"
    local before after
    before="$(count_artifactworkflows)"

    kubectl_arc create --namespace "${WORKFLOW_NS}" -f "${CHAINING_FIXTURES}/transfer-workflow.yaml"
    wait_workflow

    after="$(count_artifactworkflows)"
    [[ "${after}" -eq "${before}" ]] \
        || fail "second run created $((after - before)) new ArtifactWorkflow(s); dedup did not skip transferred items"
    log "second run created no new ArtifactWorkflows (${after} total)"
}

cmd_test() {
    verify_referenced_secrets
    push_ocm_package
    wait_source_discovery
    wait_source2_discovery
    trigger_transfer
    wait_workflow
    verify_multi_registry_transfers
    verify_dest_discovery
    verify_dedup
    summary
}
