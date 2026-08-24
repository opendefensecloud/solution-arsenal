#!/usr/bin/env bash
#
# Registers the remote Zitadel as a JWT issuer in the UI dev cluster's API
# server, so it accepts SolAr UI id_tokens directly (--auth-mode=token).
# Only needed for token mode.

set -euo pipefail

KUBECTL="${KUBECTL:-kubectl}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
WORK_DIR="${WORK_DIR:-$PROJECT_DIR/tmp/ui-dev}"

AUTH_CONFIG="$WORK_DIR/dex-auth-config.yaml"
AUTH_CONFIG_BASE="$WORK_DIR/dex-auth-config.base.yaml"

ISSUER="${ZITADEL_ISSUER:?ZITADEL_ISSUER is required}"
CLIENT_ID="${ZITADEL_CLIENT_ID:?ZITADEL_CLIENT_ID is required}"

# Guard against reconfiguring authentication on a non-local cluster.
CURRENT_CONTEXT="$($KUBECTL config current-context 2>/dev/null || true)"
EXPECTED_CONTEXT="${KIND_CLUSTER:+kind-${KIND_CLUSTER}}"
if [[ "${ALLOW_NON_LOCAL_CLUSTER:-false}" != "true" ]]; then
    if [[ -n "$EXPECTED_CONTEXT" && "$CURRENT_CONTEXT" != "$EXPECTED_CONTEXT" ]]; then
        echo "Refusing to run against context '${CURRENT_CONTEXT:-<none>}' (expected '$EXPECTED_CONTEXT')." >&2
        echo "Switch context or set ALLOW_NON_LOCAL_CLUSTER=true to override intentionally." >&2
        exit 1
    fi
    if [[ -z "$EXPECTED_CONTEXT" && ! "$CURRENT_CONTEXT" =~ ^kind- ]]; then
        echo "Refusing to reconfigure authentication on non-kind context: ${CURRENT_CONTEXT:-<none>}" >&2
        echo "Set ALLOW_NON_LOCAL_CLUSTER=true to override intentionally." >&2
        exit 1
    fi
fi

[[ -f "$AUTH_CONFIG" ]] || { echo "Missing $AUTH_CONFIG — run 'make ui-dev-cluster' first." >&2; exit 1; }

# Rebuilt from a Dex-only base, so a changed issuer or client ID can't leave a
# stale entry behind. The base must not already contain our block.
if [[ ! -f "$AUTH_CONFIG_BASE" ]]; then
    if [[ "$(grep -c '^  - issuer:' "$AUTH_CONFIG")" -ne 1 ]]; then
        echo "$AUTH_CONFIG has extra issuers and no $AUTH_CONFIG_BASE to rebuild from." >&2
        echo "Regenerate it with hack/generate-dex-certs.sh." >&2
        exit 1
    fi
    cp "$AUTH_CONFIG" "$AUTH_CONFIG_BASE"
fi

# No groups mapping: Zitadel's roles claim is a map, which the API server can't
# turn into group names. RBAC binds on the username.
DESIRED="$(printf '%s\n' "$(<"$AUTH_CONFIG_BASE")" "  - issuer:
      url: $ISSUER
      audiences:
        - \"$CLIENT_ID\"
    claimMappings:
      username:
        claim: email
        prefix: \"\"")"

if [[ "$(cat "$AUTH_CONFIG")" == "$DESIRED" ]]; then
    echo "$ISSUER already registered in $AUTH_CONFIG."
    exit 0
fi

echo "Registering $ISSUER in $AUTH_CONFIG..."
# Taken before the write: a reload logged earlier isn't evidence about ours.
SINCE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf '%s\n' "$DESIRED" > "$AUTH_CONFIG"

echo "Waiting for the API server to reload it..."
for _ in $(seq 1 45); do
    sleep 2
    LOG="$($KUBECTL logs -n kube-system -l component=kube-apiserver --since-time="$SINCE" 2>/dev/null || true)"

    ERRORS="$(grep -E "failed to (load|validate|update) authentication config" <<<"$LOG" || true)"
    MINE="$(grep -F "\"$ISSUER\"" <<<"$ERRORS" || true)"
    UNATTRIBUTED="$(grep -vF 'issuer "' <<<"$ERRORS" || true)"
    if [[ -n "$MINE$UNATTRIBUTED" ]]; then
        echo "The API server rejected the authentication config:" >&2
        printf '%s\n' "$MINE$UNATTRIBUTED" | tail -1 >&2
        exit 1
    fi

    if grep -q "reloaded authentication config" <<<"$LOG"; then
        echo "API server reloaded."
        exit 0
    fi
done

# Changed on disk but identical to what's loaded — e.g. reverting a config the
# API server refused. Nothing to reload.
echo "No API-server reload was observed after changing $AUTH_CONFIG." >&2
exit 1
