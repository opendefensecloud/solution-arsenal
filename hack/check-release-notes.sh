#!/usr/bin/env bash
#
# Lists the commits in a range that will not appear in the generated release
# notes. Prints nothing and exits 0 when the range is fully representable.
#
#   hack/check-release-notes.sh v0.3.0..HEAD
#
# A commit reaches the notes only if both of these hold:
#
#   1. it is a merge commit, because cliff.toml filters on merge_commit
#   2. its subject parses as a conventional commit, which is what picks a group
#
# Squash and rebase merges fail the first. GitHub's default
# "Merge pull request #1 from ..." subject and the Revert button's default
# title fail the second.

set -euo pipefail

RANGE="${1:?usage: $0 <git range>, e.g. v0.3.0..HEAD}"

# The types cliff.toml assigns a group to. Anything else has no section to go
# into and is dropped.
CONVENTIONAL_TYPES='feat|fix|perf|refactor|revert|docs|ci|chore|test|style|build'

# git-cliff strips these before parsing, so they have to go here too, otherwise
# a subject like "✨ feat: thing" looks unparseable when git-cliff handles it
# fine. Keep in sync with commit_preprocessors in cliff.toml.
EMOJI_RANGES='\x{1F000}-\x{1FAFF}\x{2600}-\x{27BF}\x{2B00}-\x{2BFF}\x{FE0E}\x{FE0F}\x{200D}'

normalise_subjects() {
  perl -CSD -pe "s/[${EMOJI_RANGES}]//g; s/[ \\t]{2,}/ /g"
}

commits_that_are_not_merges() {
  git log --first-parent --no-merges "$RANGE" --format='%h %s'
}

merges_without_a_conventional_subject() {
  git log --first-parent --merges "$RANGE" --format='%h %s' \
    | normalise_subjects \
    | grep -vE "^[0-9a-f]+ (${CONVENTIONAL_TYPES})(\(.+\))?!?:" || true
}

{
  commits_that_are_not_merges
  merges_without_a_conventional_subject
} | sed '/^$/d'
