---
status: draft
date: 2026-07-13
---

# Solar Agent Architecture

## Context and Problem Statement

[#61](https://github.com/opendefensecloud/solution-arsenal/issues/61) ("Implement Solar Agent") has four sub-issues:

- [#407](https://github.com/opendefensecloud/solution-arsenal/issues/407) (registration),
- [#408](https://github.com/opendefensecloud/solution-arsenal/issues/408) (status reporting),
- [#409](https://github.com/opendefensecloud/solution-arsenal/issues/409) (preflight),
- [#410](https://github.com/opendefensecloud/solution-arsenal/issues/410) (Helm chart/deployment)

each carrying open design questions. Per [#665](https://github.com/opendefensecloud/solution-arsenal/issues/665), this ADR seeks to answer the ones that are architectural.

## Decisions

### Agent <-> apiserver: polling vs event-driven watch

#408 asks for "real-time visibility," but also for efficient change-only updates, resilience to intermittent
connectivity, and stale-status detection. Those four pull in the same direction, and none of them require an
event-driven local watch to satisfy. A poll loop that pushes on change, with a heartbeat, delivers all four with
far fewer moving parts.

**Efficient, changes-only updates.** each tick, the agent diffs the freshly-collected report against the last one it
successfully pushed, and skips the push if nothing changed. A poll loop that diffs before pushing is exactly as
bandwidth-efficient as a watch-triggered push, but simpler.

**Resilient to intermittent connectivity.** A failed push is retried on the next tick. The poll interval doubles
as backoff, so no separate retry scheduler is needed. No durable queue is needed either: a report is a snapshot, not
a command that must eventually apply, so a report from three ticks ago has no value once a fresher one exists.
The agent can simply drop the old report and try again on the next tick.

### Deployment status source of truth: FluxCD conditions

The bootstrap chart creates one `OCIRepository`/`HelmRelease` pair per bound Release; the agent can roll up
their `Ready` conditions rather than tracking rollout state itself.

### Status API surface

A new per-target resource (tentatively `TargetReport`), owned solely by the agent, one per `Target`. Not
`Target.status`: that subresource is already written by solar-controller-manager
and a second writer risks data races on it. Not `ReleaseBinding.status`: that resource is provider-owned
and one Target can have many bindings, which would fragment a single agent's report across N objects.
Dead-agent detection is a `lastReportTime` heartbeat field, checked centrally by solar-controller-manager, so
the agent itself needs no self-monitoring logic.

### Registration flow / agent config

- **Auth**: a ServiceAccount token (kubeconfig-shaped), RBAC-scoped to only this Target's own report/status
- **Delivery**: a Secret, referenced from `Target.status` (e.g. `status.agentConfigSecretRef`)
- **Persistence**: a normal rotatable credential, not single-use. Rotation mechanics are out of scope for this ADR
- **Additional path, built during this spike (not required by #407)**: self-registration. Given a
  namespace-scoped bootstrap token, `solar-agent` can create its own `Target` on first run instead of requiring one
  to exist first (`pkg/agent/registrar.go`, `test/fixtures/setup-agent-self-register.sh`). This is additive to, not
  a replacement for, the Target-creation-generates-config flow #407 asks for. Where the self-registered `Target`
  lands governs whether it needs a `ReferenceGrant` to resolve its render `Registry`
  ([ADR-012](./012-ReferenceGrants.md)): registering directly into the Registry's own namespace (`solar-system` in
  the dev cluster) needs none; a separate tenant namespace needs a `ReferenceGrant` there, same as any other
  cross-namespace Target → Registry reference.

### Preflight checks on every reconciliation

#409 flags this as a TBD ("run before each reconciliation, not just on first deployment (TBD?)"). Decided: every
reconciliation. A one-time gate can't catch regressions: FluxCD CRDs removed, RBAC narrowed, a namespace deleted,
after the first successful bootstrap; the agent would then fail later reconciles with no diagnostic trail, or
worse, silently stop reconciling with no visible cause. Recomputing every reconcile also matches how every other
condition in this codebase already behaves (`RegistryResolved`, `ReleasesRendered`, ...). A sticky-once-true
`Preflight` would be the odd one out. The checks are cheap (a handful of `Get`/`List` calls), well inside the
per-tick budget the poll loop already spends on reachability and status collection.

Checks split into two kinds:

- **Self-healing**: FluxCD CRDs missing -> the agent (re-)installs FluxCD, since it (possibly) owns that install already (see
  "Deployment engine" below); target namespace missing -> the agent creates it, matching the AC's "exist or can be
  created." These aren't gates so much as repair actions the agent takes each tick before proceeding.
- **Hard-fail (external dependency, agent can't self-heal)**: apiserver reachability, OCI registry reachability,
  RBAC self-check (`SelfSubjectAccessReview`). These set `Preflight=False` with a reason/message and the tick stops
  there; the next tick retries, same as any other push failure.
- **Capacity constraints**: blocked on `Target` gaining capacity fields (#406). But once it exists, it belongs in
  the hard-fail category and needs the same every-reconciliation cadence, not just first deployment

### Deployment engine

The agent installs FluxCD itself (one easy solution proposed to ensure air-gapped capability: a pinned version
embedded in the agent binary) rather than requiring it pre-installed, then installs the target's bootstrap chart,
which creates the per-release Flux objects. This keeps the agent self-contained.

### Deployment packaging

A new `charts/solar-agent` chart is needed: single-replica Deployment with a ServiceAccount and RBAC.
A solar-controller-manager-initiated push install (`Target.spec.agentAccessSecretRef`, built during this
spike, see `pkg/controller/target_agent_installer_controller.go`) is an additional, optional path for target
clusters SolAr is already given access to. Not a replacement for manual deploy, and not something #410 asked for,
but complementary to it.

## Out of Scope / Left for Sub-Issue Implementation

- `TargetReport` resource and real status push (#408): Draft API Surface exists in `pkg/agent/status.go`,
  but might change once the real status fields are known
- Real `solar-agent` Helm chart and image (#410): not built; the current remote-install path uses a
  placeholder installer (`MarkerInstaller`).
- Agent-config Secret generation on Target creation (#407): not implemented; the self-registration
  path currently assumes a bootstrap token was provisioned some other way.
- Capacity-constraint preflight checks (#409): blocked on Target capacity fields.
- Credential rotation, for either agent config or remote-install kubeconfigs.
