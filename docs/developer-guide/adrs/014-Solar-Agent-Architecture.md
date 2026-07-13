---
status: draft
date: 2026-08-29
---

# Solar Agent Architecture

## Context and Problem Statement

[#61](https://github.com/opendefensecloud/solution-arsenal/issues/61) ("Implement Solar Agent") has four
sub-issues — [#407](https://github.com/opendefensecloud/solution-arsenal/issues/407) (registration),
[#408](https://github.com/opendefensecloud/solution-arsenal/issues/408) (status),
[#409](https://github.com/opendefensecloud/solution-arsenal/issues/409) (preflight),
[#410](https://github.com/opendefensecloud/solution-arsenal/issues/410) (chart) — each carrying open design
questions. Per [#665](https://github.com/opendefensecloud/solution-arsenal/issues/665), this ADR answers the
architectural ones.

The decisive constraint: `solar-agent` ships as part of the **fogctl default application set** (the
`WorkloadDefaults` map in `pkg/fogctl/const.go`) deployed onto every newly bootstrapped OSCP cluster. So the agent
is installed at bootstrap time — before any `Target` exists, from an air-gapped OCM catalog, onto a cluster that
already has Flux, and with no route from SolAr to the cluster.

## Decisions

### Agent → apiserver: poll loop

Each tick the agent collects a report, diffs it against the last successfully pushed one, and pushes only on
change. As bandwidth-efficient as a watch-triggered push, with less for the solar controller to do. A failed push
retries next tick, so the interval doubles as backoff.

### Status source of truth: FluxCD conditions

The bootstrap creates one `OCIRepository`/`HelmRelease` pair per bound Release, both carrying the label
`solar.opendefense.cloud/release` and an annotation of the same name. The agent lists by that label and aggregates
the pair's conditions, it does not track rollout state itself. This way the agent is a stateless collector.

Both halves of the pair are reported. `OCIRepository.Ready` covers fetching and verifying the artifact (registry
reachability and auth, semver match, cosign `spec.verify`); `HelmRelease.Ready` covers applying the chart and the
health of what it installed. They fail for different reasons. Collapsing them costs the operator that distinction.
Side Note: The PoC collector currently lists only `helmreleases`.
Conditions are only trusted when `status.observedGeneration == metadata.generation`. Without that gate a report can
present the _previous_ rollout's success as the current one's.

Deriving the states from what Flux exposes. The two objects do not carry the same condition set — most notably
`Stalled` exists only on `OCIRepository`, and `Remediated`/`Drifted`/`TestSuccess` only on `HelmRelease` — so each
state names the object it is read from:

| Reported state | Read from       | Condition / field                                                                                                                                                                                                  |
| -------------- | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Ready          | both            | `Ready=True` on both                                                                                                                                                                                               |
| Verified       | `OCIRepository` | `SourceVerified=True` — the result of cosign `spec.verify`. Distinct from Ready: an unverified artifact is a trust failure, not a fetch failure                                                                    |
| Progressing    | both            | `Reconciling=True` (`HelmRelease` marks it with reason `Progressing`), `Ready=Unknown`, or `metadata.generation != status.observedGeneration`                                                                      |
| Degraded       | both            | `Ready=False`, retry still coming — on `OCIRepository` that means no `Stalled`; on `HelmRelease` it means `Failures`/`InstallFailures`/`UpgradeFailures` below the configured `retries`                            |
| Failed         | `OCIRepository` | `Ready=False` with `Stalled=True` — terminal, no retry coming. **`HelmRelease` never sets `Stalled` on itself**; helm-controller only reads it off the source it depends on                                        |
| Failed         | `HelmRelease`   | no terminal condition exists. Derived: `Remediated=True` with `InstallFailures`/`UpgradeFailures` at the chart's `remediation.retries: 3`                                                                          |
| Rolled back    | `HelmRelease`   | `Remediated=True` (reason `RollbackSucceeded`). The release can be live at its _previous_ version while `Ready=True`; reporting that as plain Ready hides a failed upgrade, and it is the signal #587 needs        |
| Test failed    | `HelmRelease`   | `TestSuccess=False` — the bootstrap chart sets `test.enable: true`, so tests run and, unless `ignoreTestFailures`, a test failure triggers remediation                                                             |
| Drifted        | `HelmRelease`   | `Drifted=True` (reason `DriftDetected`) — a real condition, not just events. The chart enables `driftDetection`, and this is distinct from not-Ready                                                               |
| Pending        | neither         | agent-derived, not a Flux condition: no pair exists yet for a bound Release, i.e. the applied bootstrap chart is older than the ReleaseBinding set. Distinguished from Failed via `Target.status.bootstrapVersion` |

These are not all peers. `Pending`/`Progressing`/`Ready`/`Degraded`/`Failed` are mutually exclusive lifecycle
states. `Verified`, `Rolled back`, `Test failed` and `Drifted` are **orthogonal** — a release can be `Ready` and
drifted, or `Ready` while live at its pre-rollback version. Modelling them in one enum would force exactly the
collapse this section exists to avoid.

### Status API surface: a new `TargetReport`

One per `Target`, written by the agent (`pkg/agent/types.go`):

```go
type TargetReport struct {
	LastReportTime metav1.Time     `json:"lastReportTime"`
	Capacity       ClusterCapacity `json:"capacity"`
	Releases       []ReleaseStatus `json:"releases"`
}

type ClusterCapacity struct {
	NodeCount   int32               `json:"nodeCount"`
	Allocatable corev1.ResourceList `json:"allocatable"`
	Used        corev1.ResourceList `json:"used"`
}

type ReleaseStatus struct {
	Name string `json:"name"`
	Phase ReleasePhase `json:"phase"`
	Revision string `json:"revision,omitempty"`
	SourceConditions []metav1.Condition `json:"sourceConditions,omitempty"`
	HelmConditions   []metav1.Condition `json:"helmConditions,omitempty"`
}
```

### Registration: self-registration is the primary flow

Because the agent ships with the platform, it (potentially) starts before it knows whether SolAr exists at all. fogctl
may be bootstrapping the first cluster in an environment, or a cluster that will itself later host the SolAr control
plane. So installation must never presuppose a control plane, and the agent resolves which of three states it is in
on every tick. The config is re-read each time, so a state change needs no restart:

| State           | Condition                               | Behaviour                                                                                                                                                                                                          |
| --------------- | --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Dormant**     | no apiserver endpoint configured        | Idle. Pod healthy and ready, no `Target`, no writes, one log line at startup and nothing after. This is a valid steady state, not an error                                                                         |
| **Unreachable** | endpoint configured, no SolAr behind it | `Preflight=False`, retry next tick. Probe is a discovery `GET` for the `solar.opendefense.cloud/v1alpha1` API group, which also proves the endpoint is a SolAr apiserver rather than merely something that answers |
| **Registered**  | endpoint reachable                      | Self-register if no `Target` exists for this cluster, then report                                                                                                                                                  |

**Dormant must be the default.** If the chart required an endpoint, adding `solar-agent` to `WorkloadDefaults`
would break bootstrapping every cluster that has no SolAr yet, including the first one.
Given an endpoint and a namespace-scoped bootstrap token, the agent creates its own `Target` on first run

- **Auth**: ServiceAccount token (kubeconfig-shaped), RBAC-scoped to this Target's own report only.
- **Delivery**: a Secret referenced from `Target.status.agentConfigSecretRef`.
- **Persistence**: a normal rotatable credential, not single-use. Rotation mechanics out of scope.
- **Bootstrap token**: now a fogctl-provisioned input, surfaced as a Helm value or referenced Secret. Both it and
  the endpoint are optional; absent, the agent is Dormant.
- **Late binding**: an endpoint that appears after install (values updated, Secret created, SolAr deployed into
  this same cluster later in the run) is picked up on the next tick.
- **Namespace**: registering into the Registry's own namespace needs no `ReferenceGrant`; a tenant namespace needs
  one provisioned there ahead of bootstrap ([ADR-012](./012-ReferenceGrants.md)).

### Deployment engine: the agent requires Flux

The agent installs the target's bootstrap chart, which creates the per-release Flux objects.
fogctl already ships `flux` as a core workload through the same CTF transfer.

### Packaging: two paths

1. **fogctl default application set.** A **platform**-tier `WorkloadDefaults` entry (after
   `flux`, which is in the core-tier). This needs an OCM component `opendefense.cloud/fog/platform/solar-agent`
2. **Manual Helm install** `charts/solar-agent`: single-replica Deployment, ServiceAccount, RBAC. It is
   what the OCM component wraps, and what the dev cluster and E2E use.

## Consequences

- Every OSCP cluster is SolAr-manageable as soon as an endpoint is configured, with no second install step.
  Clusters with agents before SolAr exists simply sit Dormant until one does, so the ordering of the two never has to be coordinated.
- Connectivity is agent → SolAr only; SolAr needs no route to and no credential for the target, matching ADR-008.
- Air-gap capability is inherited from the platform's CTF transfer instead of re-solved in the agent, and Flux,
  cert-manager and a cluster-local `zot` are guaranteed present before the agent starts.
- The catalog, not SolAr, owns the agent version in the fleet. Version skew becomes an operational concern the
  report must surface, and shipping a fix now spans multiple repos.
- **Bootstrap-token provisioning moves to a pipeline SolAr does not own**, and self-registration lets a fleet
  create Targets faster than anyone reviews them. Token scoping and namespace placement are the only limits on
  what a bootstrapping cluster can assert about itself.

## Open Questions

- How does the bootstrap token reach the cluster — a `cluster.yaml` chart override, an
  external-secrets/Vault-backed Secret, or minted per cluster during `fogctl cluster apply`? Needs a fogctl owner.
- One token per fleet (trivial, single point of compromise) or one per cluster?
- Does a self-registered Target need approval before it is rendered for?
- Who installs the cosign public key first? we may need to have another look at #690 and decide how we handle the key,
  and whether this should perhaps be integrated in i.e. fogctl

## Out of Scope

- `TargetReport` API type — draft surface in `pkg/agent/types.go`.
- Real chart and image (#410).
- Agent-config Secret generation on Target creation (#407).
- Capacity preflight (#409), blocked on #406.
- Credential rotation of the agent config.
- The fogctl-side change itself (`WorkloadDefaults` entry, defaults file, OCM component build) — lives in the OSCP
  repos. This ADR addresses only what SolAr must provide.
