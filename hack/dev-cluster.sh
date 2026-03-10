#!/usr/bin/env bash

set -euo pipefail

KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
KUBECTL="${KUBECTL:-kubectl} --context kind-${KIND_CLUSTER_DEV}"
HELM="${HELM:-helm}"
DEV_TAG="${DEV_TAG:-dev.$(date '+%Y%m%d%H%M%S')}"
HELMDEMO_DIR="${HELMDEMO_DIR:-$(pwd)/test/fixtures/helmdemo-ctf}"
OCM="${OCM:-ocm}"
YQ="${YQ:-yq}"

export DEV_TAG

retry() {
    local max_attempts="$1"
    local delay="$2"
    local description="$3"
    shift 3

    for i in $(seq 1 "$max_attempts"); do
        if "$@"; then
            return 0
        fi
        echo "Attempt $i/$max_attempts: $description, retrying in ${delay}s..."
        sleep "$delay"
    done
    echo "Failed after $max_attempts attempts: $description"
    return 1
}

setup_cert_manager() {
    local yaml_url="https://github.com/cert-manager/cert-manager/releases/download/v1.19.1/cert-manager.yaml"

    echo -e "\nSETTING UP CERT-MANAGER:\n"
    $KUBECTL apply -f "$yaml_url"
    echo "Waiting for cert-manager-webhook to be available (timeout: 5m)..."
    $KUBECTL wait deployment.apps/cert-manager-webhook \
        --for condition=Available \
        --namespace cert-manager \
        --timeout 5m
    # shellcheck disable=SC2086
    retry 5 5 "cert-manager webhook not ready" \
        $KUBECTL apply -n cert-manager -f test/fixtures/certmanager.yaml
    echo "Waiting for selfsigned-ca certificate to be ready (timeout: 5m)..."
    $KUBECTL wait certificates.cert-manager.io/selfsigned-ca \
        --for condition=Ready \
        --namespace cert-manager \
        --timeout 5m
    $KUBECTL get secrets -n cert-manager selfsigned-ca-secret -oyaml \
        | $YQ '.data."tls.crt" | @base64d' > test/fixtures/ca.crt
}

setup_trust_manager() {
    echo -e "\nSETTING UP TRUST-MANAGER:\n"
    $HELM upgrade --install \
        --namespace cert-manager \
        trust-manager \
        oci://quay.io/jetstack/charts/trust-manager \
        --version v0.20.2
    echo "Waiting for trust-manager to be available (timeout: 5m)..."
    $KUBECTL wait deployment.apps/trust-manager \
        --for condition=Available \
        --namespace cert-manager \
        --timeout 5m
    # shellcheck disable=SC2086
    retry 5 5 "trust-manager webhook not ready" \
        $KUBECTL apply -n cert-manager -f test/fixtures/trustmanager.yaml
    $KUBECTL label namespace default trust=enabled --overwrite
}

setup_zot_certs() {
    echo -e "\nSETTING UP CERT FOR ZOTs:\n"
    $KUBECTL create namespace zot
    $KUBECTL apply --namespace zot \
        -f test/fixtures/zot-cert.yaml
}

setup_zot_discovery() {
    echo -e "\nSETTING UP ZOT (DISCOVERY):\n"
    $HELM upgrade --install \
        --create-namespace \
        --namespace=zot \
        --repo=https://zotregistry.dev/helm-charts \
        -f test/fixtures/zot-discovery.values.yaml \
        zot-discovery zot
}

setup_zot_deploy() {
    echo -e "\nSETTING UP ZOT (DEPLOY):\n"
    $HELM upgrade --install \
        --create-namespace \
        --namespace=zot \
        --repo=https://zotregistry.dev/helm-charts \
        -f test/fixtures/zot-deploy.values.yaml \
        zot-deploy zot
}

setup_solar() {
    echo -e "\nSETTING UP SOLAR:\n"
    $HELM upgrade --install \
        --create-namespace \
        --namespace=solar-system \
        solar charts/solar \
        -f test/fixtures/solar.values.yaml \
        --set apiserver.image.tag="$DEV_TAG" \
        --set controller.image.tag="$DEV_TAG" \
        --set renderer.image.tag="$DEV_TAG" \
        --set discovery.image.tag="$DEV_TAG"
    $KUBECTL apply --namespace=solar-system \
        -f test/fixtures/e2e/zot-deploy-auth.yaml
}

transfer_via_ocm() {
    echo -e "\nSETTING UP DISCOVERY:\n"
    echo "Waiting for zot-discovery rollout (timeout: 5m)..."
    $KUBECTL rollout status statefulset/zot-discovery \
        -n zot \
        --timeout 5m
    echo "Starting port-forward for zot-discovery service..."
    $KUBECTL -n zot port-forward svc/zot-discovery 4443:443 &
    echo "Waiting for port-forward to establish..."
    sleep 2
    echo "Transferring helmdemo chart via OCM..."
    SSL_CERT_FILE=test/fixtures/ca.crt $OCM \
        --config test/fixtures/ocmconfig \
        transfer ctf "$HELMDEMO_DIR" https://localhost:4443/test
    echo "Cleaning up port-forward..."
    pkill -f "port-forward.*4443:443" || true
}

main() {
    setup_cert_manager
    setup_trust_manager
    setup_zot_certs
    setup_zot_discovery
    setup_zot_deploy
    setup_solar
    transfer_via_ocm

    echo -e "\nDONE"
}

main "$@"
