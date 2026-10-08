#!/usr/bin/env bash
# Verify that every dev-kit reference in this repository resolves to the same
# commit. Run as part of lint-no-golangci.
#
# This repository pins dev-kit four ways, and only two of them are immutable:
#
#   .github/workflows/*   @<sha> # <tag>   the commit, with the tag as a comment
#   Makefile              <tag>            DEV_KIT_VERSION, fetches common.mk
#   flake.nix             <tag>            the flake input
#   flake.lock            <sha>            what the tag resolved to when locked
#
# GitHub tags are mutable. flake.lock turns the tag into a commit, but the
# conversion happens whenever the lock is refreshed — including by the
# renovate-dev-kit-lock automation, which runs unattended. A tag moved between a
# release and a relock would be adopted silently, and dev-kit's flake *is* the
# shell CI runs in, so that is arbitrary code execution in every job.
#
# The workflow pins are the defence: Renovate writes the commit it resolved, and
# a commit cannot move. Comparing the lock against them turns a moved tag into a
# failed check instead of a lockfile diff nobody reads.
#
# This detects, it does not prevent. The prevention is immutable releases on
# dev-kit, which would make the tag itself trustworthy.

set -euo pipefail

workflows=".github/workflows"
makefile="Makefile"
flake="flake.nix"
lock="flake.lock"

fail() {
	echo "error: $*" >&2
	exit 1
}

# Every opendefensecloud/dev-kit@<sha> in the workflows, with its # <tag> comment.
# Both the composite actions and the reusable-workflow stubs match.
mapfile -t pins < <(
	grep -rhoE "opendefensecloud/dev-kit/[^@]+@[0-9a-f]{40} # v[0-9]+\.[0-9]+\.[0-9]+" "$workflows" |
		grep -oE "[0-9a-f]{40} # v[0-9]+\.[0-9]+\.[0-9]+" | sort -u
)
[ "${#pins[@]}" -gt 0 ] || fail "no SHA-pinned dev-kit reference found in $workflows/ — has the pin format changed?"
if [ "${#pins[@]}" -gt 1 ]; then
	printf 'error: workflows disagree on which dev-kit commit to use:\n' >&2
	printf '  %s\n' "${pins[@]}" >&2
	exit 1
fi

sha="${pins[0]%% *}"
tag="${pins[0]##*# }"

# DEV_KIT_VERSION := vX.Y.Z
mk_tag="$(sed -nE 's/^[[:space:]]*DEV_KIT_VERSION[[:space:]]*:=[[:space:]]*(v[0-9]+\.[0-9]+\.[0-9]+)[[:space:]]*$/\1/p' "$makefile")"
[ -n "$mk_tag" ] || fail "cannot read DEV_KIT_VERSION from $makefile"
[ "$mk_tag" = "$tag" ] ||
	fail "$makefile pins dev-kit $mk_tag but the workflows pin $tag"

# url = "github:opendefensecloud/dev-kit/vX.Y.Z";
flake_tag="$(sed -nE 's#.*github:opendefensecloud/dev-kit/(v[0-9]+\.[0-9]+\.[0-9]+).*#\1#p' "$flake")"
[ -n "$flake_tag" ] || fail "cannot read the dev-kit input tag from $flake"
[ "$flake_tag" = "$tag" ] ||
	fail "$flake pins dev-kit $flake_tag but the workflows pin $tag"

# The resolved commit in the lock, which is the value an unattended relock writes.
lock_sha="$(jq -r '.nodes["dev-kit"].locked.rev // empty' "$lock")"
[ -n "$lock_sha" ] || fail "cannot read the locked dev-kit rev from $lock"
if [ "$lock_sha" != "$sha" ]; then
	cat >&2 <<-EOF
		error: $lock resolved dev-kit $tag to a different commit than the workflows pin.
		  $lock:    $lock_sha
		  workflows: $sha
		Either the lock is stale — run \`nix flake update dev-kit\` — or the $tag tag
		moved after the workflow pins were written, which needs investigating before
		this is merged: the flake input is the shell CI runs in.
	EOF
	exit 1
fi

echo "dev-kit pins agree: $tag at $sha"
