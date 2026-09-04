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

The decisive constraint: the agent is **delivered by SolAr itself**. A user creates a `Target`, the renderer
includes `solar-agent` in that Target's bootstrap output, and Flux on the target cluster — present because fogctl
put it there — installs it alongside every other bound Release. The agent is therefore never present before SolAr,
never installs itself, and arrives already knowing which `Target` it belongs to.

First contact stays manual and out of scope: someone points Flux on the target cluster at that Target's render
registry. Until that happens nothing is rendered for it and nothing reports.

## Decisions

### Agent → apiserver communication: push-only, no watch

Each tick the agent collects a report, diffs it against the last successfully pushed one, and pushes only on
change. As bandwidth-efficient as a watch-triggered push, with less for the solar controller to do. A failed push
leaves the last-pushed report untouched and so retries next tick, which makes the interval double as backoff.

Push-on-change alone would freeze `lastReportTime` on a healthy, stable cluster — and a stale `lastReportTime` is
exactly what marks a Target dead. So silence is bounded: an unchanged report is pushed anyway once it is older than
`MaxReportAge` (default ten intervals). Without that floor the two decisions contradict each other and every quiet
cluster reads as broken.

### Status source of truth: FluxCD conditions

The bootstrap creates one `OCIRepository`/`HelmRelease` pair per bound Release, both carrying the label
`solar.opendefense.cloud/release` and an annotation of the same name. The agent lists by that label and aggregates
the pair's conditions, it does not track rollout state itself. This way the agent is a stateless collector.

Both halves of the pair are reported. `OCIRepository.Ready` covers fetching and verifying the artifact (registry
reachability and auth, semver match, cosign `spec.verify`); `HelmRelease.Ready` covers applying the chart and the
health of what it installed. They fail for different reasons. Collapsing them costs the operator that distinction.
Conditions are only trusted when they describe the current spec, or a report presents the _previous_ rollout's
success as the current one's. The gate is **per condition** — each Flux condition carries its own
`observedGeneration` — not on the object's `status.observedGeneration`: helm-controller parks that field at `-1` for
the whole time a reconciliation is in flight, so gating on it discards perfectly current conditions and reports
every retrying release as `Progressing`. Verified against the dev cluster.

Deriving the states from what Flux exposes. The two objects do not carry the same condition set — most notably
`Stalled` exists only on `OCIRepository`, and `Remediated`/`Drifted`/`TestSuccess` only on `HelmRelease` — so each
state names the object it is read from:

| Reported state | Read from       | Condition / field                                                                                                                                                                                                  |
| -------------- | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Ready          | both            | `Ready=True` on both                                                                                                                                                                                               |
| Verified       | `OCIRepository` | `SourceVerified=True` — the result of cosign `spec.verify`. Distinct from Ready: an unverified artifact is a trust failure, not a fetch failure                                                                    |
| Progressing    | both            | `Reconciling=True` (`HelmRelease` marks it with reason `Progressing`), `Ready=Unknown`, or no condition current for this generation. Ranks **below** `Degraded`: Flux holds `Reconciling=True` while it retries a failed release, so checking it first would report a broken release as progressing forever |
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
	Namespace string `json:"namespace"`
	Phase ReleasePhase `json:"phase"`
	Revision string `json:"revision,omitempty"`
	SourceConditions []metav1.Condition `json:"sourceConditions,omitempty"`
	HelmConditions   []metav1.Condition `json:"helmConditions,omitempty"`
}
```

### Registration: users create Targets, SolAr renders the agent

A `Target` is created by a user, not by an agent. There is no self-registration and no discovery handshake. By the
time the agent runs its `Target` already exists, and the credential naming it was rendered into the same artifact
that delivered the agent.

| Step                                                                       | Owner                    |
| -------------------------------------------------------------------------- | ------------------------ |
| Create the `Target`                                                        | user                     |
| Mint an OAuth client for it                                                | solar-controller-manager |
| Render `solar-agent` into the Target's bootstrap output, credential inline | solar-renderer           |
| Pull and install                                                           | Flux on the target       |
| Exchange the credential for a token, report                                | the agent                |

- **Identity**: one OAuth2 client per `Target`, `client_credentials` grant, minted when the Target is created. This
  is what [#407](https://github.com/opendefensecloud/solution-arsenal/issues/407) becomes, now that no bootstrap
  token has to be distributed.
- **Delivery**: rendered inline into the bootstrap output. The render registry is already scoped per Target by its
  `RegistryBinding`, so a Target-scoped credential inside a Target-scoped artifact adds no exposure the artifact
  does not already carry.
- **Token**: a short-lived JWT from the SolAr issuer. `golang.org/x/oauth2/clientcredentials` caches it and
  re-requests on expiry, so the poll loop needs no refresh logic of its own.
- **Liveness**: the agent does not monitor itself. It arrives as an `OCIRepository`/`HelmRelease` pair like every
  other Release, so a failure of its own pair silences it. A `Target` whose `lastReportTime` has gone stale is
  therefore assumed broken, whatever the cause.

### Cross-cluster auth: a SolAr-owned OIDC issuer

ServiceAccount tokens were the obvious first answer and were rejected for various reasons, operational complexity
being the most important one.

Instead SolAr runs a small issuer: machine clients only, one grant type (`client_credentials`), a token endpoint
and a JWKS endpoint. No authorization-code flow, PKCE, consent etc.

- **Validation**: solar-apiserver validates the JWT itself, with an `oidc` authenticator unioned alongside the
  existing delegated authentication so in-cluster components (renderer, controller-manager, discovery, UI) keep
  their ServiceAccount tokens unchanged. `DelegatingAuthenticationOptions` has no OIDC support, and
  `apiserver-kit`'s builder exposes no seam to override the authenticator after `RecommendedOptions.ApplyTo`, so
  this needs a small hook there first.
- **Authorization**: the token's subject names the `Target`; a custom authorizer permits writes only to that
  Target's own report. This is what avoids one RBAC binding per agent.

### Deployment engine: Flux installs everything, the agent included

The agent installs nothing. The manual first-contact step points Flux on the target cluster at the Target's render
registry; Flux pulls the bootstrap chart, and the bootstrap chart creates the per-release
`OCIRepository`/`HelmRelease` pairs — one of which is the agent itself. The agent's only job is to read those pairs
and report on them.

Flux is present because fogctl ships it as a core workload through the CTF transfer, so nothing here installs or
owns it.

### Packaging: an ordinary catalog component

`charts/solar-agent`: single-replica Deployment, ServiceAccount, RBAC, wrapped as an OCM component like any other
catalog entry and rendered for a Target through the ordinary pipeline. The chart is also what the dev cluster and
E2E install directly.

## Consequences

- Connectivity is agent → SolAr only; SolAr needs no route to and no credential for the target cluster.
- SolAr is now in the identity business for its own API. [ADR-008](./008-No-Auth-Architecture.md) is about OCI
  registry credentials and is untouched, but the two must not be read as one policy.
- SolAr owns the agent version across the fleet. Upgrading every agent is a re-render, not a platform release.
- Nothing observes the agent from inside the target cluster. Stale `lastReportTime` is the only liveness signal,
  which makes the heartbeat's write path the part of the report that has to stay cheap at fleet scale.

## Open Questions

- Where are OAuth clients stored — a SolAr API resource, or a Secret per Target?
- Is client-secret rotation just a re-render, or does it need a grace window in which both secrets are valid?
- Who installs the cosign public key first? we may need to have another look at #690 and decide how we handle the key,
  and whether this should perhaps be integrated in i.e. fogctl

## Out of Scope

- `TargetReport` API type — draft surface in `pkg/agent/types.go`.
- Real chart and image (#410).
- OAuth client minting on Target creation (#407).
- The `apiserver-kit` authenticator hook — required, but its own change in its own repo.
- Capacity preflight (#409), blocked on #406.
