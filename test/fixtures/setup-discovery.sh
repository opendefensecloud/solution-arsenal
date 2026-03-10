#!/usr/bin/env bash
set -euo pipefail

KIND_CLUSTER_DEV="${KIND_CLUSTER_DEV:-solar-dev}"
KUBECTL="${KUBECTL:-kubectl} --context kind-${KIND_CLUSTER_DEV}"
OCM="${OCM:-ocm}"
HELMDEMO_DIR="${HELMDEMO_DIR:-$(pwd)/test/fixtures/helmdemo-ctf}"

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
