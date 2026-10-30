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
the verb `inject-target-credential` on `releases`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: solar-agent-release-admin
  namespace: test
rules:
  - apiGroups: ["solar.opendefense.cloud"]
    resources: ["releases"]
    verbs: ["inject-target-credential"]
```

Bind the role only to trusted operators. Without the verb:

- Creating a Release with the annotation, or adding it to an existing Release,
  is rejected as `Forbidden`.
- Any value other than `"true"` is rejected as `BadRequest`.
- Removing the annotation, or updating a Release that already has it, needs no
  extra permission.

## Security

- **The token is stored in the render registry.** The agent's token is part of
  the agent Release's chart, in the Target's render registry. It is stored in a
  Secret named `<release>-agent` inside the release wrapper chart, which the
  `HelmRelease` reads through `valuesFrom`. It is never in the `HelmRelease`
  spec, a `RenderTask`, a Target or any other SolAr object.
- **Scope pull credentials per namespace.** Anyone who can pull the chart can read
  the token. On a registry shared by several tenants, give each Target's pull
  credentials access to its own namespace path only (for example `<namespace>/**`
  in zot or Harbor). Do not share one pull credential across tenants.
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
- **RenderTask create is credential-equivalent.** A RenderTask's
  `spec.targetCredentialSecretRef` can name any Secret in its namespace, just
  like `pushSecretRef` and `sourceSecretRef`. Keep write access to RenderTasks
  limited to SolAr's controllers.
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
