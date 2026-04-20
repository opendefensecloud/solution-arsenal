---
status: draft
date: 2026-04-13
---

# Registry Binding Credentials and Endpoint Rewriting

## Context and Problem Statement

ADR-009 introduces Registry and RegistryBinding but leaves their concrete fields unspecified. ADR-008 says SolAr must not handle target-side authentication. Today four problems exist:

1. **Hardcoded `regcred`** in rendered OCIRepository templates (#165)
2. **Single global push secret** (`renderer.pushSecretName`) shared by all targets
3. **No endpoint rewriting** for mirrors / air-gap copies
4. **1:1 registry assumption** — real deployments need M sources and N destinations

## Two Credential Domains

| Domain | Owner | Where | SolAr's role |
|--------|-------|-------|-------------|
| **Push** (SolAr-side) | Platform provider | Controller NS | Mounts real secret into RenderTask Job |
| **Pull** (target-side) | Cluster maintainer | Target cluster | Renders the secret **name** into manifests — never reads the data |

## Registry

Single source of truth for a registry endpoint. Shared by discovery, controllers, and rendered manifests.

Today three independent `Registry` types exist (`pkg/discovery.Registry`, `api/solar.Registry`, `api/solar/v1alpha1.Registry`) with overlapping fields but no shared code. This ADR converges them:

- **Rename `PlainHTTP` → `Insecure`** across all types and the CRD.
- **`RegistryAccess` interface** (`GetName()`, `GetEndpoint()`, `Insecure()`, `ResolveCredentials()`) lets discovery, controllers, and tests program against one contract.
- **Discovery reads Registry resources** instead of its own YAML registry list. Credentials for scanning use a `SecretRef` in the SolAr cluster.

```yaml
kind: Registry
metadata:
  name: harbor-edge
spec:
  endpoint: harbor.edge.dmz:443   # hostname:port, no scheme
  insecure: false
  caConfigMapRef: { name: root-bundle }

  discovery:
    flavor: zot
    scanInterval: 24h
    webhookPath: /webhooks/harbor-edge
    secretRef: { name: discovery-pull-creds }
```

## RegistryBinding

Links one Target to one Registry with a role. Multiple bindings compose M:N access.

```yaml
kind: RegistryBinding
metadata:
  name: edge-01-harbor-source
spec:
  targetRef:   { name: edge-cluster-01 }
  registryRef: { name: harbor-edge }
  role: source          # or "destination"

  target:
    pullSecretName: harbor-pull-cred   # rendered into manifests, never read

  secretRef:
    name: render-push-creds            # real secret, mounted into Job

  rewrite:
    sourceEndpoint: registry.corp.internal
    repositoryPrefix: mirror/
```

| Field | Role | Purpose |
|-------|------|---------|
| `targetRef` | both | Target this binding applies to |
| `registryRef` | both | Registry being made accessible |
| `role` | both | `source` or `destination`. A registry that is both needs two bindings. |
| `target.pullSecretName` | both | Secret name rendered into target manifests. Omit for anonymous pull. Solves #165. |
| `secretRef` | destination | Real secret in controller NS for RenderTask push |
| `rewrite.sourceEndpoint` | source | Discovered endpoint this binding replaces |
| `rewrite.repositoryPrefix` | source | Optional path prefix on bound Registry |

### Namespace Scoping

All references are **same-namespace `LocalObjectReference`s** (name only, no namespace field). RegistryBinding, Target, Registry, and the push Secret must reside in the same namespace. Cross-namespace references are not supported.

> **Future note:** If cross-namespace references are ever introduced, an admission webhook must enforce access (ReferenceGrant or SubjectAccessReview) to prevent Secret exfiltration via RenderTask Jobs.

## Render-Time Resolution

1. Collect all RegistryBindings for the Target.
2. **Destination:** Registry endpoint → push URL; `secretRef` → mount into Job; `target.pullSecretName` → render into Bootstrap manifests.
3. **Per source resource:** match host against `rewrite.sourceEndpoint`, rewrite to bound Registry endpoint, carry `target.pullSecretName` into resolved resource.
4. **Validation:** missing source or destination binding → fail with clear error.

Resolved resources flow into `RendererConfig` as `map[string]ResolvedResourceAccess`:

```go
type ResolvedResourceAccess struct {
    Repository     string `json:"repository"`
    Insecure       bool   `json:"insecure"`
    Tag            string `json:"tag"`
    PullSecretName string `json:"pullSecretName"`
}
```

Templates become workflow-runtime agnostic:

```yaml
{{- if $resource.pullSecretName }}
secretRef:
  name: {{ $resource.pullSecretName }}
{{- end }}
```

## Endpoint Rewriting and Signatures

Endpoint rewriting does **not** break cryptographic signatures:

- **OCI digests** are content-addressed — independent of registry URL.
- **OCM component descriptor signatures** exclude access locations from signed content.
- **Cosign signatures** reference the manifest digest, not the registry URL.
- **Helm chart provenance** signs the chart content hash, not the registry path.

**Caveat:** Cosign stores signature artifacts as OCI tags in the same registry. Mirroring pipelines must copy them alongside images (`cosign copy` or `oras copy`), or Flux's `spec.verify` will fail.

SolAr does not verify signatures — that belongs to Flux/ArgoCD on the target cluster.

## Consequences

**Positive:** solves #165; per-target push credentials; M:N registry support; endpoint rewriting for air-gap without breaking signatures; ADR-008 compliant (never reads target secrets); ADR-006 compliant (push secrets stay in controller NS); single `Registry` resource replaces three divergent types; `RegistryAccess` interface decouples consumers from concrete types.

**Negative:** role-dependent optional fields on one CRD; cluster maintainers must provision the named secret on the target; M×N bindings can produce many objects (selector-based bindings deferred per ADR-009); mirroring must copy signature artifacts; discovery becomes an API reader, adding a startup dependency.

## Decisions

- **`target.pullSecretName` is optional.** Omitting it enables anonymous pull. Templates conditionally render `secretRef` only when the field is set (see template example above).

## Open Questions

- **Rewrite matching strategy** — should `rewrite.sourceEndpoint` use exact host match, or support wildcards/regex? Exact match is simpler and sufficient for known mirrors; wildcards would reduce binding count when many source registries share a common suffix (e.g. `*.corp.internal`). Recommendation: start with exact match in v1alpha1, extend later if needed.
- **Registry scope** — should Registry be cluster-scoped (shared catalog visible to all namespaces), namespace-scoped (per-tenant isolation), or mixed via ReferenceGrants (ADR-005)? Cluster-scoped simplifies operator workflows but prevents tenant isolation; namespace-scoped requires duplication for shared registries. This interacts with multi-tenancy requirements not yet defined.
- **Signature artifact mirroring** — should this ADR mandate that mirroring pipelines copy cosign/notation artifacts, or declare it out of scope? SolAr cannot enforce this at runtime (it doesn't verify signatures), but omitting it leads to silent verification failures on targets. Recommendation: document as an operational prerequisite, not a controller-enforced invariant.
- **Discovery fallback** — when discovery starts before the API server is available (bootstrap, standalone mode), should it fall back to a static YAML registry list? This creates two sources of truth and drift risk, but may be necessary for single-binary deployments without CRDs installed.

## Relationship to Other ADRs

| ADR | Relationship |
|-----|-------------|
| 006 | `secretRef` lives in controller NS; never copied to tenants |
| 008 | `pullSecretName` is a string — SolAr stays out of target-side auth |
| 009 | Concretizes Registry/RegistryBinding fields; extends with M:N support |
