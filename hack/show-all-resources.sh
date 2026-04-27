#!/usr/bin/env bash
# Verify TableConverter columns for all Solar API types.
# Each "kubectl get" should show custom columns, not just NAME/AGE.

set -euo pipefail

types=(
  "targets:NAME,RENDER REGISTRY,BOOTSTRAP VERSION,AGE"
  "releases:NAME,COMPONENTVERSION REF,STATUS,AGE"
  "profiles:NAME,RELEASE REF,MATCHED TARGETS,AGE"
  "releasebindings:NAME,TARGET,RELEASE,AGE"
  "registries:NAME,HOSTNAME,PLAIN HTTP,AGE"
  "registrybindings:NAME,TARGET,REGISTRY,AGE"
  "rendertasks:NAME,OWNER KIND,OWNER NAME,STATUS,AGE"
  "components:NAME,REGISTRY,REPOSITORY,AGE"
  "componentversions:NAME,COMPONENT REF,TAG,AGE"
)

for entry in "${types[@]}"; do
  resource="${entry%%:*}"
  expected="${entry#*:}"
  echo "=== $resource (expect: $expected) ==="
  kubectl get "$resource" -A 2>&1 || true
  echo
done
