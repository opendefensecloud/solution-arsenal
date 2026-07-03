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
2. **The Release PR is merged.** This is the first human decision. release-please creates a **draft** GitHub release with the generated changelog and pushes the `v*` tag immediately (`force-tag-creation` — without it, drafts get no tag until publication and release-please loses the previous-release boundary on subsequent runs). The `release-assets` job then runs the test suite, builds the binaries for all platforms, writes checksums, attaches build-provenance attestations, signs everything with cosign (keyless), and uploads the artefacts to the draft.
3. **The draft is published.** This is the second human decision — via the GitHub UI or `gh release edit <tag> --draft=false`. Publishing makes the release immutable.
4. **Publish-triggered workflows fire.** Docker images, Helm charts (stamped with the tag version), and versioned docs are built and published by their respective workflows on the `release: published` event.

The draft step exists because releases are immutable: GitHub rejects asset uploads to an already-published release, so the signed artefacts must be attached while the release is still a draft.

Note that the docker and Helm workflows also listen on `v*` tag pushes. Tags pushed with the default `GITHUB_TOKEN` do not trigger workflows, but if a `RELEASE_PLEASE_TOKEN` PAT is configured, images and charts for the new version are already published at draft time — only the GitHub release itself waits for the publish decision.

## Configuration

- [`release-please-config.json`](https://github.com/opendefensecloud/solution-arsenal/blob/main/release-please-config.json) — release type, draft mode, changelog sections.
- [`.release-please-manifest.json`](https://github.com/opendefensecloud/solution-arsenal/blob/main/.release-please-manifest.json) — the currently released version; maintained by release-please, do not edit by hand.
