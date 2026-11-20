#!/usr/bin/env bash
# Publishes one of the multi-helm fixtures to the dev cluster's discovery
# registry: pushes its charts, then builds and transfers the component.
#
#   publish.sh umbrella|chart-set [version]
#   UMBRELLA_CRDS_DIR=1 publish.sh umbrella [version]
#
# Run from the repository root, after `make dev-cluster`. The component version
# defaults to 0.1.0 and is the version of every chart as well.
set -euo pipefail

fixture="${1:?usage: publish.sh umbrella|chart-set [version]}"
export VERSION="${2:-0.1.0}"

KUBECTL="${KUBECTL:-kubectl} --context kind-${KIND_CLUSTER_DEV:-solar-dev}"
OCM="${OCM:-bin/go/ocm2}"
HELM="${HELM:-helm}"
LOCAL_PORT="${LOCAL_PORT:-4443}"
here="test/fixtures/multi-helm"
# hack/dev-cluster.sh writes the cluster CA here. Set unconditionally: a nix
# shell presets both variables to its own bundle.
SSL_CERT_FILE="$(pwd)/test/fixtures/ca.crt"
export SSL_CERT_FILE
export NIX_SSL_CERT_FILE="$SSL_CERT_FILE"
# The charts are pushed under src/ and referenced from there; the transfer
# republishes them under test/, next to the component, which is what Solar reads.
export CHART_REGISTRY="localhost:${LOCAL_PORT}/src"

work="$(mktemp -d)"
$KUBECTL -n zot port-forward svc/zot-discovery "${LOCAL_PORT}:443" > /dev/null &
pf_pid=$!
trap 'kill "$pf_pid" 2>/dev/null || true; rm -rf "$work"' EXIT
for _ in $(seq 1 30); do
    if (exec 3<> "/dev/tcp/localhost/${LOCAL_PORT}") 2> /dev/null; then break; fi
    sleep 1
done

push() { # push <chart dir> <version>
    "$HELM" package "$1" --destination "$work" --version "$2" --app-version "$2" > /dev/null
    "$HELM" push "$work/$(basename "$1")"-*.tgz "oci://${CHART_REGISTRY}/charts" \
        --username admin --password admin --ca-file "$SSL_CERT_FILE"
    rm -f "$work"/*.tgz
}

case "$fixture" in
    umbrella)
        "$HELM" dependency build "$here/umbrella" > /dev/null
        # A chart's name is its directory's on push; the umbrella chart is "widgets".
        cp -r "$here/umbrella" "$work/widgets"
        # Variant: the CRD in the umbrella's crds/ directory, which Helm installs
        # before any template, and switched off in the sub-chart.
        if [ "${UMBRELLA_CRDS_DIR:-0}" = 1 ]; then
            mkdir "$work/widgets/crds"
            "$HELM" template crd "$here/charts/widget-operator" -s templates/crd.yaml > "$work/widgets/crds/widget.yaml"
            printf 'widget-operator:\n  crds:\n    enabled: false\n' >> "$work/widgets/values.yaml"
        fi
        push "$work/widgets" "$VERSION"
        ;;
    chart-set)
        push "$here/charts/widget-operator" "$VERSION"
        push "$here/charts/widget-instance" "$VERSION"
        # The hand-written release chart, pointed at the charts just pushed.
        cp -r "$here/chart-set/release" "$work/release"
        sed -i "s/^    tag: .*/    tag: $VERSION/" "$work/release/values.yaml"
        push "$work/release" "$VERSION"
        rm -rf "$work/release"
        ;;
    *)
        echo "unknown fixture $fixture" >&2
        exit 2
        ;;
esac

"$OCM" --config test/fixtures/e2e/ocmconfig add cv \
    --repository "ctf::$work/ctf" --constructor "$here/$fixture/component-constructor.yaml"
"$OCM" --config test/fixtures/ocm-uploaders.yaml --config test/fixtures/e2e/ocmconfig transfer cv \
    "ctf::$work/ctf//opendefense.cloud/multi-helm-${fixture}:${VERSION}" "https://localhost:${LOCAL_PORT}/test"
