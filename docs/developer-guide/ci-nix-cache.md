# CI Nix binary cache (Cachix)

GitHub Actions workflows in `opendefensecloud/solution-arsenal` run
inside the project's nix dev shell (see issue #649). To keep cold-start
cost low, one binary cache sits in front of every nix store fetch:

- **Cachix** — org-wide binary cache at
  `https://opendefensecloud.cachix.org`. Public read; org-secret-authed
  write. Shared across every repo under `opendefensecloud` that wires
  in `cachix-action`.

Workflows reference this via two pinned actions in order:
`DeterminateSystems/nix-installer-action` (with `determinate: false` to
use upstream Nix instead of the Determinate fork), then
`cachix/cachix-action`.

## How to consume the cache (workflow author)

Drop the two-action stack into any workflow that needs nix tooling:

```yaml
- name: Install nix
  uses: DeterminateSystems/nix-installer-action@<sha>  # pin per update-action-pins
  with:
    determinate: false          # upstream Nix, not the Determinate fork
    diagnostic-endpoint: ''     # don't post install telemetry
- name: Use opendefensecloud Cachix cache
  uses: cachix/cachix-action@<sha>
  with:
    name: opendefensecloud
    authToken: ${{ secrets.CACHIX_AUTH_TOKEN }}
    signingKey: ${{ secrets.CACHIX_SIGNING_KEY }}
```

Both org-level secrets are required for push:

- `CACHIX_AUTH_TOKEN` authenticates with Cachix's HTTP API.
- `CACHIX_SIGNING_KEY` signs each store path before upload — see the
  *Signing model (BYOK)* section under Risks for why we hold the
  signing key ourselves rather than letting Cachix manage it.

Fork PRs receive neither secret (GitHub doesn't forward org secrets
to forks), and `cachix-action` auto-falls-back to read-only.

…then wrap subsequent `run:` steps in the dev shell either via job-level
`defaults.run.shell: nix develop --command bash -e {0}` or per-step
`shell:` override. See `.github/workflows/helm-lint.yaml` for the
simplest example.

## How to consume the cache (local development)

The cache is world-readable. To benefit from pre-built derivations on a
local machine without any account or login:

```bash
cachix use opendefensecloud
```

That writes `https://opendefensecloud.cachix.org` into your Nix
substituter list. Subsequent `nix develop` / `nix build` invocations
will hit Cachix before falling back to source builds.

No write access; you only push from CI.

## One-time setup (org admin reference)

These steps are run once when the cache is provisioned. Documented
here for future-readers and in case the cache needs to be recreated.

**About signing keys.** By default Cachix generates the signing
keypair server-side and holds the private half itself. That puts
Cachix in the trust boundary for anything signed as authentic (see
*Signing model (BYOK)* under Risks). To keep the private key on our
side, we use `cachix generate-keypair` — a CLI command that
generates the keypair locally, registers the public half with
Cachix, and prints the private half to stdout for us to store
securely.

Cachix has no web-UI surface for uploading a pre-existing public
key. The `cachix generate-keypair` CLI is the only supported path
([cachix/cachix#292](https://github.com/cachix/cachix/issues/292) is
the open feature request for "add existing public key"; it's been
open since 2020 and has no timeline). If we ever needed to rotate
the key on the existing cache, we'd rerun `generate-keypair` and
Cachix would append a new trusted public key alongside the old one.

### Steps

1. **Sign in to <https://cachix.org/>** with a personal GitHub
   account that has org-admin rights on `opendefensecloud`. First
   sign-in triggers Cachix's OAuth App authorisation flow; if the
   org restricts third-party OAuth Apps under
   *Settings → Third-party Access*, approve the Cachix app at that
   moment.

2. **Create a cache named `opendefensecloud`** (lowercase, matches
   the GitHub org slug). Public read access enabled. **Do not**
   generate a signing key at this step — we'll do that via the CLI
   in step 4 so the private half stays local.

3. **Get a personal auth token with admin permissions.** This is
   important and easy to miss: `cachix generate-keypair` needs a
   personal auth token that has admin on the target cache. **A
   per-cache write token is not enough**; per-cache tokens can push
   store paths but cannot alter cache configuration (which is what
   registering a signing key does).

   Get the personal token from
   *<https://app.cachix.org/personal-auth-tokens>*. Store it locally
   for the next step:

   ```bash
   export CACHIX_AUTH_TOKEN=<personal-admin-token>
   ```

4. **Generate the signing keypair** on the machine you're on:

   ```bash
   cachix generate-keypair opendefensecloud
   ```

   This does three things atomically:
   - generates the keypair,
   - **registers the public half with Cachix** as the trusted key
     for the `opendefensecloud` cache,
   - **prints the private half to stdout.** Capture it — Cachix does
     not store the private key and there is no way to retrieve it
     later. Treat it like any long-lived credential (never commit,
     never paste into a chat / ticket).

5. **Generate a write authentication token** at the cache's
   *Settings → Auth Tokens* page. This is a different token from
   the personal one used in step 3 — it's cache-scoped, write-only,
   and safe to store in CI secrets.

6. **Add two organisation-level GitHub Actions secrets**: GitHub →
   `opendefensecloud` org settings → *Secrets and variables →
   Actions → New organisation secret*. Repository access
   *Selected repositories* with every solar-sibling repo that
   consumes the cache.
   - `CACHIX_AUTH_TOKEN` — the write token from step 5 (NOT the
     personal admin token from step 3; that stays on the admin's
     machine and can be discarded once setup is done).
   - `CACHIX_SIGNING_KEY` — the private key printed by step 4.

7. **Verify the setup** with `cachix doctor <cache-name>`:

   ```bash
   cachix doctor opendefensecloud
   ```

   The doctor command reports the cache's configured public keys
   and whether they match what the client thinks it should trust.
   Expected output includes `opendefensecloud-1:...` under the
   cache's public keys section (with an actual key body, not an
   empty list). An empty `Cache public keys: []` in the Cachix
   daemon logs from a CI run means step 4 didn't stick — most
   likely `CACHIX_AUTH_TOKEN` in step 3 wasn't a personal admin
   token, so the public-key registration failed silently.

8. **Securely discard** the personal admin token from step 3.
   Everything else lives in the org secret store from now on.

`cachix-action` adds the substituter and trusted public key
automatically; nothing needs declaring in `flake.nix`.

## Risks and what to monitor

### Cachix charging if the cache outgrows the free tier

**The single most important thing to watch.** Cachix's free tier covers
OSS public caches up to roughly 10 GB at the time of writing (verify
current at <https://cachix.org/pricing>). Once the cache exceeds that
threshold, Cachix bills the cache owner — and the cache will grow
indefinitely under normal CI operation unless something stops it.

How the cache grows:

- Every successful CI run pushes new nix-store derivations: build
  outputs for each new flake input version, transitive nixpkgs
  updates, anything Renovate bumps.
- Renovate-driven flake updates are the biggest single source of
  growth. A nixpkgs bump invalidates a large fraction of the cached
  store at once.
- The cache does **not** automatically evict old derivations. Without
  intervention, every historical nixpkgs commit's outputs accumulate
  forever.

How to prevent surprise billing:

- **Set a billing alert at the GitHub-org level** — not at a personal
  level. The Cachix dashboard surfaces cache size; check it quarterly.
  If size approaches the free-tier ceiling, decide between paying,
  pruning, or migrating.
- **Configure cache retention.** Cachix exposes a *Garbage Collection*
  policy under the cache's *Settings → Garbage Collection* page. The
  conservative default keeps everything; the recommended posture for
  this cache is **delete unreachable paths older than N days** (start
  with 30 days). Reachability is computed from the current `flake.lock`
  files of consuming repos, so anything currently in use stays;
  abandoned derivations expire.
- **Watch the dashboard, don't trust silent steady-state.** A single
  large new dependency (e.g. a new browser binary or a new toolchain)
  can add multiple GB in one CI run. The dashboard's size graph makes
  this obvious; no email alert exists at free tier.

If the cache does outgrow the free tier:

- **Short-term:** enable a more aggressive garbage-collection window
  (e.g. 7 days), or run a one-off `cachix gc` to manually prune
  unreachable paths.
- **Long-term:** consider paying for an entry-tier plan ($5/month
  range at the time of writing for moderate usage), or migrate to
  FlakeHub Cache (see *Alternatives* below) which has different
  free-tier mechanics.

Why this risk is non-trivial: a forgotten OSS cache that quietly grew
past the free tier and started billing a credit-card-on-file (or
worse, the personal card of whoever signed up) is a known failure mode
for similar SaaS services. Mitigation is policy + monitoring, not
configuration we can ship in this repo.

### Signing model (BYOK)

**Why we hold our own signing key.** Cachix's default cache-creation
flow generates the Nix store signing keypair on Cachix's servers; the
private half stays with Cachix and the cache returns store paths
signed under that key. That puts Cachix in the supply-chain trust
boundary: anyone who can convince Cachix to sign arbitrary store paths
on our behalf (server compromise, internal misuse, compelled
disclosure, an auth-layer bug that cross-bleeds two customers'
caches) can ship a derivation that our consumers — CI runs and local
dev shells — verify as legitimate.

This cache is configured with **bring-your-own signing key (BYOK)**.
We generate the keypair locally during setup, hold the private half
in the `CACHIX_SIGNING_KEY` org secret, and Cachix only sees the
public half. Cachix can still host bytes, throttle, or delete the
cache — but it cannot forge a signed derivation.

**Trust boundary in plain terms:**

- *Without BYOK*: trust Cachix (the company), Cachix's
  infrastructure, every Cachix employee with access to the signing
  keystore.
- *With BYOK*: trust whoever holds the `CACHIX_SIGNING_KEY` org
  secret — i.e. the GitHub org's secret store and the maintainers
  with admin access to it.

**Key rotation playbook.** Treat the signing key like any other
long-lived secret; rotate annually, or sooner on suspected
compromise. Cachix supports multiple public keys per cache
simultaneously — every invocation of `cachix generate-keypair`
appends a new key alongside the existing ones — which is what makes
rotation safe.

Prerequisite: a **personal auth token with admin permissions** on
the cache (see [step 3 of the one-time setup](#steps)). Per-cache
write tokens are not sufficient — key registration is a
configuration change.

1. Export the personal admin token:

   ```bash
   export CACHIX_AUTH_TOKEN=<personal-admin-token>
   ```

2. Generate a new keypair and register the new public key:

   ```bash
   cachix generate-keypair opendefensecloud
   ```

   Capture the printed private key. Cachix now trusts signatures
   under *either* the old or the new public key.

3. Update the `CACHIX_SIGNING_KEY` org secret with the new private
   key. The next CI run signs all new uploads under the new key.
   Existing cache content stays valid under the old public key.

4. **Wait at least one consumer-cache lifetime** (a week is
   conservative) so any cached substituter state on devs' machines
   has refreshed to include the new public key.

5. **Remove the old public key.** Cachix's UI exposes key deletion
   under the cache's settings page (this operation *is* supported
   even though key addition is not — the asymmetry is a Cachix
   quirk). If deleting from the UI proves impossible, Cachix
   support can do it.

6. Verify the rotation with `cachix doctor opendefensecloud` — the
   old key should be absent and only the new one listed.

7. Securely discard the personal admin token from step 1.

**Auth token rotation** (orthogonal to signing — token controls
*write access*, signing key controls *what the cache says is
authentic*):

- Rotate `CACHIX_AUTH_TOKEN` annually. Cachix supports multiple
  write tokens per cache concurrently — issue a new token, update
  the org secret, then revoke the old token from Cachix.
- Limit repository-access scope on both org secrets to repos that
  actually need to push. Read-only consumers (local dev, fork PRs)
  don't need either secret.

### OAuth App approval is one-way

Cachix's OAuth App stays authorised on `opendefensecloud` until an
admin revokes it from the GitHub org settings page. Not a problem in
normal operation; mention here so it shows up in the audit trail if
someone wonders why Cachix has third-party-app access on the org.

### Fork PR pushes are silently disabled

Fork PRs don't receive organisation secrets — `CACHIX_AUTH_TOKEN`
arrives empty in those runs. `cachix-action` auto-detects this and
falls back to read-only. Fork-PR builds still benefit from the cache
(reads are public) but don't push their build outputs. This is the
secure default; without it a malicious fork PR could poison the cache.

If a fork PR fails with a Cachix push error, the symptom is a
warning-level log line — not a hard failure. The build still succeeds
via cold compile.

## Alternatives we considered

### FlakeHub Cache (Determinate Systems)

Hosted Nix binary cache from the same vendor that ships
`nix-installer-action`. Different model from Cachix on three axes:

- **Auth:** GitHub App, org-installed. Runtime auth is GitHub Actions
  OIDC — no long-lived `AUTH_TOKEN` to rotate. Fewer secrets to manage
  on the org side.
- **Setup:** install the FlakeHub GitHub App on `opendefensecloud`
  once; consuming repos add `DeterminateSystems/flakehub-cache-action`
  (or similar) to their workflows. No personal-account intermediary.
- **Pricing:** free for OSS; commercial tiers via Determinate Systems.
  At the time of writing, smaller install base than Cachix but
  actively developed.

Why we didn't pick it for this ticket:

- Cachix's OAuth flow is broadly understood; FlakeHub is the newer
  entrant.
- Switching from Cachix to FlakeHub later is mechanical — both expose
  the standard nix-store HTTP binary-cache protocol; consumers flip
  one substituter URL. Picking Cachix first is not a one-way door.
- We do not need OIDC's no-long-lived-secret property strongly enough
  to take on the smaller-ecosystem risk; BYOK on Cachix already
  removes Cachix from the signing-trust boundary, and our two
  remaining long-lived secrets (`CACHIX_AUTH_TOKEN`,
  `CACHIX_SIGNING_KEY`) rotate annually.

If the Cachix free-tier billing risk above ever becomes acute (e.g.
the cache outgrows free tier and we don't want to pay), FlakeHub is
the natural escape hatch.

### Self-hosted Attic

Open-source Nix binary cache server (PostgreSQL + S3-compatible
backend). Maximum control, no third-party dependency, no SaaS billing
risk — but we'd be operating a server. At our size the ops load
outweighs the savings. Reconsider if the project ever runs its own
infrastructure independently.

### Plain `actions/cache` against the nix store

Just persist `~/.nix/store` via GitHub's built-in cache action. Cheap
(no external accounts), but the Nix store is a many-small-files tree
that `actions/cache` handles poorly — restore times balloon, and the
GHA 10 GB cap is shared with everything else
(see [Caches in play](#caches-in-play)).

### DeterminateSystems/magic-nix-cache-action (tried and dropped)

Determinate Systems' polished wrapper around `actions/cache` that
persists the nix store between jobs in the same repo. We wired it
alongside Cachix during PR #661 as a same-job-restart fallback layer.

In practice the post-step spent ~487 s/run uploading every
nix-store path to GHA cache — regardless of whether Cachix already
held the same content. That created ~2500 tiny cache entries per
repo, filled the shared 10 GB `actions/cache` pot (which also holds
#237's Docker cache-mount preservation), and started evicting our
real caches. `use-flakehub: false` didn't help because the GHA-cache
upload path is the offending workload.

Removed. Cachix is content-addressed and world-readable — same
"warm nix store between runs" property at a different layer,
without the local upload thrash.

## Caches in play

Two independent caches are in play in the workflows this PR touches:

| Aspect | Cachix (`opendefensecloud.cachix.org`) | GHA `actions/cache` for Go build+module caches |
| - | - | - |
| **What it stores** | Nix-store derivations only (`.narinfo` + `.nar`) | `~/go/pkg/mod` and `~/.cache/go-build` |
| **Where** | Cachix CDN | GitHub-managed object store, scoped per-repo |
| **Budget** | Free tier ~10 GB; **growth risk — see Risks** | 10 GB per repo, hard cap, LRU eviction |
| **Scope** | Global across all opendefensecloud repos | Per ref (branch / PR), trust-scoped per #237 pattern |
| **Auth (read)** | Public, zero config | Implicit |
| **Auth (write)** | `CACHIX_AUTH_TOKEN` + `CACHIX_SIGNING_KEY` (BYOK); fork PRs auto-fallback to read-only | Implicit |
| **Cross-repo sharing** | Yes | No |
| **Cross-PR fork sharing** | Yes (read) | No |
| **Failure mode if missing** | Cold rebuild from source | Cold `go mod download` + full compile |
| **Failure mode if poisoned** | Content-addressed; cache cannot lie about derivation outputs (hash check fails) | Trust-scoping mitigates (see #237) |

**They cover different things.** Cachix serves nix-store derivations
(Go toolchain, all system tooling, everything the flake evaluates).
The GHA `go-cache-*` entries serve Go's own incremental compile
outputs, which live on top of whatever Go binary Cachix provides.
Both are needed; neither replaces the other.

The GHA-cache pot is shared with #237's Docker cache-mount
preservation. If it starts getting tight, see #237's list of
mitigations — they apply here too.

## Glossary

- **`actions/cache`** — GitHub's built-in cache action. Per-repo,
  per-ref, blob-store-backed. Used in this workflow for Go's module
  and build caches (`go-cache-*` scopes).
- **Cachix** — third-party Nix binary cache provider. OAuth-based
  setup. Org-owned caches at `https://<org>.cachix.org`.
- **FlakeHub Cache** — Determinate Systems' hosted Nix binary cache.
  GitHub-App-based setup, OIDC runtime auth.
- **Binary cache substituter** — a URL Nix queries before building any
  derivation; if the substituter returns the prebuilt output with a
  matching hash, Nix uses it instead of rebuilding.
- **`.narinfo` / `.nar`** — Nix's storage formats. `.narinfo` is the
  metadata index; `.nar` is the content-addressed archive of a built
  derivation.
- **Cache garbage collection** — Cachix-side process that deletes
  store paths no longer referenced by any registered `flake.lock`.
  Configured per-cache under *Settings → Garbage Collection*.
- **BYOK (Bring-your-own key)** — option at Cachix cache creation to
  supply the public signing key yourself, keeping the private half
  off Cachix's servers. Used by this cache; see *Signing model
  (BYOK)* under Risks.
