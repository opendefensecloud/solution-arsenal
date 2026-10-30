---
status: accepted
date: 2026-10-29
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

| Reported state | Read from       | Condition / field                                                                                                                                                                                                                                                                                           |
| -------------- | --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Ready          | both            | `Ready=True` on both                                                                                                                                                                                                                                                                                        |
| Verified       | `OCIRepository` | `SourceVerified=True` — the result of cosign `spec.verify`. Distinct from Ready: an unverified artifact is a trust failure, not a fetch failure                                                                                                                                                             |
| Progressing    | both            | `Reconciling=True` (`HelmRelease` marks it with reason `Progressing`), `Ready=Unknown`, or no condition current for this generation. Ranks **below** `Degraded`: Flux holds `Reconciling=True` while it retries a failed release, so checking it first would report a broken release as progressing forever |
| Degraded       | both            | `Ready=False`, retry still coming — on `OCIRepository` that means no `Stalled`; on `HelmRelease` it means `Failures`/`InstallFailures`/`UpgradeFailures` below the configured `retries`                                                                                                                     |
| Failed         | `OCIRepository` | `Ready=False` with `Stalled=True` — terminal, no retry coming. **`HelmRelease` never sets `Stalled` on itself**; helm-controller only reads it off the source it depends on                                                                                                                                 |
| Failed         | `HelmRelease`   | no terminal condition exists. Derived: `Remediated=True` with `InstallFailures`/`UpgradeFailures` at the chart's `remediation.retries: 3`                                                                                                                                                                   |
| Rolled back    | `HelmRelease`   | `Remediated=True` (reason `RollbackSucceeded`). The release can be live at its _previous_ version while `Ready=True`; reporting that as plain Ready hides a failed upgrade, and it is the signal #587 needs                                                                                                 |
| Test failed    | `HelmRelease`   | `TestSuccess=False` — the bootstrap chart sets `test.enable: true`, so tests run and, unless `ignoreTestFailures`, a test failure triggers remediation                                                                                                                                                      |
| Drifted        | `HelmRelease`   | `Drifted=True` (reason `DriftDetected`) — a real condition, not just events. The chart enables `driftDetection`, and this is distinct from not-Ready                                                                                                                                                        |
| Pending        | neither         | agent-derived, not a Flux condition: no pair exists yet for a bound Release, i.e. the applied bootstrap chart is older than the ReleaseBinding set. Distinguished from Failed via `Target.status.bootstrapVersion`                                                                                          |

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
time the agent runs its `Target` already exists, and the credential naming it was rendered into the same artifact that
delivered the agent.

| Step                                                                                              | Owner                    |
| ------------------------------------------------------------------------------------------------- | ------------------------ |
| Create the `Target`                                                                               | user                     |
| Create its ServiceAccount, Role, RoleBinding and empty `TargetReport`, mint a token into a Secret | solar-controller-manager |
| Render `solar-agent` into the Target's bootstrap output, token by reference                       | solar-renderer           |
| Pull and install                                                                                  | Flux on the target       |
| Report with the token                                                                             | the agent                |

- **Identity**: one ServiceAccount per `Target`, in the Target's namespace on the SolAr cluster. This is what
  [#407](https://github.com/opendefensecloud/solution-arsenal/issues/407) becomes, now that no bootstrap token has to
  be distributed by anyone but SolAr.
- **Delivery**: the renderer Job mounts the token Secret, which the controller derives from the owning Target and checks
  by labels carrying the Target's name and UID (labels, not an ownerReference; see _Delete_ below). It is never a
  free-form `SecretRef` on the RenderTask, and never inline in `BootstrapInput`, `Userdata` or HelmRelease values:
  `RenderTask.spec` lives in SolAr's etcd and is copied into the `config.json` Secret, and `userdata` fans out to every
  HelmRelease. The bootstrap chart emits a Secret in the agent namespace with two keys: a kubeconfig (server URL and CA
  of the SolAr cluster's kube-apiserver, `tokenFile`) and the token. The agent chart takes it as
  `apiserverKubeconfig.existingSecret` but today projects only the kubeconfig key; it must project the token key too
  (#410). Flux updates that Secret in place on every re-render and client-go re-reads the token file, so a new token
  needs no agent restart. Only the token Secret's name enters `BootstrapInput`, and every rotation writes a new Secret,
  so a rotation changes the render input and every bootstrap tag maps to exactly one token.
- **Liveness**: the agent does not monitor itself. It arrives as an `OCIRepository`/`HelmRelease` pair like every
  other Release, so a failure of its own pair silences it. A `Target` whose `lastReportTime` has gone stale is
  therefore assumed broken, whatever the cause.

### Cross-cluster auth: a ServiceAccount token per Target

The agent writes its `TargetReport` through the SolAr cluster's kube-apiserver, like every other client of the
aggregated API; there is no dedicated agent endpoint. That kube-apiserver authenticates the request and strips
`Authorization` before forwarding it, and solar-apiserver authorizes it with a delegated SubjectAccessReview. So only
credentials the SolAr cluster itself trusts work end to end: ServiceAccount tokens and client certificates from its
own signer.

ServiceAccount tokens were first planned with self-registration: each target cluster would have been rolled out with
an initial token to fetch its real credential from SolAr, which meant work by the infra team on every install. With
the agent rendered into the bootstrap that work is gone: the controller mints the token and renders it in.

A SolAr-owned OIDC issuer, per-Target keys verified by solar-apiserver, and static tokens stored as a hash are
dropped. The SolAr cluster's kube-apiserver does not know any of them, so each would need a dedicated agent endpoint
and an authenticator hook in `apiserver-kit`, for no gain over a ServiceAccount token.

- **Authorization**: plain RBAC. A Role per Target allows `get` on its own `Target` and `get`/`update` on its own
  `TargetReport`, both by `resourceNames`. RBAC cannot limit `create` to one name, so the controller creates the empty
  `TargetReport` and the agent never needs `create`; a missing report is recreated when the controller next reconciles
  the Target, and the agent's create fallback in `KubePublisher` goes away (#407). solar-apiserver gets no authorizer
  or admission code, and the in-cluster components keep their ServiceAccount tokens unchanged.
- **Token**: minted with the TokenRequest API and bound to the Secret that stores it, so deleting the Secret
  invalidates the token. The controller always sets `expirationSeconds` (TokenRequest defaults to one hour), as long as
  the SolAr cluster allows. Where `--service-account-max-token-expiration` caps it, the controller mints a new token and
  re-renders before expiry.
- **Lifecycle**: _rotate_ is on demand, by annotation on the Target: a new token in a new Secret and a re-render. The
  old Secret is deleted once the agent confirms the new token, not after a fixed grace period: an offline edge target
  may only pick it up with its next day-2 update. `Target.status.bootstrapVersion` only counts renders, so the renderer
  passes the bootstrap version into the agent's values and the agent reports it in its `TargetReport`; a running agent
  on the new version proves the new token arrived. _Revoke_ deletes every token Secret of the Target, including one kept
  for an unconfirmed rotation, takes effect within about a minute and does not re-mint; re-issuing is explicit.
  Re-renders of a revoked Target leave the agent credential out, so releases still roll out and only the agent goes
  silent. _Delete_ removes the ServiceAccount, Role, RoleBinding, every token Secret and `TargetReport` in the Target
  finalizer, not through ownerRef garbage collection: an empty SolAr etcd would otherwise let the garbage collector
  delete every agent identity in the fleet.

### Credential storage: a Secret in the SolAr cluster, nothing in SolAr's etcd

SolAr's etcd is a single replica, plaintext on the wire, unencrypted at rest and has no backup. It holds nothing
secret: the Target and its `TargetReport` carry no credential.

Every bootstrap re-render needs the token again, so a secret store is unavoidable. That store is one Secret per Target
in the Target's namespace on the SolAr cluster, which inherits whatever encryption at rest, RBAC, audit and backup the
SolAr cluster has; operators are pointed at enabling them. The controller already reads and writes Secrets there. It
additionally needs `create` on `serviceaccounts/token`, ServiceAccounts, Roles and RoleBindings, and must itself hold
the permissions it grants.

Rejected:

- a client secret on a SolAr resource: a usable bearer secret in an etcd that was never meant to hold one;
- a client secret hashed on a SolAr resource: every re-render needs the plaintext, so it ends up in a Secret anyway;
- one central signing key: a fleet-wide forging key and no per-Target revocation without extra machinery;
- SolAr as a small CA: the SolAr cluster's kube-apiserver only trusts its own client CA, so SolAr would need that
  CA's key;
- SPIFFE/SPIRE: per-target-cluster infrastructure, the SolAr cluster must trust the SPIRE CA, and without a route to
  the target it starts from a join token in the bootstrap anyway.

### Next step: agent-generated keys via the CSR API

A ServiceAccount token per Target is the answer for single-tenant edge installs, where Targets in a namespace sharing
one pull credential is acceptable. Fog and multi-tenant installs, and any install where `PreflightReady` gates
rollouts, need the private credential to never sit in the render registry. For those the agent generates its key on
the target cluster and enrolls once through the CSR API with a one-time token, the way kubelets bootstrap their client
certificates. The controller approves only that Target's subject; the Role and the pre-created `TargetReport` stay,
and only the RoleBinding's subject changes to the certificate user.

Its prerequisites are open per install: the SolAr cluster must sign `kube-apiserver-client` certificates (kubeadm, RKE2
and k3s do), mTLS must reach the kube-apiserver without TLS termination in between, the certificate user needs
permission to request its own renewal, the agent must keep its key across restarts, and the approver can mint any user
of the SolAr cluster, so its checks are security-critical. The one-time token must reach only its own Target (scoped
pulls or out of band), or a registry reader enrolls first; a failed legitimate enrollment must raise an alert.

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
- The SolAr cluster's kube-apiserver must be reachable from target networks, under a name its serving certificate
  covers. That is a new install requirement; its external URL and CA become install configuration and are rendered
  into the agent's kubeconfig.
- Each agent holds an identity of the SolAr cluster. RBAC limits it to its own Target and `TargetReport`, but it is a
  credential of that cluster living on another one. [ADR-008](./008-No-Auth-Architecture.md) is about OCI registry
  credentials and is untouched, but the two must not be read as one policy.
- Whoever can read Secrets, or create Pods or Jobs, in a Target's namespace on the SolAr cluster holds that Target's
  agent identity.
- Tokens are unique per Target, but each one ships inside its Target's bootstrap in the render registry, so a token is
  only as confidential as that bootstrap. `targetPullSecretName` is per `Registry`, not per Target, and SolAr never
  sees what the Secret behind it holds. Unless the pull credential is scoped to the Target's namespace, a sibling
  cluster can read another Target's token and forge its reports; rotation and revocation do not evict a registry
  reader. On single-tenant edge installs this is accepted. Multi-tenant installs must scope pull credentials per
  namespace. Anonymous pull on a render registry that delivers agents is not supported. `PreflightReady` must not gate
  rollouts until pulls are scoped per Target or the agent enrolls its own key.
- A credential delivered by re-render depends on bootstrap tags only ever increasing, across Target recreation and
  SolAr etcd loss.
- SolAr owns the agent version across the fleet. Upgrading every agent is a re-render, not a platform release.
- Nothing observes the agent from inside the target cluster. Stale `lastReportTime` is the only liveness signal,
  which makes the heartbeat's write path the part of the report that has to stay cheap at fleet scale.

## Out of Scope

- `TargetReport` API type — draft surface in `pkg/agent/types.go`.
- Real chart and image (#410).
- ServiceAccount, token minting, rotation and revocation, and the bootstrap version in the `TargetReport` (#407).
- Agent-generated keys via the CSR API — the direction is set above, the design follows when fog installs need it.
- Scheduled rotation.
- Capacity preflight (#409), blocked on #406.
