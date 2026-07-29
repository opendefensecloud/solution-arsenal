# Releasing

Releases are automated with [release-please](https://github.com/googleapis/release-please), driven entirely by [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/) messages on `main`. There are no release labels and no manual tagging.

## Version resolution

| Commit type | Version bump |
|---|---|
| `fix:` | patch |
| `feat:` | minor |
| `feat!:` / `BREAKING CHANGE:` footer | major |
| `chore:`, `docs:`, `ci:`, … | none |

To force a specific version, add a `Release-As: x.y.z` footer to a commit on `main`.

The first release-please run graduates from the `0.3.0-rc2` bootstrap to a stable `0.3.0`: the migration's merge commit carries a `Release-As: 0.3.0` footer so the first Release PR targets `0.3.0`. After that, versions follow semver from the commit types above. Release candidates and other prereleases are no longer cut automatically.

## What happens when

1. **Commits land on `main`.** On every push, the [Release Please workflow](https://github.com/opendefensecloud/solution-arsenal/blob/main/.github/workflows/release-please.yaml) scans commits since the last release. If at least one releasable commit (`feat`/`fix`/breaking) exists, it opens or updates a **Release PR** that bumps the version and updates `CHANGELOG.md`. `chore`-only batches never produce a release.
2. **The Release PR is merged.** First human decision. release-please creates a **draft** GitHub release with the generated changelog and pushes the `v*` tag immediately (`force-tag-creation` — without it, drafts get no tag until publication and release-please loses the previous-release boundary on subsequent runs). The `release-assets` job then runs the test suite, builds the binaries for all platforms, writes checksums, attaches build-provenance attestations, signs everything with cosign (keyless), and uploads the artefacts to the draft. Nothing is published yet.
3. **The draft is published.** Second human decision — via the GitHub UI or `gh release edit <tag> --draft=false`. This is the single ship moment: it makes the release immutable and fires every publish-triggered workflow.
4. **Publish-triggered workflows fire.** On the `release: published` event, Docker images, Helm charts (stamped with the tag version), and versioned docs are built and published by their respective workflows. These workflows do not trigger on the tag push, so nothing ships until you publish the draft.

The draft step exists because releases are immutable: GitHub rejects asset uploads to an already-published release, so the signed artefacts must be attached while the release is still a draft.

**Commits landing on `main` between merge and publish** do not touch the pending draft. They accumulate into the next Release PR. Publishing the draft ships exactly the version that was in it.

## Prerequisites

- **`RELEASE_PLEASE_TOKEN` secret** — a fine-grained PAT or GitHub App token with `contents: write` and `pull-requests: write`. The workflow falls back to `GITHUB_TOKEN`, but PRs opened with `GITHUB_TOKEN` do not trigger CI, so required checks will not run on the Release PR without this token.
- **Repository setting** — Settings → Actions → General → enable *Allow GitHub Actions to create and approve pull requests*, or release-please cannot open the Release PR.

## Configuration

- [`release-please-config.json`](https://github.com/opendefensecloud/solution-arsenal/blob/main/release-please-config.json) — release type, draft mode, changelog sections.
- [`.release-please-manifest.json`](https://github.com/opendefensecloud/solution-arsenal/blob/main/.release-please-manifest.json) — the currently released version; maintained by release-please, do not edit by hand.
