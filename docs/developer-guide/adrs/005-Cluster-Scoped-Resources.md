---
status: draft
date: 2026-03-19
---

# Cluster-Scoped Resources

## Context and Problem Statement

All SolAr resources are currently **namespaced**:

- Discovery
- Component / ComponentVersion
- Release
- Target
- Profile
- HydratedTarget

We identified two use-cases that might benefit from cluster-scoped resources:

1. **Public Catalog** - Shared components available to all tenants without each tenant maintaining their own Discovery
2. **Mandatory Releases** - Components deployed on every cluster (e.g., Sysdig Agent) with no possible opt-out

Cluster-scoped resources make sense when **modeling infrastructure that exists outside the namespace model** (similar to `ClusterIssuer`, `ClusterSecretStore`, `Node`).

## Considered Solutions

### Use-Case: Public Catalog

| Option | Approach |
|--------|----------|
| **A: Cluster-Scoped** | `ClusterComponent` + `ClusterComponentVersion` (cluster-scoped) |
| **B: Namespaced** | `Component` + `ComponentVersion` in `solar-public` namespace |

#### Option A: ClusterComponent + ClusterComponentVersion

Introduces new resource types:

- `ClusterComponent` - References external OCI Registry
- `ClusterComponentVersion` - Specific version of a ClusterComponent
- Optionally `ClusterDiscovery` - To discover and create ClusterComponents

**Pro:**
- Follows established Kubernetes patterns (ClusterIssuer, ClusterSecretStore)
- Clean cluster-wide visibility
- Enables `ClusterRelease` for Mandatory Releases
- No Cross-Namespace reference complexity
- Release can reference both `ClusterComponentVersion` and `ComponentVersion` (Hybrid possible)

**Con:**
- New resource types to maintain
- Different from tenant resources
- API surface grows

#### Option B: solar-public Namespace

Uses existing resource types in a dedicated namespace:

- Discovery in `solar-public` namespace discovers public components
- Creates Components and ComponentVersions in `solar-public` namespace
- Tenants reference via Cross-Namespace references

**Pro:**
- No new resource types (API unchanged)
- Public and Tenant Components use the same resources

**Con:**
- Cross-Namespace complexity in controller
- RBAC must be configured across namespaces
- No native `ClusterRelease` possible

### Use-Case: Mandatory Releases

| Option | Approach |
|--------|----------|
| **A: Namespaced + Controller** | Controller creates Releases in tenant namespaces |
| **B: Cluster-Scoped + Controller** | `ClusterRelease` referencing `ClusterComponentVersion` |

#### Option A: Namespaced + Controller

- Controller reads ComponentVersions from `solar-public` namespace
- Controller creates Release objects in each tenant namespace
- Controller injects Release references into Targets

**Pro:** No new resource types, transparent releases  
**Con:** Release objects must be protected from tenant deletion

#### Option B: Cluster-Scoped + Controller

- Controller reads ClusterReleases
- Controller watches Targets in tenant namespaces
- Controller injects ClusterRelease references into Targets

**Pro:** Clean, consistent pattern, no Cross-Namespace references  
**Con:** New resource types (ClusterComponentVersion, ClusterRelease)

### Key Finding

`ClusterComponent` and `ClusterRelease` only make sense together - `ClusterRelease` depends on `ClusterComponentVersion` for its reference target.

### Resources NOT Considered for Cluster-Scoped Variants

| Resource | Reason |
|----------|--------|
| Discovery | Tenant owns their registry discovery configuration |
| Target | Tenant registers their own clusters - no sharing across tenants |
| Profile | Tenant-specific release configurations |
| HydratedTarget | Derived from namespaced resources |
| RenderTask | Transient per-tenant operation |

## Decision Outcome

**Status: Deferred - Follow-up Spike Required**

Before making a final decision on whether to introduce cluster-scoped resources, we will investigate **ReferenceGrants** as an alternative approach for safe cross-namespace references.

During discussion, [ReferenceGrants](https://gateway-api.sigs.k8s.io/api-types/referencegrant/) (from the Gateway API project) were identified as a potential mechanism to enable secure cross-namespace references without requiring cluster-scoped resources.

**Next Steps:**
1. Open a follow-up spike to evaluate ReferenceGrants
2. Determine if ReferenceGrants enable a pure namespaced approach for both use-cases
3. Re-evaluate cluster-scoped options based on ReferenceGrants findings
