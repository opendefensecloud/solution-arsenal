#!/usr/bin/env bash

# Fetches the SolAr OpenAPI v3 spec from a running apiserver and writes it to ui/openapi.json.
# Usage:
#   ./fetch-openapi-spec.sh                     # uses current kubectl context
#   ./fetch-openapi-spec.sh --context kind-solar-dev  # uses specific context

set -euo pipefail

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_DIR="$SCRIPT_DIR/.."
OUTPUT_FILE="${PROJECT_DIR}/ui/openapi.json"

KUBECTL="${KUBECTL:-kubectl}"
CONTEXT_FLAG=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --context)
            CONTEXT_FLAG="--context $2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

echo "Fetching OpenAPI v3 spec for solar.opendefense.cloud/v1alpha1..."

SPEC=$($KUBECTL $CONTEXT_FLAG get --raw '/openapi/v3/apis/solar.opendefense.cloud/v1alpha1')

if [ -z "$SPEC" ]; then
    echo "ERROR: Failed to fetch OpenAPI spec. Is the SolAr apiserver running?" >&2
    exit 1
fi

echo "$SPEC" | python3 -m json.tool > "$OUTPUT_FILE"

echo "OpenAPI spec written to ui/openapi.json ($(wc -c < "$OUTPUT_FILE") bytes)"
