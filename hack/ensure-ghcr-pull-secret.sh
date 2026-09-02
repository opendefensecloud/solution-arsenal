#!/usr/bin/env bash

set -euo pipefail

kc=${GHCR_KUBECTL:-${KUBECTL:-kubectl}}
GHCR_TOKEN=${GHCR_TOKEN:-}

main() {
    local namespace="$1"
    [[ -n "$GHCR_TOKEN" ]] || return 0
    "$kc" create secret docker-registry ghcr-pull-secret \
        --namespace "$namespace" \
        --docker-server=ghcr.io \
        --docker-username=x-access-token \
        --docker-password="$GHCR_TOKEN" \
        --dry-run=client -o yaml | "$kc" apply -f -
}

main "$@"
