---
status: draft
date: 2026-04-13
---

# Registry Binding Credentials

## Context and Problem Statement

ADR-009 splits Target into Target, Registry, RegistryBinding, and ReleaseBinding but leaves their concrete fields unspecified.
ADR-008 establishes that SolAr must not handle target-side authentication.

This ADR specifies the credential semantics for Registry and RegistryBinding. It solves three concrete problems:

1. **Hardcoded `regcred`** in rendered OCIRepository templates (#165)
2. **Single global push secret** — no per-registry push credentials
3. **No pull secret propagation** — templates cannot conditionally render pull credentials

### Scope relative to ADR-009

ADR-009 decided the resource split, RBAC model, and rendering interaction at a conceptual level. This ADR adds:

- The two credential domains (push vs pull) and which resource carries each
- `targetPullSecretName` on Registry, solving #165
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

## Render-Time Resolution

1. Collect all RegistryBindings for the Target.
2. **Destination:** Resolve the Target's `renderRegistryRef` to its Registry. Use `registry.spec.hostname` as push URL and `registry.spec.solarSecretRef` as push credentials for the RenderTask.
3. **Per source resource:** Match the resource's repository host against RegistryBindings. Carry the bound Registry's `targetPullSecretName` into the resolved resource.
4. **Validation:** Missing destination registry or unresolvable `renderRegistryRef` fails rendering. A resource with no matching RegistryBinding fails with a clear error.

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

**Positive:** Solves #165 with per-registry pull secret names. Per-registry push credentials via `solarSecretRef`. ADR-008 compliant — SolAr never reads target-side secrets. ADR-006 compliant — push secrets stay in controller NS. RegistryBinding stays a simple link resource.

**Negative:** Cluster maintainers must provision the named pull secret on each target.

## Decisions

- **`targetPullSecretName` is optional.** Omitting it enables anonymous pull. Templates conditionally render `secretRef` only when the field is set.
- **No explicit `role` field on RegistryBinding.** The destination registry is identified by `Target.spec.renderRegistryRef`. RegistryBindings declare source access. Role is implicit from context.
- **Credentials live on Registry, not RegistryBinding.** RegistryBinding is a pure link — no credential fields.

## Open Questions

- **Registry scope** — namespace-scoped (current) vs cluster-scoped (`ClusterRegistry`) for shared registry definitions across namespaces. Deferred to a future ADR.

## Relationship to Other ADRs

| ADR | Relationship |
|-----|-------------|
| 006 | `solarSecretRef` lives in controller NS; push secrets never copied to tenants |
| 008 | `targetPullSecretName` is a string — SolAr stays out of target-side auth |
| 009 | Concretizes credential fields on the resources defined there |
