---
status: draft
date: 2026-03-24
---

# Authentication

## Motivation

SolAr interacts with OCI registries at every stage of its pipeline. Each stage requires credentials, and each has its own shortfalls in how those credentials are managed today.

### 1. Discovery — scanning the source registry

The `Discovery` resource carries a `spec.registry.secretRef` pointing to a Kubernetes Secret with `username`/`password` credentials. The discovery controller reads this Secret and passes the credentials to the discovery worker.

**Shortfalls:**
- The Secret must be pre-created in the Discovery's namespace by the user. There is no lifecycle management or rotation support.
- `Discovery.spec.registry.caConfigMapRef` provides CA trust for the discovery worker, but this trust configuration is not forwarded to downstream consumers (rendering, Flux).

### 2. RenderTask — pushing rendered charts to the render registry

The RenderTask controller creates a Job that must push rendered Helm charts to an OCI registry. The Job needs to mount the platform operator's push credentials, but Kubernetes restricts Pods to mounting Secrets from their own namespace. The current implementation copies the operator's auth Secret from the controller namespace into the tenant's namespace (`copyAuthSecret()`), exposing operator credentials to tenants.

**Shortfalls:**
- Operator-owned credentials are leaked into tenant namespaces.
- Addressed architecturally by [ADR-006 / #315](../006-Move-RenderTask-Jobs-to-Dedicated-Namespace.md) (make RenderTask cluster-scoped, run Jobs in controller namespace). The credential isolation problem is solved there, but the underlying question of how the push credential itself is provisioned and rotated remains.

### 3. Flux — pulling charts on target clusters

When SolAr renders a Release, it produces Flux `OCIRepository` and `HelmRelease` resources that are applied to target clusters. The `OCIRepository` needs a `spec.secretRef` to authenticate against the source or render registry when pulling charts.

The `SecretRef` propagation (#187) threads the Secret name from `Discovery.spec.registry.secretRef` through the pipeline into the rendered `OCIRepository`. However, the referenced Secret **must already exist in the target namespace** on the target cluster. Nothing in SolAr creates, syncs, or manages this Secret.

**Shortfalls:**
- The Secret must be manually created in every namespace on every target cluster where Flux resources land.
- There is no propagation mechanism; missed namespaces silently break.
- CA trust from Discovery is not forwarded to Flux `OCIRepository` resources.

### Summary of shortfalls across all stages

| Problem | Affects | Description |
|---|---|---|
| **No cross-namespace distribution** | Rendering, Flux | Secrets must be manually created in every namespace that needs them. Does not scale for fleet deployments. |
| **No credential rotation** | Discovery, Rendering, Flux | Updating credentials requires manually updating Secrets everywhere. Missed copies silently break. |
| **No multi-cluster story** | Flux | Each target cluster needs its own copy of pull credentials. No distribution or sync mechanism exists. |
| **Operator credentials leaked to tenants** | Rendering | `copyAuthSecret()` copies operator push credentials into tenant namespaces. (Solved by ADR-006/#315.) |
| **Opaque failure mode** | Flux | Missing or stale Secrets cause OCI pull errors with no SolAr-side feedback about the root cause. |
| **Single registry per component** | Discovery, Flux | All resources in a ComponentVersion get the same SecretRef. Multi-registry components are not supported. |
| **CA trust not propagated** | Rendering, Flux | `caConfigMapRef` is consumed during discovery but not forwarded to rendering Jobs or Flux resources on targets. |

## Option: External Secrets Operator (ESO)

### How it works

The [External Secrets Operator](https://external-secrets.io/) is a Kubernetes operator that synchronises secrets from an external backend into Kubernetes Secrets. It introduces two key CRDs:

- **ClusterSecretStore** — cluster-wide configuration pointing at a secret backend (configured once per cluster)
- **ExternalSecret** — per-namespace resource that tells ESO to create/refresh a Kubernetes Secret from the backend

ESO can serve all three stages of SolAr's pipeline:

```text
                          ClusterSecretStore (configured once per cluster)
                                    |
            +-----------------------+-----------------------+
            |                       |                       |
   ExternalSecret              ExternalSecret          ExternalSecret
   (discovery namespace)       (controller namespace)  (target namespace)
            |                       |                       |
            v                       v                       v
   K8s Secret                  K8s Secret              K8s Secret
   (discovery pull creds)      (render push creds)     (Flux pull creds)
            |                       |                       |
            v                       v                       v
   Discovery worker            RenderTask Job           Flux OCIRepository
```

### ESO with Kubernetes provider (no external KMS)

ESO's `kubernetes` provider can read a Secret from one namespace and mirror it to others. No external infrastructure needed — a Secret in the controller namespace serves as the single source of truth:

```text
Secret in solar-system (source of truth)
  -> ClusterSecretStore (kubernetes provider, reads from solar-system)
    -> ExternalSecret per namespace that needs credentials
      -> K8s Secret (auto-created, refreshed on interval)
```

This solves cross-namespace distribution without introducing external dependencies beyond ESO itself. Platform teams deploy ESO, create one `ClusterSecretStore`, and create `ExternalSecret` resources where needed:

- **Discovery namespace** — ESO provisions the Secret referenced by `Discovery.spec.registry.secretRef`.
- **Controller namespace** — ESO provisions the render push credential. After ADR-006/#315, the Job runs in the controller namespace, so the Secret stays local — no cross-namespace copy needed.
- **Target namespaces** — ESO provisions the Secret referenced by `ResourceAccess.SecretRef` for Flux `OCIRepository`.

### ESO with KMS provider

When a Key Management Service is available, the `ClusterSecretStore` points at the KMS instead of a Kubernetes Secret. This upgrades the same ESO pattern with additional capabilities that are **out of scope for SolAr**:

| KMS capability | Type | Why out of scope |
|---|---|---|
| Automatic credential rotation with audit trail | Security requirement | KMS rotation policies are a platform/compliance concern. SolAr consumes the resulting Secret; it does not control rotation schedules. |
| IAM / Vault access policies per cluster | Security requirement | Which clusters may read which registry credentials is an infrastructure access control decision. |
| Disaster recovery (secrets reconstructable from KMS) | Convenience | Reduces dependency on etcd backups. A platform resilience concern, not an application concern. |
| Centralised audit log (who accessed what, when) | Security requirement (regulated environments) | Compliance logging is a platform obligation. SolAr does not participate in the audit chain beyond referencing a Secret by name. |
| Scoped credentials per target cluster | Security requirement | Different clusters getting different credentials (e.g. prod vs. staging) is an infrastructure segmentation decision. |

SolAr's `SecretRef` fields are the boundary. Everything behind them — ESO backend choice, KMS provider, rotation policies, IAM — is the platform team's domain.

### What SolAr needs to change

Nothing. SolAr references Secrets by name at each stage (`Discovery.spec.registry.secretRef`, `ResourceAccess.SecretRef`, render auth Secret). Whether those Secrets are created manually, by ESO with a Kubernetes provider, or by ESO with a KMS provider is transparent to SolAr.

#### Secret structure requirements

While SolAr does not dictate how Secrets are provisioned, ESO-generated Secrets must match the structure expected by SolAr controllers:

- **Discovery credentials**: Must contain `username` and `password` data keys (type `Opaque` or `kubernetes.io/basic-auth`)
- **RenderTask push credentials**: Must be either:
  - Type `kubernetes.io/basic-auth` with `username` and `password` keys, or
  - Type `kubernetes.io/dockerconfigjson` with `.dockerconfigjson` key
- **Flux pull credentials**: Consumed by Flux controllers; see [Flux OCIRepository secret reference documentation](https://fluxcd.io/flux/components/source/ocirepositories/#secret-reference)

Platform teams configuring ESO `ExternalSecret` resources must ensure the `data` or `dataFrom` fields produce these keys.

### What the platform team does

1. Deploy ESO to the management cluster and each target cluster.
2. Create a `ClusterSecretStore` (once per cluster) — pointing at either a Kubernetes Secret in the controller namespace or an external KMS.
3. Create `ExternalSecret` resources in each namespace that needs credentials: discovery namespace, controller namespace (for rendering), and target namespaces (for Flux).

For fleet-scale deployments, the per-namespace `ExternalSecret` resources can be templated via ArgoCD ApplicationSets, Flux Kustomizations, or similar GitOps tooling.

## Problem vs. Solution Matrix

| Problem | Stage | Current (plain Secrets) | With ESO (no KMS) | With ESO + KMS |
|---|---|---|---|---|
| **Secrets must be pre-created manually** | All | `kubectl create secret` per namespace per cluster | Automatic via `ExternalSecret` syncing from a source Secret | Automatic via `ExternalSecret` syncing from KMS |
| **Cross-namespace distribution** | Rendering, Flux | Manual copy to each namespace | ESO mirrors from one source to N namespaces | ESO mirrors from KMS to N namespaces |
| **Multi-cluster distribution** | Flux | Manual copy to each target cluster | ESO per cluster reads from shared source (needs cross-cluster access or replicated source) | ESO per cluster reads from same KMS independently |
| **Credential rotation** | All | Manual update everywhere; missed copies silently break | Semi-automatic: update source Secret, ESO propagates within refresh interval | Fully automatic: KMS rotates, ESO syncs, no human step |
| **Operator credentials leaked to tenants** | Rendering | `copyAuthSecret()` copies to tenant namespace | Solved by ADR-006/#315; ESO provisions the Secret in the controller namespace only | Same as ESO only |
| **Opaque failure on missing/stale Secret** | Flux | OCI pull error, no SolAr-side feedback | `ExternalSecret` status shows sync errors (SolAr does not surface these, but operators can monitor) | Same as ESO only |
| **CA trust not propagated** | Rendering, Flux | `caConfigMapRef` consumed during discovery only | Not solved by ESO (separate concern: trust distribution, not secret distribution) | Not solved by ESO |
| **Single registry per component** | Discovery, Flux | All resources get same SecretRef | Not solved by ESO (API limitation: one SecretRef per Discovery) | Not solved by ESO |
| **Secret recovery after cluster rebuild** | All | Lost unless etcd backup exists | Recoverable if source Secret or source cluster survives | Fully recoverable from KMS |
| **Audit trail** | All | K8s audit log only (if enabled) | K8s audit log only | KMS audit log (who accessed, when rotated) + K8s audit |
| **Air-gapped environments** | All | Works anywhere | Works: ESO `kubernetes` provider is cluster-internal | Requires on-cluster Vault or similar |
| **SolAr code changes needed** | — | None | None | None |
| **Infrastructure required** | — | None | ESO operator per cluster | ESO operator + KMS backend |
| **Responsibility** | — | Tenant or platform team creates Secrets manually | Platform team: ESO + ClusterSecretStore; tenant or platform team: ExternalSecrets per namespace | Platform team: KMS + ESO + ClusterSecretStore + ExternalSecrets |

## Decision Outcome

*To be discussed.*

### Follow-up Issues

- [#165](https://github.com/opendefensecloud/solution-arsenal/issues/165) — Decide how to implement authentication (parent spike)
- [#315](https://github.com/opendefensecloud/solution-arsenal/issues/315) — Make RenderTask cluster-scoped (solves operator secret leaking, separate from this ADR)
