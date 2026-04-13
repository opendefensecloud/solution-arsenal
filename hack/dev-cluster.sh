#!/usr/bin/env bash

set -euo pipefail

KIND_CLUSTER="${KIND_CLUSTER:-solar-dev}"
SKIP_SOLAR="${SKIP_SOLAR:-false}"
TAG="${TAG:-latest}"

FLUX="${FLUX:-flux}"
HELM="${HELM:-helm}"
OCM_DEMO_DIR="${OCM_DEMO_DIR:-$(pwd)/test/fixtures/ocm-demo-ctf}"
KUBECTL="${KUBECTL:-kubectl}"
OCM="${OCM:-ocm}"
YQ="${YQ:-yq}"

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
        | $YQ -r '.data."tls.crt" | @base64d' > test/fixtures/ca.crt
}

# setup_trust_manager installs and configures trust-manager via Helm, waits for its deployment to become available, applies the test fixture with retries, and labels the `default` namespace with `trust=enabled`.
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
    # Acts as a stable alias for the discovery webhook address which is dynamic due to the randomized name of the test namespace. 
    # The discovery Zot points in its config to this fixed service's address. During testing we update this service to point to the actual 
    # discovery webhook address without a Zot redeploy.
    $KUBECTL apply --namespace zot \
        -f test/fixtures/discovery-webhook-ptr-svc.yaml
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

# setup_zots recreates the `zot` namespace and deploys Zot certificates, discovery, and deployment components.
setup_zots() {
    echo -e "\nSETTING UP NAMESPACE FOR ZOTs:\n"
    $KUBECTL get namespace zot 2>/dev/null && $KUBECTL delete ns zot
    $KUBECTL create namespace zot

    setup_zot_certs
    setup_zot_discovery
    setup_zot_deploy
}

# setup_flux sets up Flux by running the Flux CLI pre-checks, installing Flux into the cluster, and verifying the installation.
setup_flux() {
    echo -e "\nSETTING UP FLUX:\n"
    $FLUX check --pre
    $FLUX install
    $FLUX check

    # Setup private CA for flux
    $KUBECTL label namespace flux-system trust=enabled --overwrite
    $KUBECTL patch deployment.apps/source-controller \
        --namespace flux-system \
        -p '{
          "spec": {
            "template": {
              "spec": {
                "containers": [
                  {
                    "name": "manager",
                    "volumeMounts": [
                      {
                        "name": "root-bundle",
                        "mountPath": "/etc/ssl/certs/root-bundle.pem",
                        "subPath": "trust-bundle.pem"
                      }
                    ],
                    "env": [
                      {
                        "name": "SSL_CERT_FILE",
                        "value": "/etc/ssl/certs/root-bundle.pem"
                      }
                    ]
                  }
                ],
                "volumes": [
                  {
                    "name": "root-bundle",
                    "configMap": {
                      "name": "root-bundle"
                    }
                  }
                ]
              }
            }
          }
        }'
    $KUBECTL wait deployment.apps/source-controller \
        --for condition=Available \
        --namespace flux-system \
        --timeout 5m
}

# setup_solar installs the Solar Helm chart into the solar-system namespace and applies the Zot deployment authorization manifest, setting component image tags to the current TAG.
setup_solar() {
    echo -e "\nSETTING UP SOLAR:\n"
    $HELM upgrade --install \
        --create-namespace \
        --namespace=solar-system \
        solar charts/solar \
        -f test/fixtures/solar.values.yaml \
        --set apiserver.image.tag="$TAG" \
        --set controller.image.tag="$TAG" \
        --set renderer.image.tag="$TAG" \
        --set discovery.image.tag="$TAG" \
        --set ui.image.tag="$TAG"
    $KUBECTL apply --namespace=solar-system \
        -f test/fixtures/e2e/zot-deploy-auth.yaml
}

# main orchestrates cluster setup by invoking cert-manager, trust-manager, Zot components, Flux, and (unless SKIP_SOLAR is "true") Solar, then prints DONE.
main() {
    echo "Switching kubectl context to kind-${KIND_CLUSTER}..."
    $KUBECTL config use-context "kind-${KIND_CLUSTER}"

    setup_cert_manager
    setup_trust_manager
    setup_zots
    setup_flux

    if [[ "$SKIP_SOLAR" != "true" ]]; then
        setup_solar
    fi

    echo -e "\nDONE"
}

main "$@"
