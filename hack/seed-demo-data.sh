#!/usr/bin/env bash

set -euo pipefail

KUBECTL="${KUBECTL:-kubectl}"
NAMESPACE="${NAMESPACE:-default}"

echo "Seeding demo data into namespace '$NAMESPACE'..."

$KUBECTL apply -n "$NAMESPACE" -f - <<'EOF'
---
# Components
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Component
metadata:
  name: podinfo
spec:
  scheme: ociRegistry
  registry: ghcr.io
  repository: stefanprodan/podinfo
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Component
metadata:
  name: nginx
spec:
  scheme: ociRegistry
  registry: registry-1.docker.io
  repository: bitnamicharts/nginx
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Component
metadata:
  name: redis
spec:
  scheme: ociRegistry
  registry: registry-1.docker.io
  repository: bitnamicharts/redis
---
# Component Versions
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ComponentVersion
metadata:
  name: podinfo-v6.7.1
spec:
  componentRef:
    name: podinfo
  tag: "6.7.1"
  resources:
    chart:
      repository: stefanprodan/charts/podinfo
      tag: "6.7.1"
  entrypoint:
    resourceName: chart
    type: helm
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ComponentVersion
metadata:
  name: podinfo-v6.6.0
spec:
  componentRef:
    name: podinfo
  tag: "6.6.0"
  resources:
    chart:
      repository: stefanprodan/charts/podinfo
      tag: "6.6.0"
  entrypoint:
    resourceName: chart
    type: helm
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ComponentVersion
metadata:
  name: nginx-v18.3.1
spec:
  componentRef:
    name: nginx
  tag: "18.3.1"
  resources:
    chart:
      repository: bitnamicharts/nginx
      tag: "18.3.1"
  entrypoint:
    resourceName: chart
    type: helm
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ComponentVersion
metadata:
  name: redis-v20.6.2
spec:
  componentRef:
    name: redis
  tag: "20.6.2"
  resources:
    chart:
      repository: bitnamicharts/redis
      tag: "20.6.2"
  entrypoint:
    resourceName: chart
    type: helm
---
# Registries
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Registry
metadata:
  name: edge-registry
spec:
  hostname: registry.edge.example.com:5000
  plainHTTP: true
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Registry
metadata:
  name: central-registry
spec:
  hostname: registry.central.example.com
---
# Releases
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Release
metadata:
  name: podinfo-stable
spec:
  componentVersionRef:
    name: podinfo-v6.7.1
  values:
    raw: |
      {"replicaCount": 2, "ui": {"message": "Hello from SolAr"}}
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Release
metadata:
  name: podinfo-canary
spec:
  componentVersionRef:
    name: podinfo-v6.6.0
  values:
    raw: |
      {"replicaCount": 1, "ui": {"message": "Canary release"}}
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Release
metadata:
  name: nginx-prod
spec:
  componentVersionRef:
    name: nginx-v18.3.1
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Release
metadata:
  name: redis-cache
spec:
  componentVersionRef:
    name: redis-v20.6.2
  values:
    raw: |
      {"architecture": "standalone", "auth": {"enabled": false}}
---
# Targets
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Target
metadata:
  name: edge-berlin-01
  labels:
    region: europe
    site: berlin
    env: production
    tier: edge
spec:
  renderRegistryRef:
    name: edge-registry
  userdata:
    raw: |
      {"location": "Berlin, DE", "capacity": "medium", "contact": "ops-berlin@example.com"}
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Target
metadata:
  name: edge-munich-01
  labels:
    region: europe
    site: munich
    env: production
    tier: edge
spec:
  renderRegistryRef:
    name: edge-registry
  userdata:
    raw: |
      {"location": "Munich, DE", "capacity": "large", "contact": "ops-munich@example.com"}
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Target
metadata:
  name: edge-paris-01
  labels:
    region: europe
    site: paris
    env: staging
    tier: edge
spec:
  renderRegistryRef:
    name: edge-registry
  userdata:
    raw: |
      {"location": "Paris, FR", "capacity": "small", "contact": "ops-paris@example.com"}
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Target
metadata:
  name: central-hq
  labels:
    region: europe
    site: hq
    env: production
    tier: central
spec:
  renderRegistryRef:
    name: central-registry
  userdata:
    raw: |
      {"location": "Frankfurt, DE", "capacity": "xlarge", "contact": "ops-hq@example.com"}
---
# Registry Bindings
apiVersion: solar.opendefense.cloud/v1alpha1
kind: RegistryBinding
metadata:
  name: edge-berlin-01-registry
spec:
  targetRef:
    name: edge-berlin-01
  registryRef:
    name: edge-registry
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: RegistryBinding
metadata:
  name: edge-munich-01-registry
spec:
  targetRef:
    name: edge-munich-01
  registryRef:
    name: edge-registry
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: RegistryBinding
metadata:
  name: central-hq-registry
spec:
  targetRef:
    name: central-hq
  registryRef:
    name: central-registry
---
# Release Bindings
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReleaseBinding
metadata:
  name: edge-berlin-01-podinfo
spec:
  targetRef:
    name: edge-berlin-01
  releaseRef:
    name: podinfo-stable
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReleaseBinding
metadata:
  name: edge-berlin-01-nginx
spec:
  targetRef:
    name: edge-berlin-01
  releaseRef:
    name: nginx-prod
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReleaseBinding
metadata:
  name: edge-munich-01-podinfo
spec:
  targetRef:
    name: edge-munich-01
  releaseRef:
    name: podinfo-stable
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReleaseBinding
metadata:
  name: edge-munich-01-redis
spec:
  targetRef:
    name: edge-munich-01
  releaseRef:
    name: redis-cache
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReleaseBinding
metadata:
  name: edge-paris-01-podinfo
spec:
  targetRef:
    name: edge-paris-01
  releaseRef:
    name: podinfo-canary
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReleaseBinding
metadata:
  name: central-hq-podinfo
spec:
  targetRef:
    name: central-hq
  releaseRef:
    name: podinfo-stable
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReleaseBinding
metadata:
  name: central-hq-nginx
spec:
  targetRef:
    name: central-hq
  releaseRef:
    name: nginx-prod
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReleaseBinding
metadata:
  name: central-hq-redis
spec:
  targetRef:
    name: central-hq
  releaseRef:
    name: redis-cache
---
# Profiles
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Profile
metadata:
  name: edge-baseline
spec:
  releaseRef:
    name: podinfo-stable
  targetSelector:
    matchLabels:
      tier: edge
      env: production
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Profile
metadata:
  name: full-stack
spec:
  releaseRef:
    name: nginx-prod
  targetSelector:
    matchLabels:
      tier: central
EOF

echo "Demo data seeded successfully."
echo ""
echo "Resources created:"
echo "  - 3 Components (podinfo, nginx, redis)"
echo "  - 4 ComponentVersions"
echo "  - 2 Registries (edge, central)"
echo "  - 4 Releases (podinfo-stable, podinfo-canary, nginx-prod, redis-cache)"
echo "  - 4 Targets (edge-berlin-01, edge-munich-01, edge-paris-01, central-hq)"
echo "  - 3 RegistryBindings"
echo "  - 8 ReleaseBindings"
echo "  - 2 Profiles (edge-baseline, full-stack)"
