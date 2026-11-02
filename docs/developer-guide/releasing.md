# Releasing

Releases are cut with the [Release workflow](https://github.com/opendefensecloud/solution-arsenal/blob/main/.github/workflows/release.yaml), which calls dev-kit's [shared release workflow](https://github.com/opendefensecloud/dev-kit/blob/main/.github/workflows/release.yml) to compute the version and notes from [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/) messages. This repository only keeps the triggers and the release-asset build. There is no release PR and no manual tagging.

## Version resolution

| Commit type | Version bump |
|---|---|
| `fix:`, `perf:` | patch |
| `feat:` | minor |
| `feat!:` (any type with `!`) | minor while the major is `0` |
| `chore:`, `docs:`, `ci:`, … | none on their own |

Breaking changes bump the minor for as long as the major is `0`; from `1.0.0` onward they bump the major. Reaching `1.0.0` means passing `v1.0.0` as the `version` input once.

To release a specific version, pass it as the `version` input instead of letting the workflow compute one.

Prereleases work but are a one-way door for the notes. Once `v0.4.0-rc1` is tagged, the auto-computed version becomes `v0.4.0-rc1.1` rather than returning to `v0.4.0`, and the final `v0.4.0` notes cover only what landed *after* rc1. Cutting an rc means passing every subsequent version by hand and editing the final draft.

## Notes are built from merge commits

Every PR lands on `main` as a merge commit whose subject is the PR title. The notes are built from those merge commits alone; everything on the branch side of a merge is dropped, or every WIP commit inside a PR would appear next to the PR itself.

Two things have to hold, and both are checked. A commit on `main` has to *be* a merge, and its subject has to parse as a conventional commit, which is the PR title. Squash and rebase merges leave no merge commit; GitHub's default `Merge pull request #1 from ...` subject parses as nothing. Either way the commit would be missing from the notes, so the Release workflow refuses to run.

`test:`, `style:` and `build:` merges are dropped from the notes on purpose.

A breaking change is signalled with `!` in the PR title, or in a branch-side commit subject like `feat!:`; `BREAKING CHANGE:` footers are ignored. Emoji prefixes on PR titles are ignored when parsing.

## What happens when

1. **Commits land on `main`.** The Release workflow runs on every push and creates or updates the pending **draft** release in place, including its signed artefacts. It never tags or publishes. If nothing releasable landed, it leaves the draft untouched.
2. **You run the Release workflow manually.** Leave `dry_run` checked to see the computed version and rendered notes without creating anything. Re-run with it unchecked to refresh the draft and its assets. `create_tag` is off by default, so the tag is created when you publish the draft; check it if you need the tag during the run.
3. **You publish the draft.** Second human decision, via the GitHub UI or `gh release edit <tag> --draft=false`. This is the single ship moment: it creates the tag if it does not exist yet, makes the release immutable and fires every publish-triggered workflow.
4. **Publish-triggered workflows fire.** On `release: published`, Docker images, Helm charts (stamped with the tag version) and versioned docs are built and published by their respective workflows. They do not trigger on the tag push, so nothing ships until you publish the draft.

The draft step exists because releases are immutable: GitHub rejects asset uploads to an already-published release, so the signed artefacts must be attached while the release is still a draft.

The draft is written before the tag, so a failed tag leaves an updateable draft and no tag, and re-running retries. The workflow never deletes or moves a tag. Where a `protect-release-tags` ruleset is active, a tag cannot be moved or deleted; before a tag exists, abandon a draft by deleting it. Once a tag exists (for example after a run with `create_tag`), publish that draft or cut the next patch: deleting the draft leaves the tag behind and burns the version.

**Commits landing on `main` after the draft is cut** update the same pending draft on the next push. They do not create a second draft. Publishing the draft ships exactly the version that was in it.

## When something lands that the notes cannot use

If a commit reaches `main` without being a merge, or as a merge whose subject is not a conventional commit, it cannot appear in the notes and the Release workflow fails.

Fixing the history is not an option on `main`, so run **Release** with `force` set. It logs the affected commits as a warning and releases without them, rather than leaving you deadlocked. The commits are still in the repository; only the generated notes miss them, so mention them by hand in the draft before publishing.

`force` only affects this check. It does not skip the default-branch check, the pending-draft check, or the requirement that something releasable landed.

## One pending draft at a time

The workflow keeps a single pending draft. A push to `main` updates it in place (title, notes, prerelease flag and target commit); if the computed version moves, the draft is renamed to the new version. There is one pending draft, not one per push.

A draft whose tag already exists is blocked: publish it to ship, or delete the draft to abandon it. Where release tags are protected, once a tag exists (for example after a run with `create_tag`) it cannot be deleted, so publish that draft or cut the next patch. Deleting the draft leaves the tag behind and burns the version.

If something landed that you want included in the pending draft, the next push or a manual run picks it up. Otherwise publish the draft and let the new commits go into the next one.

## Recovering a failed asset upload

If the draft was created but the artefacts failed to upload, run **Release** again with `retry_assets_for` set to the draft's tag, and uncheck `dry_run`. There is no dry run on the recovery path (it would rebuild and overwrite the draft's assets under a label that promises a no-op), so the shared workflow refuses the combination. The retry skips version resolution and the notes entirely and only rebuilds and re-attaches the artefacts. It refuses if there is no draft release, or if the release is already published. Uploads use `--clobber`, so re-runs are idempotent and there is no need to delete the tag and the release.

Leave `retry_assets_for` empty for every normal release.

## Prerequisites

No credentials beyond the default `GITHUB_TOKEN`. The workflow never writes to `main`, so it needs no GitHub App, no PAT, and no *Allow GitHub Actions to create and approve pull requests* setting.

Three repository settings are load-bearing, though:

- **Merge commits enabled**, and squash and rebase merging disabled. The notes are built from merge commits.
- **Merge commit title taken from the PR title**, under Settings → General. Either *Pull request title* or *Pull request title and description* works; only *Default message* breaks it, because `Merge pull request #1 from ...` parses as nothing. The notes are built from that subject.
- **The PR title check kept as a required status check** (`.github/workflows/conventional-commits.yml`). It is what guarantees every merge subject is a conventional commit.

Releases are only ever cut from the default branch; the workflow refuses to run anywhere else.
