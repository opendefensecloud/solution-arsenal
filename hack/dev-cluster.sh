#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${REPO_ROOT:-$(cd "${SCRIPT_DIR}/.." && pwd)}"

KIND_CLUSTER="${KIND_CLUSTER:-solar-dev}"
SKIP_SOLAR="${SKIP_SOLAR:-false}"
SKIP_DISCOVERY="${SKIP_DISCOVERY:-false}"
TAG="${TAG:-latest}"
REGISTRY="${REGISTRY:-localhost/local}"
GHCR_TOKEN="${GHCR_TOKEN:-}"

FLUX="${FLUX:-flux}"
HELM="${HELM:-helm}"
KUBECTL="${KUBECTL:-kubectl}"
YQ="${YQ:-yq}"
DOCKER="${DOCKER:-docker}"
KIND="${KIND:-kind}"
MAKE="${MAKE:-make}"

# Versions are exported by the Makefile.
CERTMANAGER_VERSION="${CERTMANAGER_VERSION:?must be set (see Makefile)}"
TRUSTMANAGER_VERSION="${TRUSTMANAGER_VERSION:?must be set (see Makefile)}"
ZOT_VERSION="${ZOT_VERSION:?must be set (see Makefile)}"
CLOUD_PROVIDER_KIND_VERSION="${CLOUD_PROVIDER_KIND_VERSION:?must be set (see Makefile)}"
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:?must be set (see Makefile)}"

export KIND_EXPERIMENTAL_DOCKER_NETWORK=solar-kind

setup_docker_network() {
    local config
    if config="$("$DOCKER" network inspect solar-kind --format \
        '{{.Driver}}|{{.Internal}}|{{range .IPAM.Config}}{{.Subnet}}|{{.IPRange}}|{{.Gateway}}{{end}}' 2>/dev/null)"; then
        if [[ "$config" != 'bridge|false|172.30.0.0/16|172.30.0.0/24|172.30.0.1' ]]; then
            echo "Docker network solar-kind has incompatible configuration: $config" >&2
            return 1
        fi
    else
        "$DOCKER" network create --driver bridge \
            --subnet 172.30.0.0/16 --ip-range 172.30.0.0/24 --gateway 172.30.0.1 solar-kind
    fi
}

setup_kind_cluster() {
    if [[ -n "${KIND_CONFIG:-}" ]]; then
        "$MAKE" -C "$REPO_ROOT" setup-local-cluster KIND_CLUSTER="$KIND_CLUSTER" KIND_CONFIG="$KIND_CONFIG"
    else
        KIND="$KIND" DOCKER="$DOCKER" KIND_RECREATE="${KIND_RECREATE:-}" \
            bash "${SCRIPT_DIR}/ensure-kind-cluster.sh" "$KIND_CLUSTER" "$KIND_NODE_IMAGE"
    fi
    if [[ "${E2E_IMAGE_SOURCE:-local}" == local ]]; then
        "$MAKE" -C "$REPO_ROOT" docker-build-local-images TAG="$TAG" REGISTRY="$REGISTRY"
        "$MAKE" -C "$REPO_ROOT" kind-load-local-images TAG="$TAG" REGISTRY="$REGISTRY" KIND_CLUSTER="$KIND_CLUSTER"
    fi

    echo "Switching kubectl context to kind-${KIND_CLUSTER}..."
    $KUBECTL config use-context "kind-${KIND_CLUSTER}"
}

setup_load_balancer() {
    if "$DOCKER" container inspect solar-kind-cloud-provider >/dev/null 2>&1; then
        "$DOCKER" start solar-kind-cloud-provider >/dev/null
    else
        "$DOCKER" run -d --name solar-kind-cloud-provider --restart unless-stopped \
            --network solar-kind -e KIND_EXPERIMENTAL_DOCKER_NETWORK=solar-kind \
            -v /var/run/docker.sock:/var/run/docker.sock \
            "registry.k8s.io/cloud-provider-kind/cloud-controller-manager:${CLOUD_PROVIDER_KIND_VERSION}" \
            --gateway-channel=disabled
    fi
    $KUBECTL label nodes --all node.kubernetes.io/exclude-from-external-load-balancers- --overwrite
}

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
    local yaml_url="https://github.com/cert-manager/cert-manager/releases/download/${CERTMANAGER_VERSION}/cert-manager.yaml"

    echo -e "\nSETTING UP CERT-MANAGER (${CERTMANAGER_VERSION}):\n"
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

    # Install the CA on Kind nodes so containerd trusts registry certs
    for node in $($KUBECTL get nodes -o name | sed 's|node/||'); do
        "$DOCKER" cp test/fixtures/ca.crt "${node}:/usr/local/share/ca-certificates/solar-ca.crt"
        "$DOCKER" exec "${node}" update-ca-certificates
        "$DOCKER" exec "${node}" systemctl restart containerd
    done
    echo "Installed CA on Kind nodes and restarted containerd."
    echo "Waiting for nodes to be Ready after containerd restart..."
    $KUBECTL wait --for=condition=Ready nodes --all --timeout=2m
    # The containerd restart killed all pods including cert-manager's. Without
    # re-waiting here, the next step (trust-manager helm install) renders a
    # chart that references cert-manager.io/v1 Certificate/Issuer kinds, and
    # the apiserver's REST mapper lookup fails because the cert-manager
    # webhook is still coming back up.
    echo "Waiting for cert-manager deployments to recover (timeout: 5m)..."
    $KUBECTL wait deployment.apps \
        --selector='app.kubernetes.io/instance=cert-manager' \
        --for=condition=Available \
        --namespace cert-manager \
        --timeout=5m
}

# setup_trust_manager installs and configures trust-manager via Helm, waits for its deployment to become available, applies the test fixture with retries, and labels the `default` namespace with `trust=enabled`.
setup_trust_manager() {
    echo -e "\nSETTING UP TRUST-MANAGER (${TRUSTMANAGER_VERSION}):\n"
    $HELM upgrade --install \
        --namespace cert-manager \
        trust-manager \
        oci://quay.io/jetstack/charts/trust-manager \
        --version "${TRUSTMANAGER_VERSION}"
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
    $KUBECTL wait certificate/zot-tls --namespace zot --for=condition=Ready --timeout=5m
}

setup_zot_discovery() {
    echo -e "\nSETTING UP ZOT (${ZOT_VERSION}) for DISCOVERY:\n"
    $HELM upgrade --install \
        zot-discovery \
        oci://ghcr.io/project-zot/helm-charts/zot \
        --version "${ZOT_VERSION}" \
        --create-namespace \
        --namespace=zot \
        -f test/fixtures/zot-discovery.values.yaml
    # Acts as a stable alias for the discovery webhook address which is dynamic due to the randomized name of the test namespace. 
    # The discovery Zot points in its config to this fixed service's address. During testing we update this service to point to the actual 
    # discovery webhook address without a Zot redeploy.
    $KUBECTL apply --namespace zot \
        -f test/fixtures/discovery-webhook-ptr-svc.yaml
}

setup_zot_deploy() {
    echo -e "\nSETTING UP ZOT (${ZOT_VERSION}) for DEPLOY:\n"
    $HELM upgrade --install \
        zot-deploy \
        oci://ghcr.io/project-zot/helm-charts/zot \
        --version "${ZOT_VERSION}" \
        --create-namespace \
        --namespace=zot \
        -f test/fixtures/zot-deploy.values.yaml
}

# setup_zots recreates the `zot` namespace and deploys Zot certificates, discovery, and deployment components.
setup_zots() {
    echo -e "\nSETTING UP NAMESPACE FOR ZOTs:\n"
    $KUBECTL get namespace zot 2>/dev/null && $KUBECTL delete ns zot
    $KUBECTL create namespace zot

    $KUBECTL apply --namespace zot -f test/fixtures/zot-services.yaml \
        -l 'app.kubernetes.io/instance!=zot-discovery-2'
    setup_zot_certs
    setup_zot_discovery
    setup_zot_deploy
    $KUBECTL wait service/zot-discovery --namespace zot \
        --for=jsonpath='{.status.loadBalancer.ingress[0].ip}'=172.30.255.200 --timeout=5m
    $KUBECTL wait service/zot-deploy --namespace zot \
        --for=jsonpath='{.status.loadBalancer.ingress[0].ip}'=172.30.255.201 --timeout=5m
    $KUBECTL rollout status statefulset/zot-discovery --namespace zot --timeout=5m
    $KUBECTL rollout status statefulset/zot-deploy --namespace zot --timeout=5m
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

# setup_solar installs the Solar Helm chart into the solar-system namespace, setting component image repositories/tags to the current REGISTRY/TAG. The namespace and its prerequisites (trust label, Zot deploy auth secret) are set up by main beforehand.
setup_solar() {
    echo -e "\nSETTING UP SOLAR:\n"
    local pull_secret_args=()
    if [[ -n "$GHCR_TOKEN" ]]; then
        "${SCRIPT_DIR}/ensure-ghcr-pull-secret.sh" solar-system
        pull_secret_args=(--set 'global.imagePullSecrets[0].name=ghcr-pull-secret')
    fi
    $HELM upgrade --install \
        --namespace=solar-system \
        solar charts/solar \
        -f test/fixtures/solar.values.yaml \
        --set apiserver.image.repository="$REGISTRY/solar-apiserver" \
        --set controller.image.repository="$REGISTRY/solar-controller-manager" \
        --set renderer.image.repository="$REGISTRY/solar-renderer" \
        --set apiserver.image.tag="$TAG" \
        --set controller.image.tag="$TAG" \
        --set renderer.image.tag="$TAG" \
        "${pull_secret_args[@]}"

    # Wait for the aggregated apiserver to be ready before returning. Without
    # this, callers (e.g. the UI e2e tests) can hit solar.opendefense.cloud
    # resources while kube-aggregator has no ready backend endpoints and gets
    # a 503. The APIService Available condition is the authoritative gate:
    # it flips to True only once the aggregator can reach the backing service.
    echo "Waiting for solar apiserver deployment to be available (timeout: 5m)..."
    $KUBECTL wait deployment \
        --namespace solar-system \
        -l app.kubernetes.io/component=apiserver \
        --for condition=Available \
        --timeout 5m
    echo "Waiting for solar APIService to be available (timeout: 5m)..."
    $KUBECTL wait apiservice/v1alpha1.solar.opendefense.cloud \
        --for condition=Available \
        --timeout 5m
}

# setup_discovery installs the solar-discovery Helm chart into the solar-system
# namespace in scan mode, so the local catalog populates reproducibly.
#
# The scan Registry is applied BEFORE the worker is deployed on purpose: the
# discovery pipeline builds one scanner per registry that is present when the
# pod constructs its pipeline (see pkg/discovery/pipeline). A Registry created
# after the worker is already running never gets a scanner, so the catalog would
# stay empty. Scan mode (vs webhook) is used here because it reconciles on an
# interval and is timing independent, which a one-command dev setup needs; the
# webhook path is exercised by the e2e suite instead.
setup_discovery() {
    echo -e "\nSETTING UP SOLAR-DISCOVERY:\n"
    $KUBECTL apply --namespace=solar-system -f test/fixtures/e2e/zot-discovery-auth.yaml
    $KUBECTL apply --namespace=solar-system -f test/fixtures/e2e/zot-discovery-registry-scan.yaml
    local pull_secret_args=()
    if [[ -n "$GHCR_TOKEN" ]]; then
        pull_secret_args=(--set 'imagePullSecrets[0].name=ghcr-pull-secret')
    fi
    $HELM upgrade --install \
        --namespace=solar-system \
        solar-discovery charts/solar-discovery \
        -f test/fixtures/solar-discovery-scan.values.yaml \
        --set image.repository="$REGISTRY/solar-discovery" \
        --set image.tag="$TAG" \
        --set namespace=solar-system \
        "${pull_secret_args[@]}"
    $KUBECTL wait deployment \
        --namespace solar-system \
        -l app.kubernetes.io/instance=solar-discovery \
        --for condition=Available \
        --timeout 5m
}

# main orchestrates cluster setup by invoking cert-manager, trust-manager, Zot components, Flux, and (unless SKIP_SOLAR is "true") Solar, then prints DONE.
main() {
    setup_docker_network
    setup_kind_cluster
    setup_load_balancer
    setup_cert_manager
    setup_trust_manager
    setup_zots
    setup_flux

    if [[ "$SKIP_SOLAR" != "true" ]]; then
        $KUBECTL create namespace solar-system 2>/dev/null || true
        $KUBECTL label namespace solar-system trust=enabled --overwrite
        $KUBECTL apply --namespace=solar-system \
            -f test/fixtures/e2e/zot-deploy-auth.yaml
        setup_solar
    fi
    if [[ "$SKIP_DISCOVERY" != "true" ]]; then
        setup_discovery
    fi

    echo -e "\nDONE"
}

main "$@"
