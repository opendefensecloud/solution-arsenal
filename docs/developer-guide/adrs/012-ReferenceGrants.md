---
status: accepted
date: 2026-05-19
---

# Cross-Namespace References via ReferenceGrants

## Context and Problem Statement

[ADR-005](./005-Cluster-Scoped-Resources.md) deferred a decision on cluster-scoped resources and identified ReferenceGrants as a mechanism worth evaluating to enable safe cross-namespace references without introducing new cluster-scoped types.

All SolAr resources are namespace-scoped. A multi-stakeholder deployment naturally splits resources across namespaces that correspond to roles (see [Roles](../roles.md)):

- **App Catalog Maintainer** namespace — `Component`, `ComponentVersion`
- **K8s Cluster Provider** namespace — `Release`, `Profile`, `Registry`, `RegistryBinding`, `ReleaseBinding`
- **K8s Cluster User** namespace — `Target`

This creates four cross-namespace reference needs:

1. A `Release` (provider or user namespace) references a `ComponentVersion` in the app-catalog-maintainer namespace.
2. A `Target` (user namespace) references a `Registry` in the provider namespace.
3. A `Profile` (provider namespace) matches `Target` resources in the user namespace, creating cross-namespace `ReleaseBinding` resources.
4. A `RegistryBinding` (provider namespace) references a `Target` in the user namespace.

## Considered Options

- **Cluster-scoped resources** — `ClusterComponent`, `ClusterComponentVersion`, `ClusterRelease` (ADR-005 Option A)
- **Dedicated shared namespace** — `solar-public` namespace for shared catalog resources (ADR-005 Option B)
- **ReferenceGrants** — a CRD that explicitly declares authorization for cross-namespace references, adapted from the [Kubernetes Gateway API pattern](https://gateway-api.sigs.k8s.io/api-types/referencegrant/)

## Decision Outcome

Chosen option: **ReferenceGrants**

A `ReferenceGrant` lives in the namespace that **owns the referenced resource**. It explicitly lists which resource kinds from which namespaces are permitted to reference resources in its namespace. Controllers validate the grant before following any cross-namespace reference; if no grant exists, the reference is refused and a `NotGranted` status condition is set.

### Why ReferenceGrants over the alternatives

- No new cluster-scoped resource types needed — the API surface stays manageable.
- Authorization is explicit and auditable: the receiving namespace always controls who can reference its resources.
- All existing namespaced resources work unchanged; cross-namespace access is opt-in per namespace.
- The pattern is well-established in the Kubernetes ecosystem (Gateway API uses the same model).
- Cluster-scoped types (`ClusterRelease`, `ClusterComponent`) would only make sense together and add permanent maintenance burden. ReferenceGrants achieve the same outcomes with no new types.

## Implementation Patterns

### Pattern 1: Release → ComponentVersion

A `Release` in the provider or user namespace references a `ComponentVersion` in the app-catalog-maintainer namespace.

**ReferenceGrant** (in `app-catalog-maintainer` namespace):

```yaml
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
```

**Release** (in `k8s-cluster-provider` namespace):

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Release
metadata:
  name: my-app
  namespace: k8s-cluster-provider
spec:
  componentVersionRef:
    name: my-app-v1.2.3
  componentVersionNamespace: app-catalog-maintainer
  # ...
```

### Pattern 2: Target → Registry

A `Target` (user namespace) references a `Registry` in the provider namespace.

**ReferenceGrant** (in `k8s-cluster-provider` namespace):

```yaml
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
```

**Target** (in `k8s-cluster-user` namespace):

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Target
metadata:
  name: my-cluster
  namespace: k8s-cluster-user
spec:
  renderRegistryRef:
    name: harbor-edge
  renderRegistryNamespace: k8s-cluster-provider
  # ...
```

### Pattern 3: Profile → Target (cross-namespace ReleaseBindings)

A `Profile` (provider namespace) matches `Target` resources in the user namespace. The Profile controller creates `ReleaseBinding` resources **in the Profile's namespace** with `spec.targetNamespace` pointing to the Target's namespace. The Target controller then discovers these cross-namespace bindings using the same grant.

**ReferenceGrant** (in `k8s-cluster-user` namespace):

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReferenceGrant
metadata:
  name: allow-provider-target-access
  namespace: k8s-cluster-user
spec:
  from:
  - group: solar.opendefense.cloud
    kind: Profile
    namespace: k8s-cluster-provider
  - group: solar.opendefense.cloud
    kind: ReleaseBinding
    namespace: k8s-cluster-provider
  - group: solar.opendefense.cloud
    kind: RegistryBinding
    namespace: k8s-cluster-provider
  to:
  - group: solar.opendefense.cloud
    kind: Target
```

**Profile** (in `k8s-cluster-provider` namespace):

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Profile
metadata:
  name: fleet-rollout
  namespace: k8s-cluster-provider
spec:
  releaseRef:
    name: my-app
  targetSelector:
    matchLabels:
      env: prod
```

The Profile controller discovers Targets in the user namespace via the grant, then creates a `ReleaseBinding` in the provider namespace:

```yaml
# Created automatically by the Profile controller
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReleaseBinding
metadata:
  name: fleet-rollout-my-cluster-<hash>
  namespace: k8s-cluster-provider        # lives in Profile's namespace
spec:
  releaseRef:
    name: my-app
  targetRef:
    name: my-cluster
  targetNamespace: k8s-cluster-user      # Target lives in user namespace
```

The Target controller reads the same grant and collects cross-namespace `ReleaseBinding` resources when reconciling a Target.

### Pattern 4: RegistryBinding → Target

A `RegistryBinding` (provider namespace) references a `Target` in the user namespace to declare which Registry the Target is allowed to use as a source. The `RegistryBinding → Target` cross-namespace reference is authorized by the same grant as Pattern 3 (the `RegistryBinding` entry in the `from` list).

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: RegistryBinding
metadata:
  name: harbor-edge-binding
  namespace: k8s-cluster-provider
spec:
  targetRef:
    name: my-cluster
  targetNamespace: k8s-cluster-user     # Target lives in user namespace
  registryRef:
    name: harbor-edge
```

## Consequences

**Positive:**
- No cluster-scoped resource types needed for any of the identified use-cases.
- Receiving namespace retains full control over who can reference its resources.
- Controllers fail closed: a missing grant causes a `NotGranted` condition, not a silent access bypass.
- Grant listing is scoped to the relevant namespace (not cluster-wide), keeping the performance impact minimal.

**Negative:**
- Each cross-namespace pattern requires a `ReferenceGrant` to be provisioned in the correct namespace. Operators must understand which namespace owns the referenced resource.
- Controllers must perform a grant check on every reconcile for cross-namespace references.

## Relationship to ADR-005

ADR-005 deferred the cluster-scoped resources decision pending this investigation. ReferenceGrants solve all identified use-cases without cluster-scoped types; ADR-005's Option B (namespaced resources in a shared namespace) is not needed either. Cluster-scoped resource types are not introduced.
