#!/usr/bin/env bash

setup_base_cluster() {
    info "CREATING E2E CLUSTER"
    (
      cd "${REPO_ROOT}" || exit 1
      # REGISTRY (not TAG) so build/load/deploy share the same image registry.
      "${MAKE:-make}" e2e-cluster \
        KIND_NODE_IMAGE="$KIND_NODE_IMAGE" \
        KIND_CLUSTER_E2E="$KIND_CLUSTER" \
        REGISTRY="$SOLAR_IMAGE_REGISTRY"
    )
}

setup_solar() {
    info "SETTING UP SOLAR (${SOLAR_NS}):"
    "$HELM" upgrade --install \
        solar "${REPO_ROOT}/charts/solar" \
        --create-namespace \
        --namespace "${SOLAR_NS}" \
        -f "${REPO_ROOT}/test/fixtures/solar.values.yaml" \
        --set apiserver.image.repository="${SOLAR_IMAGE_REGISTRY}/solar-apiserver" \
        --set apiserver.image.tag="${SOLAR_IMAGE_TAG}" \
        --set controller.image.repository="${SOLAR_IMAGE_REGISTRY}/solar-controller-manager" \
        --set controller.image.tag="${SOLAR_IMAGE_TAG}"
    wait_deployment kubectl_solar "${SOLAR_NS}" solar-apiserver
    wait_apiservice kubectl_solar v1alpha1.solar.opendefense.cloud
    log "solar ready"
}

setup_discovery() {
    local kc="$1"
    local release="$2"
    local ns="$3"
    local secret="$4"
    local registry_manifest="$5"

    create_namespace "$kc" "$ns"
    "$kc" label namespace "$ns" trust=enabled --overwrite

    "$kc" create secret generic "${secret}" -n "$ns" \
        --from-literal=username=admin \
        --from-literal=password=admin \
        --dry-run=client -o yaml | "$kc" apply -f -
    "$kc" apply --namespace "$ns" -f "${registry_manifest}"

    "$HELM" upgrade --install \
        "${release}" "${REPO_ROOT}/charts/solar-discovery" \
        --namespace "${ns}" \
        -f "${REPO_ROOT}/test/fixtures/solar-discovery-scan.values.yaml" \
        --set image.repository="${SOLAR_IMAGE_REGISTRY}/solar-discovery" \
        --set image.tag="${SOLAR_IMAGE_TAG}" \
        --set namespace="${ns}"
    wait_deployment "$kc" "${ns}" "${release}"
    log "${release} ready in ${ns}"
}

setup_source_discovery() {
    info "SETTING UP SOURCE DISCOVERY (${SOURCE_NS}):"
    setup_discovery kubectl_source "solar-discovery" "${SOURCE_NS}" "zot-discovery-auth" \
        "${REPO_ROOT}/test/fixtures/e2e/zot-discovery-registry-scan.yaml"
}

setup_dest_discovery() {
    info "SETTING UP DESTINATION DISCOVERY (${DEST_NS}):"
    setup_discovery kubectl_dest "solar-discovery-dst" "${DEST_NS}" "dst-secret" \
        "${CHAINING_FIXTURES}/registry-dst.yaml"
}

setup_arc() {
    info "SETTING UP ARC (${ARC_VERSION} in ${ARC_NS}):"
    "$HELM" upgrade --install \
        arc \
        oci://ghcr.io/opendefensecloud/charts/arc \
        --version "${ARC_VERSION}" \
        --create-namespace \
        --namespace "${ARC_NS}"
    wait_deployment kubectl_arc "${ARC_NS}" arc-apiserver
    wait_apiservice kubectl_arc v1alpha1.arc.opendefense.cloud
    log "arc ready"
}

setup_argo() {
    info "SETTING UP ARGO WORKFLOWS (${ARGO_WORKFLOWS_VERSION}):"
    create_namespace kubectl_arc "${ARGO_NS}"
    kubectl_arc delete --namespace "${ARGO_NS}" configmap workflow-controller-configmap >/dev/null 2>&1 || true
    kubectl_arc apply --namespace "${ARGO_NS}" --server-side -f \
        "https://github.com/argoproj/argo-workflows/releases/download/${ARGO_WORKFLOWS_VERSION}/quick-start-minimal.yaml"
    kubectl_arc patch configmap workflow-controller-configmap \
        --namespace "${ARGO_NS}" \
        --type=merge \
        -p '{"data": {"artifactRepository": "archiveLogs: false\n"}}'
    wait_deployment kubectl_arc "${ARGO_NS}" workflow-controller
    log "argo ready"
}

validate_token() {
    token="$(kubectl_source get secret catalog-chaining-access-token -n "${WORKFLOW_NS}" \
        -o jsonpath='{.data.token}')"
    [ -n "$token" ]
}

# create_kubeconfig_secret builds a kubeconfig that the transfer workflow uses
# to query the source Solar catalog. In-cluster DNS + a dedicated ServiceAccount
# token; multi-cluster setups point KUBECONFIG_SERVER at the source cluster.
create_kubeconfig_secret() {
    info "CREATING KUBECONFIG SECRET (${KUBECONFIG_SECRET} in ${WORKFLOW_NS}):"
    local ca token kubeconfig
    ca="$(kubectl_source get configmap kube-root-ca.crt -n "${WORKFLOW_NS}" \
        -o jsonpath='{.data.ca\.crt}' | base64 -w0)"
    wait_for "ServiceAccount Token to be populated" validate_token
    token="$(kubectl_source get secret catalog-chaining-access-token -n "${WORKFLOW_NS}" \
        -o jsonpath='{.data.token}')"
    kubeconfig="$(cat <<EOF
apiVersion: v1
kind: Config
current-context: solar-a
clusters:
- cluster:
    certificate-authority-data: ${ca}
    server: ${KUBECONFIG_SERVER}
  name: solar-a
contexts:
- context:
    cluster: solar-a
    namespace: solar-a
    user: catalog-chaining-access
  name: solar-a
users:
- name: catalog-chaining-access
  user:
    token: $(printf '%s' "${token}" | base64 -d)
EOF
)"
    kubectl_arc create secret generic "${KUBECONFIG_SECRET}" \
        --namespace "${WORKFLOW_NS}" \
        --from-literal=kubeconfig="${kubeconfig}" \
        --dry-run=client -o yaml | kubectl_arc apply -f -
    log "kubeconfig secret ready"
}

setup_workflow_resources() {
    info "SETTING UP WORKFLOW RESOURCES (${WORKFLOW_NS}):"
    kubectl_arc create secret generic src-reg-secret \
        --namespace "${WORKFLOW_NS}" \
        --from-literal=username="user" \
        --from-literal=password="user" \
        --dry-run=client -o yaml | kubectl_arc apply -f -

    kubectl_arc create secret generic dst-reg-secret \
        --namespace "${WORKFLOW_NS}" \
        --from-literal=username="admin" \
        --from-literal=password="admin" \
        --dry-run=client -o yaml | kubectl_arc apply -f -

    kubectl_source apply --namespace "${WORKFLOW_NS}" -f "${CHAINING_FIXTURES}/rbac-source.yaml"
    kubectl_arc apply -n "${WORKFLOW_NS}" -f "${REPO_ROOT}/assets/workflows/chaining-cluster-workflow-template.yaml"
    # The ocm transfer pipeline template, plus the ClusterArtifactType that
    # declares its parameters. Fetched from the arc release tag in upstream
    # artifact-conduit (examples/ocm), kept in sync with the chart version.
    kubectl_arc apply -f "https://raw.githubusercontent.com/opendefensecloud/artifact-conduit/${ARC_VERSION}/examples/ocm/cluster-workflow-template.yaml"

    # FIXME: release working ARC version
    kubectl_arc apply -f "https://raw.githubusercontent.com/opendefensecloud/artifact-conduit/fbd37328893bb96443c9ba343e9cb44164b7f90b/examples/ocm/cluster-workflow-template.yaml"

    kubectl_arc apply -f "https://raw.githubusercontent.com/opendefensecloud/artifact-conduit/${ARC_VERSION}/examples/ocm/artifact-type.yaml"
    create_kubeconfig_secret
    log "workflow resources ready"
}

setup_done() {
    "${KIND}" get clusters 2>/dev/null | grep -q "${KIND_CLUSTER}" &&
        kubectl_solar get deployment solar-apiserver -n "${SOLAR_NS}" >/dev/null 2>&1 &&
        kubectl_source get deployment solar-discovery -n "${SOURCE_NS}" >/dev/null 2>&1 &&
        kubectl_dest get deployment solar-discovery-dst -n "${DEST_NS}" >/dev/null 2>&1 &&
        kubectl_arc get deployment arc-apiserver -n "${ARC_NS}" >/dev/null 2>&1 &&
        kubectl_arc get deployment workflow-controller -n "${ARGO_NS}" >/dev/null 2>&1 &&
        kubectl_arc get secret "${KUBECONFIG_SECRET}" -n "${WORKFLOW_NS}" >/dev/null 2>&1
}

cmd_setup() {
    if setup_done; then
        log "setup already complete; skipping"
        return 0
    fi
    setup_base_cluster
    setup_solar
    setup_source_discovery
    setup_dest_discovery
    setup_arc
    setup_argo
    setup_workflow_resources
    log "SETUP COMPLETE"
}
