#!/usr/bin/env bash
# Unit test for the catalog-chaining cv-list builder.
#
# The script under test is extracted from the ConfigMap in the workflow
# template, so the test always runs the exact bytes the workflow runs and
# cannot drift from it.
#
# Usage: bash test/fixtures/chaining/cv-list-test.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${REPO_ROOT:-$(cd "${SCRIPT_DIR}/../../.." && pwd)}"
TESTDATA="${SCRIPT_DIR}/testdata"
TEMPLATE="${REPO_ROOT}/assets/workflows/chaining-cluster-workflow-template.yaml"
YQ="${YQ:-yq}"

if ! "${YQ}" --version 2>&1 | grep -q mikefarah; then
    echo "FAIL: needs mikefarah yq (yq-go); found: $("${YQ}" --version 2>&1 | head -1)" >&2
    echo "      run inside 'nix develop', or set YQ=/path/to/yq-go" >&2
    exit 1
fi

WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT

"${YQ}" -o=json '.' "${TEMPLATE}" \
  | jq -r 'select(.kind == "ConfigMap" and .metadata.name == "solar-catalog-transfer-scripts")
           | .data["build-cv-list.sh"]' \
  > "${WORK}/build-cv-list.sh"

[ -s "${WORK}/build-cv-list.sh" ] || { echo "FAIL: could not extract build-cv-list.sh"; exit 1; }

fails=0

# as_json <fixture.yaml> -- converts a YAML fixture to JSON and prints the path.
as_json() {
    local src="$1"
    local out
    out="${WORK}/$(basename "${src%.yaml}").json"
    "${YQ}" -o=json '.' "${src}" > "${out}"
    printf '%s' "${out}"
}

# run_case <name> <expected.yaml> <input.yaml...>
# Reads FALLBACK_SRC_SECRET from the environment.
run_case() {
    local name="$1" expected="$2"
    shift 2

    local inputs=()
    local f
    for f in "$@"; do
        inputs+=("$(as_json "${TESTDATA}/${f}")")
    done

    local got="${WORK}/${name}.actual.json"
    if ! FALLBACK_SRC_SECRET="${FALLBACK_SRC_SECRET:-}" \
         SRC_SECRET_OVERRIDES="${SRC_SECRET_OVERRIDES:-}" \
         bash "${WORK}/build-cv-list.sh" "${inputs[@]}" \
         > "${got}" 2>"${WORK}/${name}.stderr"; then
        echo "FAIL ${name}: script exited non-zero"
        cat "${WORK}/${name}.stderr"
        fails=$((fails + 1))
        return
    fi

    if diff <("${YQ}" -o=json '.' "${TESTDATA}/${expected}" | jq -S .) <(jq -S . "${got}") \
            > "${WORK}/${name}.diff"; then
        echo "ok   ${name}"
    else
        echo "FAIL ${name}: output differs from ${expected}"
        cat "${WORK}/${name}.diff"
        fails=$((fails + 1))
    fi
}

OVERRIDES='{"10.96.200.10:443": "src-a-pull", "10.96.200.11:443": "src-b-pull"}'

# Each registry resolves its own Secret from srcSecrets; the third has no entry
# and no fallback, so it goes anonymous.
SRC_SECRET_OVERRIDES="${OVERRIDES}" FALLBACK_SRC_SECRET="" \
    run_case "per-registry-secrets" "expected-per-registry-secrets.yaml" \
    components.yaml componentversions.yaml dst-empty.yaml

SRC_SECRET_OVERRIDES='{}' \
FALLBACK_SRC_SECRET="global-fallback" \
    run_case "fallback-secret" "expected-fallback-secret.yaml" \
    components.yaml componentversions.yaml dst-empty.yaml

# Already in the destination with matching content -> skipped.
SRC_SECRET_OVERRIDES="${OVERRIDES}" FALLBACK_SRC_SECRET="" \
    run_case "dedup" "expected-dedup.yaml" \
    components.yaml componentversions.yaml dst-componentversions.yaml

# Same name and version in the destination, but different resource digests: the
# source tag was repointed. Must be re-transferred, not skipped.
SRC_SECRET_OVERRIDES="${OVERRIDES}" FALLBACK_SRC_SECRET="" \
    run_case "content-changed" "expected-content-changed.yaml" \
    components.yaml componentversions.yaml dst-content-changed.yaml

if [ "${fails}" -ne 0 ]; then
    echo "${fails} case(s) failed"
    exit 1
fi
echo "all cases passed"
