---
status: draft
date: 2026-04-13
---

# Registry Credentials and Endpoint Rewriting

## Context and Problem Statement

ADR-009 splits Target into Target, Registry, RegistryBinding, and ReleaseBinding but leaves their concrete fields unspecified.
ADR-008 establishes that SolAr must not handle target-side authentication.

This ADR specifies the credential and endpoint rewriting semantics for Registry and RegistryBinding. It solves four concrete problems:

1. **Hardcoded `regcred`** in rendered OCIRepository templates (#165)
2. **Single global push secret** — no per-registry push credentials
3. **No endpoint rewriting** for mirrors / air-gap copies
4. **No pull secret propagation** — templates cannot conditionally render pull credentials

### Scope relative to ADR-009

ADR-009 decided the resource split, RBAC model, and rendering interaction at a conceptual level. This ADR adds:

- The two credential domains (push vs pull) and which resource carries each
- `targetPullSecretName` on Registry, solving #165
- Endpoint rewriting fields on RegistryBinding
- The render-time resolution algorithm and `ResolvedResourceAccess` output

This ADR does **not** cover: Discovery integration (Discovery remains independent of the Kubernetes API resources), ReleaseBinding semantics, Profile interactions, or signature verification (out of scope — handled by Flux/Kyverno on the target cluster).

## Two Credential Domains

| Domain | Owner | Lives on | SolAr's role |
|--------|-------|----------|-------------|
| **Push** (SolAr-side) | Platform provider | `Registry.spec.solarSecretRef` | Mounts real Secret into RenderTask Job |
| **Pull** (target-side) | Cluster maintainer | `Registry.spec.targetPullSecretName` | Renders the name into manifests — never reads the data |

Both credential references live on **Registry**, not RegistryBinding. This follows from ADR-009's design: Registry is the single source of truth for a registry endpoint, and RegistryBinding is a pure link resource.

## Registry

This ADR adds two credential fields to the Registry spec defined in ADR-009:

```yaml
kind: Registry
metadata:
  name: harbor-edge
spec:
  hostname: harbor.edge.dmz:443
  plainHTTP: false
  solarSecretRef:                        # LocalObjectReference — real Secret in same NS
    name: render-push-creds              # mounted into RenderTask Job for pushing
  targetPullSecretName: harbor-pull-cred # string — rendered into manifests, never read
```

| Field | Type | Purpose |
|-------|------|---------|
| `solarSecretRef` | `LocalObjectReference` | Secret in controller NS with push credentials. Passed to `RenderTask.spec.pushSecretRef`. Required for registries used as render destinations. |
| `targetPullSecretName` | `string` | Name rendered into target manifests (e.g. Flux `OCIRepository.spec.secretRef.name`). The actual Secret is provisioned on the target by the cluster maintainer. Omit for anonymous pull. Solves #165. |

`solarSecretRef` is a reference-by-name to a `v1.Secret` in the same namespace — the same pattern used by `RenderTask.spec.pushSecretRef` today.

## RegistryBinding: Endpoint Rewriting

RegistryBinding remains a link resource per ADR-009 (`targetRef` + `registryRef`). This ADR adds optional **rewrite** fields for mirror and air-gap scenarios:

```yaml
kind: RegistryBinding
metadata:
  name: edge-01-harbor-source
spec:
  targetRef:   { name: edge-cluster-01 }
  registryRef: { name: harbor-edge }
  rewrite:
    sourceEndpoint: registry.corp.internal    # discovered endpoint to replace
    repositoryPrefix: mirror/                 # optional path prefix on bound registry
```

| Field | Purpose |
|-------|---------|
| `rewrite.sourceEndpoint` | When a resource's repository matches this host, rewrite to the bound Registry's endpoint. Exact host match. |
| `rewrite.repositoryPrefix` | Prepended to the repository path after rewriting. |

Rewriting is per-target-per-registry because different targets may mirror different sources through the same registry.

A RegistryBinding without `rewrite` simply declares that the target has access to the bound registry at its original endpoint.

## Render-Time Resolution

1. Collect all RegistryBindings for the Target.
2. **Destination:** Resolve the Target's `renderRegistryRef` to its Registry. Use `registry.spec.hostname` as push URL and `registry.spec.solarSecretRef` as push credentials for the RenderTask.
3. **Per source resource:** Match the resource's repository host against `rewrite.sourceEndpoint` on each RegistryBinding. If matched, rewrite to the bound Registry's endpoint (with `repositoryPrefix`). Carry the bound Registry's `targetPullSecretName` into the resolved resource.
4. **No rewrite match:** Use the resource's original endpoint. If a RegistryBinding exists for that host (without rewrite), use its Registry's `targetPullSecretName`.
5. **Validation:** Missing destination registry or unresolvable `renderRegistryRef` fails rendering. A resource with no matching RegistryBinding fails with a clear error.

Resolved resources flow into `RendererConfig` as:

```go
type ResolvedResourceAccess struct {
    Repository     string `json:"repository"`
    Insecure       bool   `json:"insecure"`
    Tag            string `json:"tag"`
    PullSecretName string `json:"pullSecretName,omitempty"`
}
```

Templates become:

```yaml
spec:
  url: oci://{{ $resource.repository }}
  {{- if $resource.pullSecretName }}
  secretRef:
    name: {{ $resource.pullSecretName }}
  {{- end }}
```

This replaces the hardcoded `regcred` (#165).

## Consequences

**Positive:** Solves #165 with per-registry pull secret names. Per-registry push credentials via `solarSecretRef`. Endpoint rewriting enables air-gap and mirror deployments. ADR-008 compliant — SolAr never reads target-side secrets. ADR-006 compliant — push secrets stay in controller NS. RegistryBinding stays a simple link resource.

**Negative:** Rewrite matching adds rendering complexity. Cluster maintainers must provision the named pull secret on each target. Exact-match rewriting may require many bindings when many source registries share a suffix (wildcard matching deferred).

## Decisions

- **`targetPullSecretName` is optional.** Omitting it enables anonymous pull. Templates conditionally render `secretRef` only when the field is set.
- **Rewrite uses exact host match.** Wildcard or regex matching is deferred — start simple, extend if needed.
- **No explicit `role` field on RegistryBinding.** The destination registry is identified by `Target.spec.renderRegistryRef`. RegistryBindings declare source access, optionally with rewrite. Role is implicit from context.
- **Credentials live on Registry, not RegistryBinding.** RegistryBinding is a pure link with optional rewrite — no credential fields.

## Open Questions

- **Registry scope** — namespace-scoped (current) vs cluster-scoped (`ClusterRegistry`) for shared registry definitions across namespaces. Deferred to a future ADR.
- **Rewrite for non-host dimensions** — should rewriting also support tag or digest transforms, or is host + prefix sufficient?

## Relationship to Other ADRs

| ADR | Relationship |
|-----|-------------|
| 006 | `solarSecretRef` lives in controller NS; push secrets never copied to tenants |
| 008 | `targetPullSecretName` is a string — SolAr stays out of target-side auth |
| 009 | Concretizes credential fields and adds endpoint rewriting to the resources defined there |
