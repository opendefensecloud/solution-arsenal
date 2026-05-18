---
status: draft
date: 2026-05-11
---

# Component Dependencies 

## Context and Problem Statement

SolAr is designed to run on air-gapped edge clusters. An edge cluster operating inside a security boundary cannot reach
external OCI registries at runtime. All artifacts required for a deployment must therefore be pre-staged in a locally
reachable registry.

OCM component descriptors already express inter-component dependencies via `ComponentReferences` in the component
descriptor. A component such as `opendefense.cloud/my-platform-service` may declare that it needs
`opendefense.cloud/cert-manager` at `>= 1.15`. Today Solar's discovery pipeline reads the descriptor
(`compdesc.ComponentSpec` is already carried in `WriteAPIResourceEvent`) but discards the `References` slice entirely.

This means Solar has no model for inter-component dependencies. Consequences:

1. An operator deploying a `Release` has no way to know from the Solar API which other components must also be present on the target.
2. When Flux/the agent tries to reconcile the desired state image, it may pull manifests
   that reference container images living in an unreachable external registry, causing image-pull errors at
   deploy time rather than a clear pre-flight failure. Without an explicit dependency model Solar cannot validate completeness before triggering a render.

The gap between what OCM models (a directed acyclic graph of components) and what Solar models (isolated
`ComponentVersions`) must be closed.

## Decision Drivers

- All artifacts required for a Release must be available in the air-gapped registry before deployment.
- The Solar catalog must represent the full closure of what needs to be deployed, not just the top-level
  component.
- All dependency constraints are resolved at catalog time via Solar's lock-file mechanism: a dedicated lock
  resolver evaluates each SemVer constraint against locally available `ComponentVersions` and writes the result into
  `componentVersionRef`. Rendering always consumes the locked reference never an unresolved constraint and never
  an external registry call (see Concern #2).
- The solution must remain compatible with ADR-008 (no auth handling by Solar), ADR-009 (Registry/RegistryBinding
  model), and ADR-005 (public vs. tenant catalog split).
- Operators should be able to inspect and validate the full dependency graph through the Solar API.
- Infrastructure and application components must be distinguishable at the API level, not only in UIs, because
  future RBAC policies will grant different ClusterRoles access to infrastructure- vs. application-layer
  `ComponentVersions`.
- A dependency shared by multiple consumers must not be garbage-collected while any consumer is still present.

## Proposed Approach: Handle OCM Component References as distinct ComponentVersions

OCM's `ComponentReferences` map naturally to Solar's existing `ComponentVersion` resource. The proposal is:

Every direct and transitive OCM component reference becomes a `ComponentVersion` entry in the Solar catalog.
`ComponentVersionSpec` gains a `Dependencies` field that records the direct dependencies. The Release controller
gates rendering on all transitive dependencies being present and rendered.

### Why this approach is attractive

- No new resource types are introduced; the existing `Component` / `ComponentVersion` model absorbs dependencies
  naturally.
- The OCM component descriptor already contains the authoritative dependency graph; Solar reads it instead of
  inventing a parallel mechanism.
- Operators can query `kubectl get componentversions` and see all transitive artifacts, including infrastructure
  components like cert-manager without looking outside Solar.
- Validating "are the air-gapped registries complete?" reduces to "does every `ComponentVersion` referenced in the
  dependency tree have a corresponding `ComponentVersion` resource in Solar?".

---

## Feasibility Analysis

### Data source — OCM component descriptor

The `WriteAPIResourceEvent` already carries `compdesc.ComponentSpec`. Its `References` slice holds each component's
direct dependencies: the referenced component name, version specifier, and a local alias. No additional registry
calls are required; the descriptor itself is the authoritative source for the full dependency graph.

### API changes required

**`ComponentVersionSpec`** gains a `Dependencies` slice:

```go
// ComponentLayer classifies a ComponentVersion within the Solar catalog.
// +enum
type ComponentLayer string

const (
    // ComponentLayerApplication marks a component as a tenant-selectable application.
    // Application-layer ComponentVersions are visible in the catalog UI and may be referenced
    // in Releases directly by deployment coordinators.
    ComponentLayerApplication ComponentLayer = "application"
    // ComponentLayerInfrastructure marks a component as platform infrastructure.
    // Infrastructure-layer ComponentVersions are managed implicitly as dependencies; they are
    // hidden from the default catalog view and subject to dedicated RBAC rules.
    ComponentLayerInfrastructure ComponentLayer = "infrastructure"
)

type ComponentDependency struct {
    // ComponentRef is a reference to the dependent Component resource in the same namespace.
    // Populated by the discovery worker after resolving the ComponentReference.
    ComponentRef corev1.LocalObjectReference `json:"componentRef"`
    // ComponentVersionRef is the locked (resolved) ComponentVersion for this dependency.
    // Written by the dependency lock resolver after evaluating the Version constraint against
    // locally available ComponentVersions. The Release controller reads only this field;
    // rendering is always deterministic regardless of what the raw Version constraint says.
    // Empty until the lock resolver has run; DependenciesResolved is False while empty.
    // +optional
    ComponentVersionRef corev1.LocalObjectReference `json:"componentVersionRef,omitempty"`
    // Version is the SemVer constraint for this dependency as declared in the OCM component
    // descriptor (e.g. "^1.15.0", ">=1.15.0 <2.0.0"). Evaluated by the lock resolver against
    // locally available ComponentVersions; the resolved name is written into ComponentVersionRef.
    Version string `json:"version"`
    // ClusterWide indicates that this dependency must be installed cluster-wide on the target
    // (i.e. not isolated within a tenant namespace). Typical examples are operators such as
    // cert-manager or Kyverno whose CRDs and controllers must be cluster-scoped.
    // When true, the Release controller must ensure the dependency is rendered and deployed
    // before any namespace-scoped consumer, and only once per target regardless of how many
    // Releases reference it.
    // +optional
    ClusterWide bool `json:"clusterWide,omitempty"`
}

type ComponentVersionSpec struct {
    // ... existing fields ...
    // Layer classifies this ComponentVersion within the Solar catalog.
    // Defaults to "application" if not set.
    // +optional
    Layer ComponentLayer `json:"layer,omitempty"`
    // Dependencies are the direct OCM component references declared in the component descriptor.
    // Transitive closure is computed by following each dependency's own Dependencies.
    // +optional
    Dependencies []ComponentDependency `json:"dependencies,omitempty"`
}
```

A new status condition `DependenciesResolved` on `ComponentVersion` signals whether all direct dependencies are
themselves present in the catalog.

### Discovery pipeline changes

The `Handler` in `pkg/discovery/handler/` currently processes a single `ComponentVersionEvent` and emits one
`WriteAPIResourceEvent`. It must be extended to:

1. Extract `ComponentSpec.References` from the component descriptor.
2. For each reference, emit a new `RepositoryEvent` for the dependency (using the same registry, since
   OCM transport typically co-locates all components in the same OCI namespace for air-gapped scenarios).
3. Track a "visited" set (component name + exact version) to prevent infinite loops for self-referential or
   accidentally cyclic descriptors.
4. Propagate the `Dependencies` slice into the `WriteAPIResourceEvent` so the API writer can populate
   `ComponentVersionSpec.Dependencies`.

The `APIWriter` in `pkg/discovery/apiwriter/` must be extended to:
- Upsert `ComponentVersion` resources for dependencies before writing the parent.
- Set `ComponentVersionSpec.Dependencies` on the parent.

### Release controller changes

Before creating a `RenderTask` for a `Release`, the controller must:

1. Walk the transitive dependency graph of the target `ComponentVersion`.
2. Verify that every reachable `ComponentVersion` exists and has condition `DependenciesResolved=True`.
3. Verify that every reachable `ComponentVersion` has already been rendered (i.e. its artifacts are present in the
   render-destination registry bound to the `Target`). Initially this means a corresponding rendered artifact exists;
   a dedicated `Rendered` status condition on `ComponentVersion` (or a lookup of a completed `RenderTask` or similar) is needed.
4. Only then create the `RenderTask`. A new condition `DependenciesReady` on `Release` controls the blocking.

If dependency `ComponentVersions` are only available in a different namespace / registry (e.g. the public catalog from ADR-005), the existing `ReferenceGrant` mechanism must be extended to cover dependency traversal.

### Renderer changes

When a component's Helm chart references resources (container images, sub-charts) pulled from the
OCM source registry, those resources must be available locally. Two rendering strategies emerge:

**Option A — Pre-rendered dependencies (preferred):**
Each `ComponentVersion` is rendered independently and pushed to the target registry before its consumers are rendered.
The Release controller schedules `RenderTasks` in topological order (dependencies first). This is the most modular approach.

**Option B — Bundle rendering:**
The renderer receives the full dep tree in the `RendererConfig` and renders everything in one job. Simpler
scheduling, but creates large monolithic jobs and couples unrelated components.

Option A aligns better with the existing `RenderTask` model and the one-task-per-release approach.

---

## What Needs to Be Changed / Ensured

### Must-do

| Area | Change |
|------|--------|
| `api/solar/componentversion_types.go` | Add `Layer ComponentLayer`, `Dependencies []ComponentDependency` (incl. `ClusterWide bool`) to `ComponentVersionSpec`. Add `DependenciesResolved` condition. |
| `pkg/discovery/handler/` | Extract `ComponentSpec.References` and Solar OCM labels (`layer`, `cluster-wide`(possible?)), emit `RepositoryEvent` per dependency, keep track of cycle-detection state, populate `WriteAPIResourceEvent.Dependencies`. |
| `pkg/discovery/apiwriter/` | Write dependency `ComponentVersion` records before the parent; set `spec.dependencies`, `spec.layer` (`spec.cluster-wide`?). |
| `pkg/controller/release_controller.go` | Do not create `RenderTask` unless `DependenciesReady` resolves. Add topological ordering (dependencies first), enforce single-instance-per-target for `clusterWide` dependencies. |
| Dependency GC / reference counting | Implement finalizer + `status.consumerCount` on dependency `ComponentVersions`; block deletion while counter > 0. |
| Dependency lock resolver | Implement a controller (or similar) that watches for new `ComponentVersion` resources and evaluates `spec.dependencies[*].version` (SemVer constraints) against locally available `ComponentVersions`. Writes the resolved name into `spec.dependencies[*].componentVersionRef`. Enforces forward-only lock advancement; tracks unsatisfied constraints as `DependenciesResolved=False`. |

### Should-do

| Area | Change |
|------|--------|
| Release controller | Add `DependenciesReady` condition that shows/tracks which dependency is blocking and why (name, version, health state). |
| Release/Profile status | Add `Rendered` condition so the topological scheduler can detect which deps are ready. |
| ComponentVersion status | Add `consumerCount` reference count for operators and tooling. |
| RBAC | Define ClusterRole templates separating `infrastructure`- from `application`-layer `ComponentVersion` access. |
| Public catalog integration | `ReferenceGrant` policy must allow cross-namespace dependency traversal. Cross-namespace reference counting requires a dedicated index or finalizer-based approach. |
| Day-2 / fleet management | Define infrastructure Profile conventions. Formalise whether `InfrastructureProfile` warrants a dedicated resource type (see Concern #10). |

### Nice-to-have

| Area | Change |
|------|--------|
| `kubectl` UX | A `kubectl solar tree <release>` plugin or `additionalPrinterColumns` showing dep depth / readiness. |
| Catalog sync tooling | A Solar CLI command that prints the full OCI artifact list needed for a release (for use with ARC / ocm). |

---

## Concerns and Risks

### Version ranges vs. exact pinning

This is a fundamental architectural trade-off. The wrong choice in either direction has severe operational
consequences.

**Why semver ranges cannot work naively:** Range resolution (`>= 1.15`) requires querying a registry for available
versions. In an air-gapped cluster only the local registries exist, so the result depends entirely on what was
transported, creating non-determinism that can produce unexpected upgrades simply because the local registries were
updated, without any change to the consuming component descriptor.

**Resolution: Add Solar lock-file mechanism**

SemVer constraints (`^1.15.0`, `>=1.15.0`) are used in `ComponentDependency.Version`, as declared in the OCM
component descriptor. Solar adds a lock-file layer on top: a dedicated **dependency lock resolver** (a controller
watching for new `ComponentVersion` resources) evaluates each constraint against the locally available
`ComponentVersions` and writes the result into `ComponentVersionRef`. This combines the flexibility of SemVer
with the determinism of exact pinning:

- **Deterministic rendering.** The renderer and Release controller read only `componentVersionRef`, an exact,
  locked name. Two render passes of the same locked state produce byte-identical results.
- **No re-publish cascade.** When cert-manager 1.15.1 lands, only that one component needs to be transported
  through the air gap. The lock resolver determines when and how to update consumer `componentVersionRef` pointers.
  No application `ComponentVersion` is re-published.
- **No external registry access at resolution time.** The lock resolver queries Solar's own `ComponentVersion`
  resources, never an upstream registry, so the air-gap boundary is never crossed during resolution.
- **Auditable lock state.** `kubectl get componentversion my-app -o jsonpath='{.spec.dependencies}'` shows the
  exact locked reference for every dependency, at any point in time.

**How the lock resolver works:**

Whenever a new `ComponentVersion` resource is created or updated by the discovery worker, the lock resolver:

1. Queries all `ComponentVersions` in the same namespace whose `spec.dependencies` contain a `version` constraint
   for the newly arrived component.
2. For each consumer, resolves all constraints against the full locally available set (highest semver satisfying
   the constraint wins).
3. Patches `spec.dependencies[n].componentVersionRef` with the resolved name.
4. Sets `DependenciesResolved=True` when all constraints are satisfied; `DependenciesResolved=False` when no
   available version satisfies a constraint

The Release controller reads only the locked `componentVersionRef`. If it is empty, it means the constraint is not yet resolved or no satisfying version is present locally, thus `DependenciesReady=False` blocks rendering.

**Downgrade prevention:** The lock resolver must only advance locks forward to a higher satisfying version.
If a `ComponentVersion` is removed from the local registry, the resolver must surface `DependenciesResolved=False`
rather than silently downgrading to an older version.

**Update walkthrough (cert-manager CVE patch):**

1. A USB stick / artifact diode delivers `cert-manager@1.15.1` to the connected side.
2. `ocm transfer` imports it into the shared infrastructure OCI registry.
3. Solar's discovery worker detects the new tag → creates `ComponentVersion/cert-manager-1.15.1`.
4. The lock resolver detects the new `ComponentVersion`, finds all consumers whose `version` constraint (e.g.
   `^1.15.0`) is satisfied by `1.15.1`, and patches their `componentVersionRef` to `cert-manager-1.15.1`.
5. The updated `ComponentVersion` resources trigger Release controller reconcile.
6. The Release controller schedules new `RenderTasks` per target wave.
7. Flux on each target cluster picks up the updated desired-state image and reconciles cert-manager to 1.15.1.
8. The platform operator proceeds to the next wave once the health gate is satisfied.

No application component is re-published or re-transported.

### Diamond dependencies

Component A depends on C@1.0; Component B (also a dependency of A) depends on C@1.1. Solar must decide:

- **Conflict** — raise `DependencyConflict` condition, block rendering.
- **Latest wins** — silently pick C@1.1. Risky; the component expecting C@1.0 may break.
- **Both coexist** — render both C@1.0 and C@1.1 as separate `ComponentVersion` resources; each consumer gets its own
  rendered image. Safer but wastes resources.

**Important constraint for cluster-scoped components:** The "both coexist" option is **fundamentally impossible**
for any dependency with `clusterWide: true`. A cluster-scoped operator (cert-manager, Kyverno etc.) can only be
installed once per target cluster; two versions cannot coexist. This means that for infrastructure dependencies a
common denominator version must always be enforced: all consuming components in a release set must
agree on exactly one version before the bundle is published. Solar surfaces a conflict as `DependencyConflict` and
blocks the render; resolution is the responsibility of the OCM package author, not of Solar.

For the initial implementation, **conflict** is the safe default for all diamond cases. Coexistence (namespaced
dependencies only) can be revisited later.

### Shared dependency rendering and race conditions

Multiple `Releases` bound to the same `Target` may share a dependency. Rendering the shared dependency twice
independently could produce distinct image digests and confuse Flux reconciliation.

**Mitigation:** The `Release` controller must use a registry-key-based leader-election or a
`DependencyRenderTask` concept: a `RenderTask` is created for a (ComponentVersion, Target) tuple, not for a
(Release, Target) tuple. If a `RenderTask` already exists for that tuple, the controller waits for it to complete
rather than creating a duplicate.

### Cycle detection in the discovery pipeline

OCM forbids cycles in component references, but Solar cannot trust external package authors to adhere to this.

**Mitigation:** The discovery handler must maintain a visited set of (component, version) pairs and abort with an
`ErrorEvent` if a cycle is detected. The affected `ComponentVersion` receives a `CyclicDependency` condition.

### Catalog layering — UX, RBAC, and cluster-wide installation scope

Surfacing all transitive dependencies as individual `ComponentVersion` resources leads to two distinct problems that
both require an explicit `Layer` classification on `ComponentVersion`:

**UX / Developer Experience:** If infrastructure components (cert-manager, Prometheus Operator, Kyverno etc.) appear
alongside tenant applications in the catalog, end users face a confusing mix of many platform components next to the
applications they actually want to deploy. The `layer` field drives catalog UI filtering: `infrastructure` components
are hidden from default views and only visible to platform administrators.

**RBAC separation:** In multi-tenant environments, deployment coordinators should be able to create `Releases`
referencing `application`-layer `ComponentVersions` only. Platform providers manage `infrastructure`-layer
`ComponentVersions` via separate ClusterRoles. Solar's admission logic (or RBAC at the Kubernetes level) can enforce
this once the layer is a first-class field rather than a convention.

**Cluster-wide vs. namespace-scoped installation:** Some infrastructure dependencies (cert-manager, Kyverno,
Prometheus CRD stacks) must be installed cluster-wide because their CRDs and controllers are cluster-scoped. A
namespace-scoped tenant application cannot install them in isolation. The `ClusterWide` flag on `ComponentDependency`
signals this to the Release controller:

- `clusterWide: true` — the dependency is rendered and expected to be deployed to the target as a cluster-scoped
  workload. Only one instance per (ComponentVersion, Target) pair is allowed, regardless of how many Releases share
  it
- `clusterWide: false` (default) — the dependency is deployed within the Release's `targetNamespace`, isolated per
  tenant.

Multi-cluster setups with local pull-through proxies or shared registry mirrors are tangentially affected (a
cluster-wide dependency might be served from a different local mirror than namespace-scoped resources), but this
amounts to a Registry / RegistryBinding configuration concern and does not change the dependency model itself.

**Mitigation:** Populate `layer` during discovery from a OCM label
(`ocm.software/solar/layer: infrastructure`); fall back to `application` if absent. Set `ClusterWide` from an OCM
label (`ocm.software/solar/cluster-wide: "true"`) on the component descriptor reference entry. These labels become
the publishing convention for OCM package authors targeting Solar.

### Garbage collection and reference counting

When a `Release` or its parent `ComponentVersion` is deleted, the Solar controller must not automatically delete
dependency `ComponentVersions` that are still referenced by other `ComponentVersion` resources. Without explicit
reference tracking, deleting one Release could silently remove a shared cert-manager `ComponentVersion` that another
Release still needs, breaking a deployed workload on the target.

The implementation approach is deliberately left open in this ADR, but any solution must satisfy:

1. **No orphan references.** A `ComponentVersion` with `layer: infrastructure` (or any dependency) must not be
   garbage-collected while it appears in the `spec.dependencies` list of at least one other `ComponentVersion` that
   is itself not being deleted.
2. **Cascading cleanup.** When the last consumer `ComponentVersion` is deleted, the dependency `ComponentVersion`
   becomes eligible for cleanup (either automatically or by surfacing a `ReferencedBy: 0` signal to an operator).
3. **Cross-namespace awareness.** If dependency `ComponentVersions` live in a shared namespace, reference counting 
   must span namespace boundaries. Standard Kubernetes owner references do not support this, so a finalizer-based
   approach with a `ReferencedBy` counter in status or a dedicated index is required.

The concrete mechanism is an implementation decision; this ADR captures the requirement.

### Registry placement for infrastructure components

Infrastructure-layer `ComponentVersions` are shared by every target in a
fleet. For Solar's rendering pipeline to work, every target's bound `Registry` resources must include a registry that
contains these infrastructure artifacts. Two placement strategies are possible:

**Option A: Co-located with application artifacts:**
Infrastructure components are mirrored into the same per-tenant or per-namespace OCI registry as application
components. The existing `Registry` / `RegistryBinding` model (ADR-009) handles this without changes. However, with
many targets each requiring its own `RegistryBinding` pointing to the same infrastructure registry.
At scale this becomes unwieldy.

**Option B: Dedicated shared infrastructure registry:**
A separate OCI registry is designated exclusively for infrastructure-layer components and is accessible from all
targets. A single `Registry` resource describes it; ideally, a single platform-wide `RegistryBinding` (or a
cluster-scoped equivalent) would bind it to all targets automatically. This does not scale well under the
current namespaced `RegistryBinding` model because one binding object per target is still required.

**Open question / further discussion needed:**
Neither option is fully satisfying today. Option A requires manual binding maintenance at scale; Option B exposes a
limitation of the current `RegistryBinding` model for fleet-wide resources. A possible path forward is a
`ClusterRegistryBinding` (cluster-scoped, matches all or selected Targets) that grants access to a shared
infrastructure registry without per-target boilerplate, but this is a separate ADR-level decision.
For now, Option A is the only implementable path. The scaling concern is recorded here and must be
addressed before Solar targets fleet sizes where per-target binding management becomes operationally expensive.

#### Open questions

- **Infrastructure Profile as a first-class concept:** Should Solar formalise `InfrastructureProfile` as a distinct
  resource (separate RBAC, separate controller, reserved for platform providers) rather than relying on conventions
  on an existing `Profile`? This is a follow-up ADR question.
- **Health gate between waves:** How does Solar determine that wave 1 is healthy before advancing to wave 2? This
  requires integration with the target cluster's health reporting and is not yet available (solar-agent?)

---

## Consequences

### Positive

- Full declarative visibility of every artifact needed for any deployment, directly queryable via `kubectl`.
- Air-gapped completeness can be validated before any render or deployment is attempted.
- OCM's own component graph is the single source of truth; Solar does not invent a parallel dependency mechanism.
- `Release` controller failures become deterministic and informative (`DependenciesReady=False, reason=MissingDependency`).
- Shared infrastructure components (e.g. cert-manager) can be modeled as required dependencies, with a registry
  presence gate enforced before any consuming Release proceeds to render.
- SemVer constraints with lock-file: infrastructure CVE patches require deployment only one component per target / namespace, regardless of how many application components declare a dependency constraint on it. No re-publish cascade.

### Negative

- API surface grows: `ComponentVersionSpec.Layer`, `ComponentVersionSpec.Dependencies` (incl. `ClusterWide`), new
  conditions, `status.consumerCount`.
- Discovery pipeline complexity increases significantly (recursive descent, cycle detection, label extraction,
  cross-registry checks).
- Topological scheduling of `RenderTask` creation adds a new ordering concern to the Release controller.
- The lock resolver introduces a new control loop: forward-only lock advancement must be correctly implemented to
  prevent silent downgrades if a `ComponentVersion` is removed from the local registry.
- Diamond dependency conflicts require human intervention; automated resolution is not safe without explicit policy.
- Reference counting logic is non-trivial in the cross-namespace case
- Bootstrapping order (transport complete → discovery → render → deploy) must be clearly documented and ideally
  enforced via pre-flight checks, otherwise operators will encounter confusing `MissingDependency` conditions.
- Infrastructure Profile pattern (label-selector waves, health gate) requires formalisation before fleet-scale
  rollout is operationally safe.
