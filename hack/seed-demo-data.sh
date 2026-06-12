#!/usr/bin/env bash
#
# Seed demo SolAr resources across the three role namespaces defined in
# docs/developer-guide/roles.md so impersonation can be demonstrated:
#
#   app-catalog-maintainer  → Components, ComponentVersions
#   k8s-cluster-provider    → Registries, Releases, Profiles, ReleaseBindings, RegistryBindings
#   k8s-cluster-user        → Targets
#
# Cross-namespace references (Release → ComponentVersion, ReleaseBinding →
# Target, Target → Registry) are permitted by the ReferenceGrants installed
# alongside the RBAC in test/fixtures/e2e/dex/dex-rbac.yaml.

set -euo pipefail

KUBECTL="${KUBECTL:-kubectl}"

ACM_NS=app-catalog-maintainer
KCP_NS=k8s-cluster-provider
KCU_NS=k8s-cluster-user

echo "Seeding demo data into namespaces $ACM_NS, $KCP_NS, $KCU_NS..."

# Namespaces are normally created by dex-rbac.yaml during ui-dev-cluster
# setup; create here too so the seeder is safe to run standalone.
$KUBECTL apply -f - <<EOF
---
apiVersion: v1
kind: Namespace
metadata:
  name: $ACM_NS
---
apiVersion: v1
kind: Namespace
metadata:
  name: $KCP_NS
---
apiVersion: v1
kind: Namespace
metadata:
  name: $KCU_NS
EOF

# ----- ReferenceGrants -------------------------------------------------------
# These are served by the aggregated solar-apiserver, so they can only be
# applied once solar is up (which is why they live here and not in the Dex
# RBAC fixture). They permit the cross-namespace references used by the
# Release / ReleaseBinding / Target / Registry resources seeded below.
$KUBECTL apply -f - <<'EOF'
---
# Allow Releases in KCP/KCU to reference ComponentVersions in the catalog.
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReferenceGrant
metadata:
  name: allow-release-cv-access
  namespace: app-catalog-maintainer
spec:
  from:
    - group: solar.opendefense.cloud
      kind: Release
      namespace: k8s-cluster-provider
    - group: solar.opendefense.cloud
      kind: Release
      namespace: k8s-cluster-user
  to:
    - group: solar.opendefense.cloud
      kind: ComponentVersion
---
# Allow Targets in KCU to reference Registries in KCP.
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReferenceGrant
metadata:
  name: allow-target-registry-access
  namespace: k8s-cluster-provider
spec:
  from:
    - group: solar.opendefense.cloud
      kind: Target
      namespace: k8s-cluster-user
  to:
    - group: solar.opendefense.cloud
      kind: Registry
---
# Allow ReleaseBindings/RegistryBindings/Profiles in KCP to reference
# Targets in KCU.
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReferenceGrant
metadata:
  name: allow-provider-target-access
  namespace: k8s-cluster-user
spec:
  from:
    - group: solar.opendefense.cloud
      kind: ReleaseBinding
      namespace: k8s-cluster-provider
    - group: solar.opendefense.cloud
      kind: RegistryBinding
      namespace: k8s-cluster-provider
    - group: solar.opendefense.cloud
      kind: Profile
      namespace: k8s-cluster-provider
  to:
    - group: solar.opendefense.cloud
      kind: Target
EOF

# ----- App Catalog Maintainer namespace --------------------------------------
$KUBECTL apply -n "$ACM_NS" -f - <<'EOF'
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
EOF

# ----- K8s Cluster User namespace --------------------------------------------
# Targets live with the cluster user but are managed by the cluster provider.
$KUBECTL apply -n "$KCU_NS" -f - <<'EOF'
---
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
  renderRegistryNamespace: k8s-cluster-provider
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
  renderRegistryNamespace: k8s-cluster-provider
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
  renderRegistryNamespace: k8s-cluster-provider
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
  renderRegistryNamespace: k8s-cluster-provider
  userdata:
    raw: |
      {"location": "Frankfurt, DE", "capacity": "xlarge", "contact": "ops-hq@example.com"}
EOF

# ----- K8s Cluster Provider namespace ----------------------------------------
$KUBECTL apply -n "$KCP_NS" -f - <<'EOF'
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
# Releases — reference ComponentVersions in the App Catalog Maintainer ns
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Release
metadata:
  name: podinfo-stable
spec:
  componentVersionRef:
    name: podinfo-v6.7.1
  componentVersionNamespace: app-catalog-maintainer
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
  componentVersionNamespace: app-catalog-maintainer
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
  componentVersionNamespace: app-catalog-maintainer
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Release
metadata:
  name: redis-cache
spec:
  componentVersionRef:
    name: redis-v20.6.2
  componentVersionNamespace: app-catalog-maintainer
  values:
    raw: |
      {"architecture": "standalone", "auth": {"enabled": false}}
---
# Registry Bindings — bind Targets (in KCU) to Registries (here)
apiVersion: solar.opendefense.cloud/v1alpha1
kind: RegistryBinding
metadata:
  name: edge-berlin-01-registry
spec:
  targetRef:
    name: edge-berlin-01
  targetNamespace: k8s-cluster-user
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
  targetNamespace: k8s-cluster-user
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
  targetNamespace: k8s-cluster-user
  registryRef:
    name: central-registry
---
# Release Bindings — bind Releases (here) to Targets (in KCU)
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReleaseBinding
metadata:
  name: edge-berlin-01-podinfo
spec:
  targetRef:
    name: edge-berlin-01
  targetNamespace: k8s-cluster-user
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
  targetNamespace: k8s-cluster-user
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
  targetNamespace: k8s-cluster-user
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
  targetNamespace: k8s-cluster-user
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
  targetNamespace: k8s-cluster-user
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
  targetNamespace: k8s-cluster-user
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
  targetNamespace: k8s-cluster-user
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
  targetNamespace: k8s-cluster-user
  releaseRef:
    name: redis-cache
---
# Profiles — select Targets (in KCU) by label
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
echo "Resources by namespace:"
echo "  $ACM_NS:"
echo "    - 3 Components (podinfo, nginx, redis)"
echo "    - 4 ComponentVersions"
echo "  $KCP_NS:"
echo "    - 2 Registries (edge, central)"
echo "    - 4 Releases (podinfo-stable, podinfo-canary, nginx-prod, redis-cache)"
echo "    - 3 RegistryBindings"
echo "    - 8 ReleaseBindings"
echo "    - 2 Profiles (edge-baseline, full-stack)"
echo "  $KCU_NS:"
echo "    - 4 Targets (edge-berlin-01, edge-munich-01, edge-paris-01, central-hq)"
