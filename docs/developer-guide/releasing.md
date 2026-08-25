# Releasing

Releases are cut with the [Release workflow](https://github.com/opendefensecloud/solution-arsenal/blob/main/.github/workflows/release.yaml), which generates notes from [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/) messages using [git-cliff](https://git-cliff.org). There is no release PR and no manual tagging.

## Version resolution

| Commit type | Version bump |
|---|---|
| `fix:`, `perf:` | patch |
| `feat:` | minor |
| `feat!:` / `BREAKING CHANGE:` footer | minor while the major is `0` |
| `chore:`, `docs:`, `ci:`, … | none on their own |

Breaking changes bump the minor for as long as the major is `0`. This is git-cliff's default behaviour and needs no configuration: from `1.0.0` onward a breaking change bumps the major on its own. Reaching `1.0.0` means passing `v1.0.0` as the `version` input once. Nothing in `cliff.toml` has to change.

To release a specific version, pass it as the `version` input instead of letting git-cliff compute one.

Prereleases work but are a one-way door for the notes. Once `v0.4.0-rc1` is tagged, the auto-computed version becomes `v0.4.0-rc1.1` rather than returning to `v0.4.0`, and the final `v0.4.0` notes cover only what landed *after* rc1. Cutting an rc means passing every subsequent version by hand and editing the final draft.

## Notes are built from merge commits

Every PR lands on `main` as a merge commit whose subject is the PR title. The notes are built from those merge commits alone; everything on the branch side of a merge is dropped, or every WIP commit inside a PR would appear next to the PR itself.

Two things have to hold, and both are checked. A commit on `main` has to *be* a merge, and its subject has to parse as a conventional commit, which is the PR title. Squash and rebase merges leave no merge commit; GitHub's default `Merge pull request #1 from ...` subject parses as nothing. Either way the commit would be missing from the notes, so the Changelog Preview warns when one lands and the Release workflow refuses to run.

`test:`, `style:` and `build:` merges are dropped from the notes on purpose.

## What happens when

1. **Commits land on `main`.** The [Changelog Preview workflow](https://github.com/opendefensecloud/solution-arsenal/blob/main/.github/workflows/changelog-preview.yaml) renders the pending version and its notes into the run's job summary. Nothing is tagged, committed or built.
2. **You run the Release workflow.** First human decision. Leave `dry_run` checked to see the computed version and the rendered notes without creating anything. Re-run with it unchecked to push the tag and create a **draft** release, after which the `assets` job runs the test suite, builds the binaries for all platforms, writes checksums, attaches build-provenance attestations, signs everything with cosign (keyless), and uploads the artefacts to the draft. Nothing is published yet.
3. **You publish the draft.** Second human decision, via the GitHub UI or `gh release edit <tag> --draft=false`. This is the single ship moment: it makes the release immutable and fires every publish-triggered workflow.
4. **Publish-triggered workflows fire.** On `release: published`, Docker images, Helm charts (stamped with the tag version) and versioned docs are built and published by their respective workflows. They do not trigger on the tag push, so nothing ships until you publish the draft.

The draft step exists because releases are immutable: GitHub rejects asset uploads to an already-published release, so the signed artefacts must be attached while the release is still a draft.

The tag is created explicitly before the draft, because a draft release does not create its own tag until it is published. Tags are not covered by the `protect-main` ruleset, which targets branches only.

**Commits landing on `main` between the tag and publication** do not touch the pending draft. They accumulate into the next release. Publishing the draft ships exactly the version that was in it.

## One pending draft at a time

Re-running **Release** while a draft is pending is refused. It does not roll the existing draft forward, and it cannot: the tag pins the changelog boundary, so a second run would cut a second version, and the artefacts already attached are signed and attested against the commit the draft was cut from.

If something landed that you want included, delete the draft and its tag, then run **Release** again. It will pick up everything since the last published release. Otherwise publish the draft and let the new commits go into the next one.

## Recovering a failed asset upload

If the draft was created but the artefacts failed to upload, run **Release** again with `retry_assets_for` set to the draft's tag. That skips version resolution and the notes entirely and only rebuilds and re-attaches the artefacts. It refuses if the tag has no release, or if the release is already published. Uploads use `--clobber`, so re-runs are idempotent and there is no need to delete the tag and the release.

Leave `retry_assets_for` empty for every normal release.

## Prerequisites

No credentials beyond the default `GITHUB_TOKEN`. The workflow never writes to `main`, so it needs no GitHub App, no PAT, and no *Allow GitHub Actions to create and approve pull requests* setting.

Three repository settings are load-bearing, though:

- **Merge commits enabled**, and squash and rebase merging disabled. The notes are built from merge commits.
- **Default merge commit message set to *Pull request title***, under Settings → General. The message is what the notes are made of.
- **The PR title check kept as a required status check** (`.github/workflows/conventional-commits.yml`). It is what guarantees every merge subject is a conventional commit.

Releases are only ever cut from the default branch; the workflow refuses to run anywhere else.

## Configuration

- [`cliff.toml`](https://github.com/opendefensecloud/solution-arsenal/blob/main/cliff.toml) — commit grouping, the merge-commit filter and the bump rules.
- [`CHANGELOG.md`](https://github.com/opendefensecloud/solution-arsenal/blob/main/CHANGELOG.md) — frozen at `v0.3.0`. From `v0.4.0` on, the notes live on the [Releases page](https://github.com/opendefensecloud/solution-arsenal/releases).
