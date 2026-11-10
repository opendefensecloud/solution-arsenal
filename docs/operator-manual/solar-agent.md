# Setting up the solar-agent

This page is for operators of a SolAr installation. It covers how to switch on
agent credentials, how to publish the agent to Targets, and how to rotate or
revoke a credential. For the user side, see [Agent
Registration](../user-guide/agent-registration.md).

## Enable agent credentials

Agent credentials are off by default. Set these Helm values for the `solar`
chart:

| Value | Default | Meaning |
| ----- | ------- | ------- |
| `controller.agent.apiserverURL` | empty | URL of the SolAr cluster's kube-apiserver, as the target clusters reach it. Empty switches the feature off. |
| `controller.agent.apiserverCAFile` | `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt` | CA bundle for that URL, written into each agent's kubeconfig. |
| `controller.agent.tokenTTL` | `8760h` | Lifetime requested for each token. Values below `10m` are rejected at startup. |
| `controller.agent.reportStaleAfter` | `10m` | How old a `TargetReport` may get before the Target shows `AgentReporting=False`. See [Agent liveness](#agent-liveness). |

```yaml
controller:
  agent:
    apiserverURL: https://solar.example.com:6443
```

The default CA file is the cluster CA. It is correct only if target clusters
reach the kube-apiserver without a TLS-terminating proxy. Otherwise, point
`apiserverCAFile` at the CA that signs the certificate they see.

The agent talks to `solar-apiserver` through the kube-apiserver (API
aggregation). The kube-apiserver checks the token, and `solar-apiserver`
authorizes the request with the Role the controller created. No extra
authentication setup is needed.

When `apiserverURL` is empty, the controller creates no identity and no token.
Each Target shows `AgentCredentialReady=False` with reason `Disabled`.
It also creates no `TargetReport`. An agent you run with your own kubeconfig
(`apiserverKubeconfig.existingSecret`) only updates its report, so create the
`TargetReport` yourself, named like the Target. Until then the agent logs `NotFound`.
Annotated Releases still render, but without a credential.

The kube-apiserver may cap token lifetimes with
`--service-account-max-token-expiration`. The expiry that the cluster returns is
the one that counts.

## Catalog setup

The agent is an ordinary catalog component, `opendefense.cloud/solar-agent`. The
released component is built and published from the `ocm-components` repository.
For dev clusters, `make ocm-agent-component` builds a local copy from
`test/fixtures/solar-agent`. See [OCM packaging](../user-guide/ocm-packaging.md#the-solar-agent-component).

Once discovery has created the `ComponentVersion`, create one `Release` for it.
The annotation `solar.opendefense.cloud/target-credential: "true"` tells SolAr to
render each Target's token into this Release's chart. Only the value `"true"` is
valid.

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Release
metadata:
  name: solar-agent
  namespace: test
  annotations:
    solar.opendefense.cloud/target-credential: "true"
spec:
  componentVersionRef:
    name: opendefense-cloud-solar-agent-v1-0-0  # name of your ComponentVersion
```

Then create one `Profile` with an empty `targetSelector`. An empty selector
matches every Target in the namespace, so each new Target gets a
`ReleaseBinding` for the agent.

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Profile
metadata:
  name: solar-agent
  namespace: test
spec:
  releaseRef:
    name: solar-agent
  targetSelector: {}
```

Do this once per namespace that holds Targets.

## RBAC for the annotation

Anyone who can write Releases could otherwise get a Target's token rendered into
a chart they control. The admission plugin `SolarTargetCredential` in
`solar-apiserver` prevents this. To add the annotation to a Release, a user needs
the verb `inject-target-credential` on `releases`.

The `solar` chart ships a ClusterRole with only that verb,
`<fullname>-target-credential-injector` (`solar-target-credential-injector` for
a Helm release named `solar`). It is bound to nobody. Bind it to the Solar
Operator in each namespace that holds the agent Release:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: solar:target-credential-injector
  namespace: test
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: solar-target-credential-injector
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: solar-operator
```

Bind it only to trusted operators. Without the verb:

- Creating a Release with the annotation, or adding it to an existing Release,
  is rejected as `Forbidden`.
- Any value other than `"true"` is rejected as `BadRequest`.
- Changing the spec of a Release that has the annotation is rejected as
  `Forbidden` (see [Security](#security)).
- Removing the annotation, or a metadata-only update (labels, other
  annotations) of a Release that has it, needs no extra permission.

## Agent liveness

The agent updates its `TargetReport` whenever something changes, and at least
every `--max-report-age` as a heartbeat (default 10 times its 30s interval, so
5m). The Target shows whether that still happens:

| `AgentReporting` | Reason | Meaning |
| ---------------- | ------ | ------- |
| `True` | `Reporting` | The agent wrote its report within `controller.agent.reportStaleAfter`. |
| `Unknown` | `NoReport` | The report was pre-created, but the agent has not written it yet. |
| `False` | `Stale` | The agent has not written its report for longer than `controller.agent.reportStaleAfter`. |

The UI shows a Target with `AgentReporting=False` as `Agent not reporting`.

The default threshold of `10m` is twice the default heartbeat. The heartbeat is
10 times the agent chart's `interval` unless `--max-report-age` is set (chart
value `extraArgs`). If you raise either, keep `reportStaleAfter` well above the
resulting heartbeat, or quiet Targets flap to `Stale`: with `interval: 1m` the
heartbeat is already `10m`.

Staleness is measured by the SolAr controller's clock, from when it saw the
agent's last write, so a wrong clock on the target cluster doesn't matter. It
only skews the time of the last report the UI shows, which the agent stamps.
After a controller restart, a silent agent is marked `Stale` up to one
threshold late.

If you remove the agent from a Target on purpose, revoke its credential (see
[Revoke a credential](#revoke-a-credential)) and delete its `TargetReport`.
Otherwise the Target keeps showing `Agent not reporting`. While credentials are
enabled and not revoked, the controller re-creates a deleted report, and the
Target shows `NoReport` again.

### Which Flux objects the agent reports

The agent reads the Flux `OCIRepository`/`HelmRelease` pairs of its own Target
only. An agent delivered through SolAr watches the bootstrap namespace, where
the bootstrap chart creates those pairs, even if its Release installs it
elsewhere with `spec.targetNamespace`. Other Targets' pairs on the same cluster
are therefore not reported as this Target's.

To override this, set `watchNamespace` in the agent Release's `values`: a
namespace name to watch that namespace instead, or `"*"` to watch all
namespaces, which also reports pairs that belong to other Targets on the same
cluster. An agent installed by hand with the chart (no `watchNamespace`) watches
its own release namespace.

The chart grants read access to match. The agent lists `OCIRepository` and
`HelmRelease` objects through a Role and RoleBinding in the watched namespace
(a ClusterRole and ClusterRoleBinding with `"*"`), and nodes and pods
cluster-wide through a ClusterRole for its capacity check.

## Security

- **The token is stored in the render registry.** The agent's token is part of
  the agent Release's chart, in the Target's render registry. It is stored in a
  Secret named `<release>-agent` inside the release wrapper chart, which the
  `HelmRelease` reads through `valuesFrom`. It is never in the `HelmRelease`
  spec, a `RenderTask`, a Target or any other SolAr object.
- **The namespace is the isolation boundary.** Anyone who can pull the chart can
  read the token. All Targets in a namespace share one chart repository
  (`<namespace>/...`), so a pull credential for that path can read the token of
  every Target in the namespace. On a registry shared by several tenants, give
  each Target's pull credentials access to its own namespace path only (for
  example `<namespace>/**` in zot or Harbor). Do not share one pull credential
  across tenants. Targets that must not read each other's tokens belong in
  separate namespaces.
- **What the token can do.** It acts as a ServiceAccount that may `get` one
  `Target` and `get` and `update` one `TargetReport`. It cannot create objects and
  cannot reach another Target.
- **The controller's own rights.** To create these Roles, the controller's
  ClusterRole must hold the same rights. It can also create `serviceaccounts/token`
  cluster-wide, and it writes Secrets in Target namespaces. Treat the controller
  as highly privileged and limit who can change its deployment.
- **Changing an annotated Release needs the verb too.** Changing the spec of a
  Release that carries the `target-credential` annotation also requires
  `inject-target-credential`. The Release's own values override the agent
  connection values in the rendered chart (Flux merges `spec.values` after
  `valuesFrom`), so whoever edits them could change where an agent sends its
  token. Only users with the verb can do that. Metadata-only updates (labels,
  other annotations) stay free.
- **RenderTasks are gated too.** Creating a RenderTask that sets
  `spec.targetCredentialSecretRef` or `releaseConfig.agentCredential`, or
  changing the spec of one that does, requires `inject-target-credential` on
  `rendertasks`. Only the controller-manager holds it. Before rendering, the
  RenderTask controller also checks that the Secret is a credential of the
  Target that owns the task. `pushSecretRef` and `sourceSecretRef` are not
  checked this way, so still keep write access to RenderTasks limited to SolAr's
  controllers.
- **Cross-namespace bindings share the token.** An annotated Release in a
  provider namespace that is bound to a tenant's Target (ReleaseBinding plus
  ReferenceGrant) receives that Target's token. Tenants should only grant
  cross-namespace access to providers they trust with it.
- **Revocation is not instant.** A revoked token stops working within about a
  minute, because of token and authentication caches.

## Runbook

All commands run on the SolAr cluster.

### Rotate a credential

```bash
kubectl annotate target cluster-1 -n test \
  solar.opendefense.cloud/rotate-agent-credential=$(date +%s) --overwrite
```

Any new value triggers one rotation. The controller then:

1. Mints a new token in a new Secret (`solar-agent-<target>-<n+1>`).
2. Re-renders the agent chart with a new tag, so Flux updates the target.
3. Deletes the old Secrets only after the `TargetReport` shows that the agent
   release runs the new revision. The old token stays valid until then.

The controller also rotates on its own when less than 20% of a token's lifetime
remains. No annotation is needed for that.

### Offline targets

An offline target keeps the old token until it can pull the new chart. The old
token is valid until it expires. If the target stays offline longer than the
token lifetime, the agent loses its identity until the next update. Other
Releases are not affected. If the SolAr cluster caps token lifetimes, that cap
sets the limit. Choose `tokenTTL` with the longest expected outage in mind.

### Revoke a credential

```bash
kubectl annotate target cluster-1 -n test \
  solar.opendefense.cloud/agent-credential=revoked --overwrite
```

The controller deletes all credential Secrets and mints no new ones. The agent
Release renders without a credential, so the agent runs in standalone mode. Other
Releases keep rendering. The ServiceAccount, Role and RoleBinding stay in place.
The condition shows `AgentCredentialReady=False`, reason `Revoked`.

To lift the revocation, remove the annotation. The controller mints a fresh
credential.

```bash
kubectl annotate target cluster-1 -n test solar.opendefense.cloud/agent-credential-
```

### Delete a Target

Deleting a Target removes its ServiceAccount, Role, RoleBinding, `TargetReport`
and credential Secrets.
