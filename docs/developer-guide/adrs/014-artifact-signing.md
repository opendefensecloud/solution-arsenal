---
status: accepted
date: 2026-07-06
---

# Solar Artifact Signing

## Context and Problem Statement

Rendered Artifacts are rendered and pushed by the solar-renderer. However there
is no way users can verify the generated artifacts are actually authentic.

## Considered Options

There are multiple decisions to be considered:

### I. What method of verification should be used in air-gapped environments?

The verification process itself should be handled by FluxCD by configuring the
OCIRepository Resources accordingly.

#### 1. Key-pair based verification

Use only the public key of the generated keypair to verify the OCI artifact
via cosign's key-based mode. The private key signs the artifact during
rendering; the public key is embedded in the OCIRepository's `spec.verify`
block on the target cluster.

Solar needs to distribute the public key to the target clusters.

Since keypairs do not have any method of invalidation in this mode, multiple
keypairs should be generated (1 per target) in order to minimize the blast
radius in case of the private key being leaked.

**Pros:**

- Simple to implement and reason about.
- No external infrastructure dependencies (no need for Rekor, Fulcio, or an
  OIDC provider).
- Works natively in air-gapped environments without internet connectivity.
- FluxCD has built-in support for cosign key-based verification.

**Cons:**

- Key rotation requires regenerating per-target keypairs and updating every
  OCIRepository resource.
- No transparency log; key compromise is not detectable until the key is used
  maliciously.
- Blast radius is limited to one target per keypair, but managing N keypairs
  adds operational overhead.

#### 2. Key-less verification

Self-host the necessary infrastructure to enable keyless verification (Rekor,
Fulcio, OIDC provider, ingress).

OCIRepository resources need a configuration secret pointing Flux' controller
to the private infrastructure.

Keyless verification uses ephemeral certificates issued by Fulcio (backed by an
OIDC identity token) and records the signature in Rekor's transparency log.
This eliminates the need to manage and distribute long-lived signing keys.

**Pros:**

- No long-lived key material to manage, rotate, or distribute.
- Transparency log provides an auditable trail of all signatures.
- Compromise detection is inherent: a key used without the corresponding OIDC
  identity is visible in Rekor.

**Cons:**

- Requires self-hosting and maintaining Rekor, Fulcio, and an OIDC provider
  (e.g., Dex or Keycloak), plus ingress and TLS termination.
- Significant infrastructure complexity and operational burden.
- FluxCD's keyless verification mode expects access to a Rekor instance, which
  adds a runtime dependency on the private infrastructure.

### II. How are artifacts Signed?

#### 1. Extend Renderer CLI to both render & push as well as sign the artifact

The existing `solar-renderer` binary gains a signing step after pushing the
rendered artifact. The private key (or a pointer to it) is passed via the
existing `RendererConfig`. Signing happens synchronously within the same
process.

**Pros:**

- Minimal changes to the existing job architecture; one pod, one process.
- Signing is tightly coupled to rendering by design — the rendered artifact is
  signed before the pod completes, so there is no window where an unsigned
  artifact exists in the registry.
- No additional controller logic or Job creation needed.

**Cons:**

- The renderer binary takes on a responsibility (signing) that is conceptually
  distinct from rendering.
- If the signing step fails, the entire render job fails, even though the
  artifact was successfully pushed.

#### 2. Extend RenderTask Controller to schedule multiple Jobs for render & push and signing

The RenderTask controller creates a second Job (or a second container in the
same Pod) that reads the rendered artifact from the registry, signs it, and
pushes the signature back. The two Jobs execute sequentially: render first,
sign second.

**Pros:**

- Clear separation of concerns — the renderer only renders, the signer only
  signs.
- Each step can be retried independently; a failed signing attempt does not
  require re-rendering.

**Cons:**

- The controller must track Job state across two phases (render → sign) and
  handle the transition, adding complexity.
- More Kubernetes API calls and Pod overhead.

#### 3. Remodel RenderTasks as ArgoWorkflows

Replace the current single-Job RenderTask with an ArgoWorkflow that models the
render → sign pipeline. Each step runs in its own container; Argo manages
sequencing, retries, and artifacts.

**Pros:**

- Native support for multi-step pipelines with automatic sequencing and
  parallelization.
- Built-in retry, timeout, and artifact passing between steps.

**Cons:**

- Introduces a heavyweight dependency (ArgoWorkflows CRDs, controller,
  controller-manager) for what is currently a two-step pipeline.
- Existing RenderTask controller and CRD would need significant rework or
  replacement.
- Operational complexity: operators must deploy and maintain ArgoWorkflows
  alongside Solar.

## Decision Outcome

We decided to use key-pair based verification (I-1). The keypair will be
generated by the `target_controller` and the public key will be distributed
with the `solar-agent` to the target cluster. Each target gets its own keypair
so that a single key compromise does not affect other targets.

The renderer CLI will be extended to also sign the rendered artifact inline
(II-1). The render-and-sign logic is tightly coupled: the signing key is
available in the render job context.

Extending the RenderTask controller to schedule separate Jobs (II-2) or
replacing RenderTasks with ArgoWorkflows (II-3) would introduce orchestration
complexity that is not justified for a single sequential signing step.
