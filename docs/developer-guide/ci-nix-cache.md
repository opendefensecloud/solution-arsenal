# CI Nix binary cache (Cachix + magic-nix-cache)

GitHub Actions workflows in `opendefensecloud/solution-arsenal` run
inside the project's nix dev shell (see issue #649). To keep cold-start
cost low, two cache layers sit in front of every nix store fetch:

- **Cachix** — org-wide binary cache at
  `https://opendefensecloud.cachix.org`. Public read; org-secret-authed
  write. Shared across every repo under `opendefensecloud` that wires
  in `cachix-action`.
- **magic-nix-cache** — per-repo nix store cache backed by GitHub's
  built-in `actions/cache`. Same-job-restart fallback for Cachix
  misses; managed automatically by
  `DeterminateSystems/magic-nix-cache-action`.

Workflows reference both via three pinned actions in order:
`DeterminateSystems/nix-installer-action`,
`cachix/cachix-action`, then
`DeterminateSystems/magic-nix-cache-action`.

## How to consume the cache (workflow author)

Drop the three-action stack into any workflow that needs nix tooling:

```yaml
- name: Install nix
  uses: DeterminateSystems/nix-installer-action@<sha>  # pin per update-action-pins
- name: Use opendefensecloud Cachix cache
  uses: cachix/cachix-action@<sha>
  with:
    name: opendefensecloud
    authToken: ${{ secrets.CACHIX_AUTH_TOKEN }}
- name: Use magic-nix-cache
  uses: DeterminateSystems/magic-nix-cache-action@<sha>
```

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

These steps are already done; documented here for future-readers and
in case the cache needs to be recreated (e.g. after deletion or
migration).

1. Sign in to <https://cachix.org/> with a personal GitHub account
   that has org-admin rights on `opendefensecloud`. The first sign-in
   triggers Cachix's OAuth App authorisation flow; if
   `opendefensecloud` restricts third-party OAuth Apps under
   *Settings → Third-party Access*, approve the Cachix app for the
   org at that moment.
2. Create a cache named **`opendefensecloud`** (lowercase, matches
   the GitHub org slug). Public read access enabled.
3. Generate a write authentication token under the cache's
   *Settings → Auth Tokens* page.
4. Add the token as an **organisation-level** GitHub Actions secret:
   GitHub → `opendefensecloud` org settings → *Secrets and variables
   → Actions → New organisation secret*, name `CACHIX_AUTH_TOKEN`,
   value as copied, repository access *Selected repositories* with
   every solar-sibling repo that consumes the cache.

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

### Token rotation

`CACHIX_AUTH_TOKEN` is long-lived. Standard secret hygiene applies:

- Rotate annually (or sooner if a compromise is suspected). Cachix
  supports multiple write tokens per cache concurrently — issue a new
  token, update the org secret, then revoke the old token from Cachix.
- Limit repository-access scope on the org secret to repos that
  actually need to push. Read-only consumers (local dev, fork PRs)
  don't need it at all.

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
through magic-nix-cache + cold compile.

## Alternatives we considered

### FlakeHub Cache (Determinate Systems)

Hosted Nix binary cache from the same vendor that ships
`nix-installer-action` and `magic-nix-cache-action`. Different model
from Cachix on three axes:

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
  to take on the smaller-ecosystem risk.

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
(see [Caches in play](#caches-in-play)). magic-nix-cache-action is the
optimised version of this approach; we use it as the fallback layer.

## Caches in play

| Aspect | GHA `actions/cache` (incl. magic-nix-cache) | Cachix |
| - | - | - |
| **What it stores** | Arbitrary blobs (nix store paths, Docker layer caches, anything wrapped) | Only nix-store derivations |
| **Where** | GitHub-managed object store, scoped per-repo | `opendefensecloud.cachix.org` CDN |
| **Budget** | 10 GB per repo, hard cap, LRU eviction | Free tier ~10 GB; **growth risk — see Risks** |
| **Scope** | Per ref (branch / PR), with trust-scoping splits per #237 | Global across all opendefensecloud repos |
| **Auth (read)** | Implicit | Public, zero config |
| **Auth (write)** | Implicit | `CACHIX_AUTH_TOKEN`; fork PRs auto-fallback to read-only |
| **Cross-repo sharing** | No | Yes |
| **Cross-PR fork sharing** | No | Yes (read) |
| **Failure mode if missing** | Cold rebuild from source | Cold rebuild from source |
| **Failure mode if poisoned** | Trust-scoping mitigates (see #237) | Content-addressed; cache cannot lie about derivation outputs (hash check fails) |

**Why both:** Cachix is the load-bearing cache (shared, world-readable,
content-addressed). magic-nix-cache is the same-job-restart fallback
and a warm extra layer for when Cachix has a cold miss. Cost of running
both is low; benefit is that an outage at either layer doesn't kill CI.

If the GHA cache budget becomes constrained (it's shared with the
Docker cache-mount preservation from #237), drop magic-nix-cache first
— Cachix covers the same ground at a different layer.

## Glossary

- **`actions/cache`** — GitHub's built-in cache action. Per-repo,
  per-ref, blob-store-backed.
- **`magic-nix-cache-action`** — Determinate Systems' wrapper around
  `actions/cache` that persists the nix store between jobs in the same
  repo. No external account required.
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
