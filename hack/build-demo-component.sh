#!/usr/bin/env bash
#
# Build the ocm-demo component fixture into a local CTF.
#
# Two steps, both deliberate:
#
#  1. The Helm chart is packaged and pushed into a throwaway OCI registry, and
#     the component references it as an `ociArtifact`. This is the only way to
#     get a chart published as a real tagged artifact later on: OCM v2 derives
#     the `referenceName` that makes a resource publishable from an absolute
#     source reference, and no input method (`helm`, `file`, `dir`, `utf8`) can
#     set one. A chart added with `input: helm` stays a local blob addressable
#     only by digest inside the component's own repository.
#
#  2. The component is then transferred by value into the CTF the suites use.
#     That materialises every resource as a local blob, carrying the
#     referenceName derived from its source reference, so test transfers are
#     offline, and the fixture matches the shape of a real air-gapped CTF.
#
# A later `transfer cv --copy-resources --upload-as ociArtifact` into a registry
# republishes chart and image as tagged artifacts from those referenceNames. The
# values template has no referenceName and stays a local blob, which is correct:
# it is component metadata, not something a cluster pulls.
set -euo pipefail

OCM="${OCM:-ocm}"
HELM="${HELM:-helm}"
DOCKER="${DOCKER:-docker}"
OCM_DEMO_SRC="${OCM_DEMO_SRC:?OCM_DEMO_SRC must point at the component source directory}"
OCM_DEMO_DIR="${OCM_DEMO_DIR:?OCM_DEMO_DIR must point at the CTF to build}"
OCM_DEMO_VERSION="${OCM_DEMO_VERSION:?OCM_DEMO_VERSION must be set}"
OCM_DEMO_COMPONENT="${OCM_DEMO_COMPONENT:-opendefense.cloud/ocm-demo}"
CHART_REGISTRY_IMAGE="${CHART_REGISTRY_IMAGE:-registry:2}"

work="$(mktemp -d)"
registry_name="solar-demo-chart-registry-$$"

cleanup() {
    "$DOCKER" rm -f "$registry_name" >/dev/null 2>&1 || true
    rm -rf "$work"
}
trap cleanup EXIT

port="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')"
chart_registry="127.0.0.1:${port}"

echo "Starting a throwaway chart registry on ${chart_registry}..."
"$DOCKER" run -d --name "$registry_name" -p "${chart_registry}:5000" "$CHART_REGISTRY_IMAGE" >/dev/null

ready=false
for _ in $(seq 1 60); do
    if curl -fsS "http://${chart_registry}/v2/" >/dev/null 2>&1; then
        ready=true
        break
    fi
    sleep 1
done
if [ "$ready" != "true" ]; then
    echo "throwaway chart registry never became ready on ${chart_registry}" >&2
    exit 1
fi

echo "Packaging and pushing the chart..."
"$HELM" package "${OCM_DEMO_SRC}/charts/demo" -d "$work" >/dev/null
"$HELM" push "$work"/demo-*.tgz "oci://${chart_registry}/ocm-demo" --plain-http

echo "Building the component..."
# The helm and file input methods resolve relative paths against the working
# directory rather than the constructor's location, so build from the source
# directory.
(
    cd "$OCM_DEMO_SRC"
    CHART_REGISTRY="http://${chart_registry}" "$OCM" add component-version \
        --repository "ctf::${work}/stage" --constructor component-constructor.yaml
)

echo "Transferring by value into ${OCM_DEMO_DIR}..."
rm -rf "$OCM_DEMO_DIR"
"$OCM" transfer cv --copy-resources \
    "ctf::${work}/stage//${OCM_DEMO_COMPONENT}:${OCM_DEMO_VERSION}" "ctf::${OCM_DEMO_DIR}"
