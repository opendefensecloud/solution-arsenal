---
status: draft
date: 2026-03-16
---

# kro Integration

## Motiviation

[kro](https://kro.run/) (Kube Resource Orchestrator) is a Kubernetes-native project
under [kubernetes-sigs](https://github.com/kubernetes-sigs/kro) that lets platform teams define custom APIs by composing
groups of Kubernetes resources — including any CRD — into a single declarative abstraction called a
ResourceGraphDefinition (RGD). A controller running in-cluster continuously reconciles the resulting ResourceGroup
instances, wiring values between resources via CEL expressions and providing automatic drift detection. kro's API is
currently at v1alpha1; see the [documentation](https://kro.run/docs/overview/) for details.

This document should discuss the tool and it's integration.

## Aspects

### kro operator as mandatory infrastructure on targets

Deploying kro-based components requires the kro operator to be installed and running on every target cluster. Unlike
Helm, which only needs a client-side renderer, kro relies on a server-side controller that watches
ResourceGraphDefinition (RGD) and ResourceGroup custom resources. This makes the kro operator a prerequisite at the
target level — comparable to a CNI or cert-manager — and must be treated as mandatory cluster infrastructure before any
kro-backed Release can be rolled out.

### Close conceptual distance to Profile, diverging in cardinality

SolAr's Profile and a kro ResourceGroup instance serve the same purpose: bind a definition to targets with custom
configuration and let a controller reconcile actual state. However, they diverge in cardinality. A Profile is a
one-to-many broadcast — one Profile selects many Targets via label selectors and the TargetReconciler propagates the
binding into each HydratedTarget. A kro instance is one-to-one — each ResourceGroup renders exactly one resource graph
in its own namespace. Bridging the two means a single Profile referencing a kro-backed Release would fan out into one
ResourceGroup instance per matched Target.

### Missing official OCM artifact type for kro resources

The OCM specification defines artifact types for `helmChart`, `ociImage`, `blob`, `directoryTree`/`fileSystem`, and
others, but there is no registered artifact type for kro ResourceGraphDefinitions. Today an RGD packaged in an OCM
component would have to be stored as a generic `fileSystem`/`directoryTree` (a tgz of YAML manifests) or a plain `blob`,
losing the semantic distinction that lets the discovery pipeline classify it. The handler classification logic (
`handler.go:149-159`) relies on typed resources to route to the correct handler; without a dedicated type, detection
would fall back to heuristics (e.g. inspecting manifest content), which is fragile. An upstream OCM artifact type
proposal — or at minimum a project-local convention — is needed before the discovery side can reliably identify and
process kro components.

### Updating RGD Specs

Prior to 0.8, modifying an RGD spec was effectively unsafe. Adding or renaming a field could leave the generated CRD
out of sync with existing ResourceGroup instances — the controller did not update the CRD schema atomically, so
instances created under the old schema would fail validation or get stuck with hanging finalizers
([kro#428](https://github.com/kubernetes-sigs/kro/issues/428)). Changes to default values were silently ignored for
existing instances because the dynamic controller compared generations rather than detecting drift
([kro#329](https://github.com/kubernetes-sigs/kro/issues/329)).

Since 0.8, kro detects breaking changes (removed fields, type changes, new required fields without defaults) at
admission time and rejects them by default, preventing the most dangerous failure mode. Non-breaking changes (adding
optional fields, updating defaults) are now applied to the CRD schema consistently. This makes day-2 RGD maintenance
viable, but propagation to existing instances is still immediate and all-or-nothing — there is no way to control
rollout pace or roll back.

The [0.9 milestone](https://github.com/kubernetes-sigs/kro/milestone/9) targets controlled versioning and rollout of
RGD changes ([kro#883](https://github.com/kubernetes-sigs/kro/issues/883)): opt-in update propagation to instances,
rollback support, and multi-version RGD definitions. Once landed, RGD authors and instance owners will be able to
coordinate upgrades rather than having every change applied globally on write.

## Open Questions, how to move on
 
- **RGD lifecycle**: How is the RGD deployed? How are instances deployed against it? Can two instances use different RGD
  versions simultaneously?
- **Implementation approach** — two options identified:
    1. **[Dependency mechanism](https://github.com/opendefensecloud/solution-arsenal/issues/71)**: separate releases for RGD and instance; potentially complex for users
    2. **Handle RGD during rendering**: less user-facing complexity but adds rendering logic

