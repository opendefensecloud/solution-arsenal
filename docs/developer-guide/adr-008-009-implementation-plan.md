# Implementation Plan — ADR-008 & ADR-009

This document breaks the work required to realize
[ADR-008 (No-Auth Architecture)](./adrs/008-No-Auth-Architecture.md) and
[ADR-009 (Target Bindings Split)](./adrs/009-Target-Bindings-Split.md) into
epics and concrete work items.

It is intended as a starting point for issue tracker import — each work item
should be small enough to become a single issue/PR. Epics are ordered roughly
by dependency, but many items inside an epic can be parallelized.

## Guiding Principles

- **Ship in slices.** Each epic should land in a state where `make test` and
  `make test-e2e` are green; resources and controllers are introduced behind
  feature gates where that reduces churn.
- **Favor deletion over compatibility shims.** Per CLAUDE.md conventions: when
  replacing old behavior, remove the old code rather than leaving both paths.
  Migration is handled out-of-band.
- **Decouple discovery from the main pipeline early** so everything downstream
  can assume the catalog can be populated by any means.
- **API first, then controllers, then renderer, then cleanup.** Establish the
  new shape before rewiring the control loops.

---

## Epic Overview

| # | Epic | Drives |
|---|------|--------|
| E1 | Standalone Discovery Workstream | ADR-008 principle 6 |
| E2 | Component Origin & Registry Alias Metadata | ADR-008 principles 1 & 5 |
| E3 | `Registry` Resource | ADR-009 |
| E4 | `RegistryBinding` Resource | ADR-009, ADR-008 principle 2 |
| E5 | `ReleaseBinding` Resource | ADR-009 |
| E6 | Target Slim-Down & Render Registry Reference | ADR-009 |
| E7 | Renderer Refactor Around Bindings | ADR-008 principles 3–5, ADR-009 |
| E8 | Profile Controller Rewrite → Emits ReleaseBindings | ADR-009 |
| E9 | Bootstrap Obsolescence & Removal | ADR-009 |
| E10 | RBAC, Samples, Fixtures & E2E | ADR-009 |
| E11 | Docs, CLI & UX | Both ADRs |

---

## E1 — Standalone Discovery Workstream

**Goal:** `solar-discovery-worker` is fully optional. The apiserver,
controller-manager, and renderer can operate with an empty or
externally-populated catalog.

- [ ] **E1-1** Audit the apiserver/controller-manager startup paths for any
      implicit dependency on a running discovery worker; list them in the
      issue description.
- [ ] **E1-2** Remove any hard dependencies found in E1-1 (e.g. watches that
      panic if `Discovery` CRs are missing, init containers that assume
      discovery state).
- [ ] **E1-3** Gate discovery-worker deployment in the Helm chart behind
      `discovery.enabled` (default `true` for now); ensure the chart renders
      and installs with `discovery.enabled=false`.
- [ ] **E1-4** Add a minimal "catalog-only" e2e scenario that creates
      `Component`/`ComponentVersion` via the API (no discovery worker
      running) and proceeds through render + rollout.
- [ ] **E1-5** Document how to run SolAr without discovery in
      `docs/developer-guide/` (short how-to, link from README feature list).
- [ ] **E1-6** Split the discovery worker into its own compose/skaffold
      profile in `hack/` so local development can omit it.

---

## E2 — Component Origin & Registry Alias Metadata

**Goal:** Every `ComponentVersion` declares the registry it originates from
and any aliases needed for multi-environment reachability. This is the
pre-requisite for render-time registry validation (ADR-008 principle 5).

- [ ] **E2-1** Extend the `ComponentVersion` API type with a structured
      `origin` field: canonical registry URL + optional aliases list.
- [ ] **E2-2** Update discovery pipeline (`pkg/discovery/handler`,
      `pkg/discovery/apiwriter`) to populate `origin` from the scanned
      registry.
- [ ] **E2-3** Add validation: `origin.registry` is required on write;
      aliases must be unique and syntactically valid.
- [ ] **E2-4** Backfill existing fixtures and e2e component versions with
      explicit origin.
- [ ] **E2-5** `make codegen` + regenerate client-go / openapi / CRDs.
- [ ] **E2-6** Introduce a small `pkg/registry/alias` helper that matches a
      required registry URL against a set of aliases (shared by renderer &
      validator). Add unit tests.

---

## E3 — `Registry` Resource

**Goal:** Introduce `Registry` as a first-class API type.

- [ ] **E3-1** Decide scope: **cluster-scoped vs namespaced** (ADR-009 open
      question). Produce a short decision note and update ADR-009.
- [ ] **E3-2** Define `api/solar/registry_types.go` +
      `api/solar/v1alpha1/registry_types.go` with fields: `url`, `aliases`,
      `capabilities` (source/destination/mirror/pull-through),
      `credentialsSecretRef`, `tls`.
- [ ] **E3-3** Implement REST storage (`registry_rest.go`) following the
      pattern of existing resources.
- [ ] **E3-4** Add defaulting + validation (URL syntax,
      capabilities enum, at most one `credentialsSecretRef`).
- [ ] **E3-5** `make codegen` + `make manifests`; wire into RBAC generation.
- [ ] **E3-6** Add a `RegistryController` in `pkg/controller/registry/` that
      surfaces connectivity-independent status conditions (`Valid`,
      `SecretResolved`) without actually dialing the registry.
- [ ] **E3-7** Unit tests for validation, defaulting, and the controller.
- [ ] **E3-8** Add kubectl printer columns (`URL`, `Capabilities`, `Age`).

---

## E4 — `RegistryBinding` Resource

**Goal:** Introduce `RegistryBinding` as a pure link between `Target` and
`Registry`, per ADR-009.

- [ ] **E4-1** Define `api/solar/registrybinding_types.go` +
      versioned type with fields: `targetRef`, `registryRef`, `role`
      (source/destination), optional `pullSecretOverrideRef`.
- [ ] **E4-2** REST storage implementation.
- [ ] **E4-3** Admission validation: both refs must resolve; `role` is
      required; duplicate (target, registry, role) triples are rejected.
- [ ] **E4-4** `RegistryBindingController` that reconciles status:
      `TargetResolved`, `RegistryResolved`, aggregate `Ready`.
- [ ] **E4-5** Index bindings by `targetRef` and by `registryRef` (client-go
      field selectors or controller-runtime indexers) — used heavily by the
      renderer in E7.
- [ ] **E4-6** Unit + envtest coverage.
- [ ] **E4-7** Printer columns: `Target`, `Registry`, `Role`, `Ready`.

---

## E5 — `ReleaseBinding` Resource

**Goal:** Externalize release-to-target assignment from Target and from
Profile/HydratedTarget.

- [ ] **E5-1** Define `api/solar/releasebinding_types.go` +
      versioned type with fields: `targetRef`, `releaseRef` (or
      `profileRef`), optional `overrides`, optional scheduling hints.
- [ ] **E5-2** REST storage implementation.
- [ ] **E5-3** Validation: exactly one of `releaseRef` / `profileRef`;
      `targetRef` resolves; overrides schema checks.
- [ ] **E5-4** `ReleaseBindingController` surfaces: `TargetResolved`,
      `ReleaseResolved`, `Rendered`, `Ready`.
- [ ] **E5-5** Target-indexed lister for renderer consumption.
- [ ] **E5-6** Unit + envtest coverage.
- [ ] **E5-7** Printer columns: `Target`, `Release`, `Ready`.

---

## E6 — Target Slim-Down & Render Registry Reference

**Goal:** `Target` becomes a focused identity/lifecycle resource with a
single `renderRegistryRef`.

- [ ] **E6-1** Remove embedded registry access fields from `Target` spec
      (`registries`, inline credentials, etc.). List every field being
      removed in the issue description.
- [ ] **E6-2** Remove embedded release assignments from `Target` spec.
- [ ] **E6-3** Add `spec.renderRegistryRef` (required on create; must
      resolve to an existing `Registry` with `destination` capability).
- [ ] **E6-4** Update `Target` admission to enforce the above on create and
      update.
- [ ] **E6-5** Update `TargetController` to reflect the new shape and drop
      reconciliation logic that is now owned by bindings.
- [ ] **E6-6** Regenerate client-go, openapi, CRDs, conversions.
- [ ] **E6-7** Update every fixture under `test/fixtures/**` that references
      Target to use the new shape; delete obsolete YAML keys.
- [ ] **E6-8** Decide and document status surfacing (ADR-009 open question):
      which conditions live on Target vs on bindings.

---

## E7 — Renderer Refactor Around Bindings

**Goal:** `solar-renderer` + `pkg/renderer/` consume `ReleaseBinding`s,
`RegistryBinding`s, and the Target's render registry — with render-time
validation per ADR-008 principle 5.

- [ ] **E7-1** Replace the renderer's "read Target spec" step with three
      lookups: (a) `ReleaseBinding`s for this Target, (b) `RegistryBinding`s
      for this Target (resolved to `Registry` objects), (c) Target's
      `renderRegistryRef`.
- [ ] **E7-2** Implement source-registry validation: for each bound Release,
      walk required component origins and assert each is covered by a
      `source`-role RegistryBinding (using the alias matcher from E2-6).
      Fail early with a clear error naming the missing binding.
- [ ] **E7-3** Resolve destination push target from the Target's render
      registry (no longer from bindings).
- [ ] **E7-4** Preserve "granular output" (ADR-008 principle 4): confirm the
      existing multi-artifact model is kept; add a regression test.
- [ ] **E7-5** Per-target rendering stays the default (ADR-008 principle 3).
      Leave a TODO + issue link for the future dedup optimization, but do
      not implement it now.
- [ ] **E7-6** Update `pkg/controller/rendertask` to drive renders from
      bindings instead of Target-embedded fields.
- [ ] **E7-7** Update `pkg/renderer/template/release/templates/release.yaml`
      and related Helm template logic to consume the new value shape where
      needed.
- [ ] **E7-8** Unit tests for binding resolution, validation failures, and
      destination selection. envtest coverage for the controller glue.

---

## E8 — Profile Controller Rewrite → Emits ReleaseBindings

**Goal:** Profiles become a higher-level templating concept whose output is
a set of `ReleaseBinding`s.

- [ ] **E8-1** Redefine Profile semantics: describe selector/criteria for
      matching Targets and the Releases to materialize. Update docs.
- [ ] **E8-2** Replace the existing Profile→HydratedTarget reconciliation
      with a Profile→ReleaseBinding reconciliation loop in
      `pkg/controller/profile/`.
- [ ] **E8-3** Ensure Profile-owned `ReleaseBinding`s carry an owner
      reference back to the Profile so deletion cascades cleanly.
- [ ] **E8-4** Add Profile-level status summarizing how many Targets were
      matched and how many bindings are Ready.
- [ ] **E8-5** envtest coverage for profile reconcile: create/update/delete
      should produce the expected bindings.
- [ ] **E8-6** Update e2e profile fixtures accordingly.

---

## E9 — Bootstrap Obsolescence & Removal

**Goal:** The `Bootstrap` resource (formerly `HydratedTarget`) is removed
once its role is fully expressed by `ReleaseBinding`s and the Target's
render registry reference.

- [ ] **E9-1** Confirm there are no remaining consumers of `Bootstrap` once
      E7 and E8 have landed (grep `pkg/`, `cmd/`, `charts/`, fixtures).
- [ ] **E9-2** Delete the `Bootstrap` API type, REST storage, controller,
      and its generated client-go code.
- [ ] **E9-3** Delete `pkg/renderer/render_bootstrap*.go` and associated
      templates; move any still-useful logic into the release renderer.
- [ ] **E9-4** Remove bootstrap fixtures under `test/fixtures/**`.
- [ ] **E9-5** Update the e2e suite: the "should bootstrap a cluster
      successfully" test either migrates to a binding-based equivalent or
      is retired in favor of a binding-centric replacement.
- [ ] **E9-6** Update architecture docs, user stories, and any remaining
      references in `docs/`.
- [ ] **E9-7** Regenerate manifests; confirm no dangling CRD.

---

## E10 — RBAC, Samples, Fixtures & E2E

**Goal:** Ownership split is enforceable via RBAC and demonstrated via
samples and e2e.

- [ ] **E10-1** Define a set of reference Roles matching the ADR-009 RBAC
      table (Cluster Maintainer, Platform Provider, Tenant). Ship them as
      optional manifests under `config/rbac/` or `charts/solar/templates/`.
- [ ] **E10-2** Add sample YAMLs under `config/samples/` for `Registry`,
      `RegistryBinding`, `ReleaseBinding`, and the slimmed `Target`.
- [ ] **E10-3** Update `hack/dev-cluster.sh` + `test/fixtures/e2e/` to the
      new resource shape end-to-end. Delete every YAML that embeds old
      Target fields.
- [ ] **E10-4** Add an e2e test that exercises the full chain: create
      `Registry` → create `RegistryBinding` (source + destination) →
      create `Target` with `renderRegistryRef` → create `ReleaseBinding` →
      observe render + deployment healthy (reuse the label-based deployment
      check added in the bootstrap test).
- [ ] **E10-5** Add a negative e2e: missing source `RegistryBinding` causes
      rendering to fail with the expected error.
- [ ] **E10-6** Update `make test-e2e` docs if the new flow requires extra
      setup.

---

## E11 — Docs, CLI & UX

**Goal:** Users understand the new model and tooling surfaces it well.

- [ ] **E11-1** Rewrite `docs/developer-guide/architecture.md` to describe
      the Target/Registry/RegistryBinding/ReleaseBinding model and the
      no-auth stance.
- [ ] **E11-2** Update `docs/developer-guide/user-stories.md` to reflect
      the new ownership split (who creates what).
- [ ] **E11-3** Update `docs/developer-guide/rendertask_controller.md` for
      the binding-driven render flow.
- [ ] **E11-4** Add a short migration note for anyone running the
      pre-split version (link-in from the README).
- [ ] **E11-5** Audit CLI/UI surfaces (if any) for references to the
      removed Target fields and update printer columns, descriptions, and
      help text.
- [ ] **E11-6** Close out the ADR-009 "Open Questions" section with the
      decisions made during implementation (namespace scope, status
      location, render-registry authorization).

---

## Dependency Sketch

```
E1  ──┐
E2  ──┤
E3  ──┼──► E4 ──┐
      │         ├──► E7 ──► E9 ──► E10 ──► E11
E5  ──┘         │
                │
E6  ────────────┘
E8 depends on E5 (and ideally E6 for clean Target shape)
```

E1 and E2 are independent and can start immediately. E3/E5/E6 can progress
in parallel once their API shape is agreed. E7 is the integration point and
should wait until E3–E6 have landed at least skeleton versions. E9 is the
final cleanup and should not begin until E7 and E8 are green.
