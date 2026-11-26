# Releasing

Releases are cut with the [Release workflow](https://github.com/opendefensecloud/solution-arsenal/blob/main/.github/workflows/release.yaml), which calls dev-kit's [shared release workflows](https://github.com/opendefensecloud/dev-kit#release-workflow). They compute the version and notes from [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/) messages, tag the commit and create the draft release. This repository only keeps the triggers and the release-asset build. There is no release PR and no manual tagging.

## Version resolution

| Commit type | Version bump |
|---|---|
| `fix:`, `perf:` | patch |
| `feat:` | minor |
| `feat!:` (any type with `!`) | minor while the major is `0` |
| `chore:`, `docs:`, `ci:`, … | none on their own |

Breaking changes bump the minor for as long as the major is `0`; from `1.0.0` onward they bump the major. Reaching `1.0.0` means passing `v1.0.0` as the `version` input once.

To release a specific version, pass it as the `version` input instead of letting the workflow compute one.

A release candidate is cut by passing its version, for example `v0.5.0-rc1`. It cannot be promoted to its final version yet ([dev-kit#101](https://github.com/opendefensecloud/dev-kit/issues/101)): the notes of `v0.5.0` cover only what landed *after* the rc, and on the rc's own commit they are empty, so the run fails before tagging. Until that is solved, cut the final version from a later commit and complete the draft's notes by hand.

## Notes are built from merge commits

Every PR lands on `main` as a merge commit whose subject is the PR title. The notes are built from those merge commits alone; everything on the branch side of a merge is dropped, or every WIP commit inside a PR would appear next to the PR itself.

Two things have to hold, and both are checked. A commit on `main` has to *be* a merge, and its subject has to parse as a conventional commit, which is the PR title. Squash and rebase merges leave no merge commit; GitHub's default `Merge pull request #1 from ...` subject parses as nothing. Either way the commit would be missing from the notes, so the Release workflow refuses to run.

`test:`, `style:` and `build:` merges are dropped from the notes on purpose.

A breaking change is signalled with `!` in the PR title, or in a branch-side commit subject like `feat!:`; `BREAKING CHANGE:` footers are ignored. Emoji prefixes on PR titles are ignored when parsing.

## What happens when

1. **Commits land on `main`.** The Release workflow runs on every push as a dry run. Its job summary previews the next version, the notes and the state of the required checks. It creates no tag and no release: between releases no release object exists.
2. **You run the Release workflow manually** on `main`, with `dry_run` unchecked, once the required checks on that commit have passed. The run tags the commit, builds and signs the artefacts from the tag, and creates the **draft** release with notes and artefacts in one step. The job summary links the draft.
3. **You publish the draft**, after the run has finished. Second human decision, via the GitHub UI or `gh release edit <tag> --draft=false`. This is the single ship moment: it makes the release immutable and fires every publish-triggered workflow.
4. **Publish-triggered workflows fire.** On `release: published`, Docker images, Helm charts (stamped with the tag version) and versioned docs are built and published by their respective workflows. They do not trigger on the tag push, so nothing ships until you publish the draft.

The draft step exists because releases are immutable: GitHub rejects asset uploads to an already-published release, so the signed artefacts must be attached while the release is still a draft. For the same reason, do not publish while the run is still going: artefacts missing at that moment can no longer be added.

## Required checks

A commit is only tagged if these checks passed on it:

- `test` (which needs `lint`)
- `test-e2e / test-e2e`
- `test-e2e / test-ui-e2e`
- `test-e2e / chaining-test-e2e`

A check that failed, is still running, was skipped or never ran blocks the release; the push preview only reports it. If a check ran several times on the commit, every run has to have passed. That includes the e2e run that `release: published` starts on the released commit.

These checks therefore run on every push to `main`, documentation-only commits included. Do not add `paths-ignore` to the push trigger of `docker.yaml` or `golang.yaml`, and do not require a job that a condition can skip.

## Tags are final

The `protect-release-tags` ruleset refuses moving and deleting a release tag, and the workflow refuses to tag without it. The tag is created before anything is built, so a run can leave a tag that is never released: the wrong commit was dispatched, or the build fails and is not re-run.

Such a tag is burned. It stays, nothing shipped from it. Delete its draft, if there is one, and run the release again: the next version skips the burned number and the notes still start at the last published release. Do not pass a burned number as `version`; it is refused.

## One release at a time

A manual run fails while any draft release exists. Publish the draft to ship it, or delete it to abandon it, which burns its tag.

Commits that land on `main` after the draft was cut are not part of it. They go into the next release.

## When something lands that the notes cannot use

If a commit reaches `main` without being a merge, or as a merge whose subject is not a conventional commit, it cannot appear in the notes and the Release workflow fails.

Fixing the history is not an option on `main`, so run **Release** with `force` set. It logs the affected commits as a warning and releases without them, rather than leaving you deadlocked. The commits are still in the repository; only the generated notes miss them, so mention them by hand in the draft before publishing.

`force` only affects this check. It does not skip the default-branch check, the required checks, the pending-draft check, or the requirement that something releasable landed.

## Recovering a failed run

If the asset build or the draft job fails, use **Re-run failed jobs**. The tag already exists, so the rebuild uses the same commit, and the draft job creates or updates the same draft.

**Re-run all jobs** cannot recover a release: the version computation then finds its own tag. In that case, and for a run older than 30 days, the tag is burned: delete the draft and release again.

## Prerequisites

No credentials beyond the default `GITHUB_TOKEN`. The workflow never writes to `main`, so it needs no GitHub App, no PAT, and no *Allow GitHub Actions to create and approve pull requests* setting.

These repository settings are load-bearing, though:

- **The `protect-release-tags` ruleset and immutable releases.** A repository admin sets both with `make repo-settings`.
- **Merge commits enabled**, and squash and rebase merging disabled. The notes are built from merge commits.
- **Merge commit title taken from the PR title**, under Settings → General. Either *Pull request title* or *Pull request title and description* works; only *Default message* breaks it, because `Merge pull request #1 from ...` parses as nothing. The notes are built from that subject.
- **The PR title check kept as a required status check** (`.github/workflows/conventional-commits.yml`). It is what guarantees every merge subject is a conventional commit.

Releases are only ever cut from the default branch; the workflow refuses to run anywhere else.
