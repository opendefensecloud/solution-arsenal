# Releasing

Releases are automated with [release-please](https://github.com/googleapis/release-please), driven entirely by [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/) messages on `main`. There are no release labels and no manual tagging.

## Version resolution

| Commit type | Version bump |
|---|---|
| `fix:` | patch |
| `feat:` | minor |
| `feat!:` / `BREAKING CHANGE:` footer | major |
| `chore:`, `docs:`, `ci:`, … | none |

To force a specific version (e.g. after a release candidate), add a `Release-As: x.y.z` footer to a commit on `main`.

## What happens when

1. **Commits land on `main`.** On every push, the [Release Please workflow](https://github.com/opendefensecloud/solution-arsenal/blob/main/.github/workflows/release-please.yaml) scans commits since the last release. If at least one releasable commit (`feat`/`fix`/breaking) exists, it opens or updates a **Release PR** that bumps the version and updates `CHANGELOG.md`. `chore`-only batches never produce a release.
2. **The Release PR is merged.** This is the first human decision. release-please creates a **draft** GitHub release with the generated changelog. Draft releases do not create a git tag yet. The `release-assets` job then runs the test suite, builds the binaries for all platforms, writes checksums, attaches build-provenance attestations, signs everything with cosign (keyless), and uploads the artefacts to the draft.
3. **The draft is published.** This is the second human decision — via the GitHub UI or `gh release edit <tag> --draft=false`. Publishing creates the `v*` tag and makes the release immutable.
4. **Tag/publish-triggered workflows fire.** Docker images, Helm charts (stamped with the tag version), and versioned docs are built and published by their respective workflows.

The draft step exists because releases are immutable: GitHub rejects asset uploads to an already-published release, so the signed artefacts must be attached while the release is still a draft.

## Configuration

- [`release-please-config.json`](https://github.com/opendefensecloud/solution-arsenal/blob/main/release-please-config.json) — release type, draft mode, changelog sections.
- [`.release-please-manifest.json`](https://github.com/opendefensecloud/solution-arsenal/blob/main/.release-please-manifest.json) — the currently released version; maintained by release-please, do not edit by hand.
