#!/usr/bin/env bash
# Tags a release once its CHANGELOG section is merged, and pushes the tag
# (which starts .github/workflows/release.yml):
#
#   scripts/tag-release.sh 0.15.1
#   scripts/tag-release.sh -n 0.15.1   # checks only, no tag
#
# It tags origin/main, not your checkout, after checking that:
#   - the version is X.Y.Z and vX.Y.Z is not taken (here or on origin);
#   - origin/main's CHANGELOG.md has a "## [X.Y.Z]" section;
#   - CI passed on that commit (it waits while CI is still running).
# Needs git and gh (logged in).
set -euo pipefail

die() { echo "tag-release: $*" >&2; exit 1; }

dry=
[ "${1:-}" = -n ] && { dry=1; shift; }
[ $# -eq 1 ] || die "usage: scripts/tag-release.sh [-n] X.Y.Z"
version=${1#v}
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || die "not a version: $1"
tag="v$version"

command -v gh >/dev/null || die "gh is not installed"
git fetch --quiet --tags origin main

git rev-parse -q --verify "refs/tags/$tag" >/dev/null && die "$tag already exists"
[ -z "$(git ls-remote --tags origin "refs/tags/$tag")" ] || die "$tag already exists on origin"

sha=$(git rev-parse origin/main)
subject=$(git log -1 --format=%s "$sha")
changelog=$(git show "$sha:CHANGELOG.md")
grep -q "^## \[$version\]" <<<"$changelog" ||
  die "origin/main's CHANGELOG.md has no \"## [$version]\" section: merge the release PR first"
echo "origin/main is ${sha:0:7} $subject"
case $subject in
  "chore: release $version"*) ;;
  *) echo "note: the last commit is not \"chore: release $version\"" ;;
esac

# CI for that commit: wait while it runs, stop unless it passed.
while :; do
  run=$(gh run list --workflow CI --commit "$sha" --limit 1 --json databaseId,status,conclusion \
    --jq '.[0] | "\(.databaseId) \(.status) \(.conclusion)"')
  [ -n "$run" ] || die "no CI run for ${sha:0:7} yet; try again in a minute"
  read -r id status conclusion <<<"$run"
  [ "$status" = completed ] && break
  echo "CI is $status, waiting… (gh run watch $id)"
  sleep 20
done
[ "$conclusion" = success ] || die "CI on ${sha:0:7} ended in $conclusion: gh run view $id"
echo "CI passed"
[ -z "$dry" ] || { echo "-n: would tag ${sha:0:7} as $tag and push it"; exit 0; }

git tag -a "$tag" -m "panal $version" "$sha"
git push origin "$tag"
echo "pushed $tag; the release builds at:"
sleep 5
gh run list --workflow Release --limit 1 --json url --jq '.[0].url' || true
