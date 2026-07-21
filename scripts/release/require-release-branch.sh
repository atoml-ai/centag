#!/usr/bin/env bash
# Gate: GitHub Release publish must run from the version branch for that release.
#
# Allowed local/CI branch names for version 0.2.7 (no leading v in VERSION):
#   v0.2.7 | feature/v0.2.7 | release/v0.2.7
#
# Usage:
#   bash scripts/release/require-release-branch.sh --version 0.2.7
#   bash scripts/release/require-release-branch.sh --ci --version 0.2.7
#
# Env:
#   CENTAG_RELEASE_VERSION          used if --version omitted
#   CENTAG_RELEASE_ALLOW_ANY_BRANCH=1   emergency bypass
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

MODE="local"
VERSION="${CENTAG_RELEASE_VERSION:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ci) MODE="ci"; shift ;;
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    -h|--help)
      sed -n '2,20p' "$0"
      exit 0
      ;;
    *)
      echo "error: unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

VERSION="${VERSION#v}"

if [[ "${CENTAG_RELEASE_ALLOW_ANY_BRANCH:-}" == "1" ]]; then
  echo "warn: CENTAG_RELEASE_ALLOW_ANY_BRANCH=1 — skipping version-branch gate" >&2
  exit 0
fi

fail() {
  echo "error: $*" >&2
  echo "hint: checkout the version branch (v${VERSION:-X} | feature/v${VERSION:-X} | release/v${VERSION:-X}), then release." >&2
  echo "      emergency: CENTAG_RELEASE_ALLOW_ANY_BRANCH=1" >&2
  exit 1
}

[[ -n "$VERSION" ]] || fail "--version / CENTAG_RELEASE_VERSION required (e.g. 0.2.7)"

allowed_branches() {
  echo "v${VERSION}"
  echo "feature/v${VERSION}"
  echo "release/v${VERSION}"
}

branch_is_allowed() {
  local b="$1" a
  for a in $(allowed_branches); do
    [[ "$b" == "$a" ]] && return 0
  done
  return 1
}

remote_version_refs() {
  allowed_branches | while read -r b; do
    echo "origin/${b}"
  done
}

commit_on_version_branch() {
  local sha="$1" ref
  for ref in $(remote_version_refs); do
    if git rev-parse --verify "$ref" >/dev/null 2>&1; then
      if git merge-base --is-ancestor "$sha" "$ref"; then
        echo "$ref"
        return 0
      fi
    fi
  done
  return 1
}

if [[ "$MODE" == "ci" ]]; then
  EVENT="${GITHUB_EVENT_NAME:-}"
  REF="${GITHUB_REF:-}"
  REF_NAME="${GITHUB_REF_NAME:-}"
  SHA="${GITHUB_SHA:-}"

  case "$EVENT" in
    workflow_dispatch)
      branch_is_allowed "$REF_NAME" \
        || fail "workflow_dispatch release only from version branch for v${VERSION} (got ${REF_NAME}; want $(allowed_branches | tr '\n' ' '))"
      echo "OK: CI workflow_dispatch on ${REF_NAME} (v${VERSION})"
      ;;
    push)
      [[ "${GITHUB_REF_TYPE:-}" == "tag" ]] || fail "unexpected push ref type (want tag)"
      tag_ver="${GITHUB_REF_NAME#v}"
      [[ "$tag_ver" == "$VERSION" ]] \
        || fail "CI tag ${GITHUB_REF_NAME} does not match --version ${VERSION}"
      # Ensure remotes for allowed branches are available
      for b in $(allowed_branches); do
        git fetch --no-tags origin "$b" 2>/dev/null || true
      done
      hit="$(commit_on_version_branch "$SHA" || true)"
      [[ -n "$hit" ]] \
        || fail "tag ${GITHUB_REF_NAME} (${SHA:0:12}) is not on a version branch for v${VERSION} ($(allowed_branches | tr '\n' ' '))"
      echo "OK: CI tag ${GITHUB_REF_NAME} is on ${hit}"
      ;;
    *)
      fail "unsupported CI event for release: ${EVENT:-<empty>}"
      ;;
  esac
  exit 0
fi

# --- local ---
BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
[[ -n "$BRANCH" ]] || fail "not a git repository"
[[ "$BRANCH" != "HEAD" ]] || fail "detached HEAD — checkout v${VERSION} / feature/v${VERSION} to release"
branch_is_allowed "$BRANCH" \
  || fail "local release upload only from version branch for v${VERSION} (current: ${BRANCH}; want $(allowed_branches | tr '\n' ' '))"

REMOTE="origin/${BRANCH}"
if git rev-parse --verify "$REMOTE" >/dev/null 2>&1; then
  LOCAL_SHA="$(git rev-parse HEAD)"
  REMOTE_SHA="$(git rev-parse "$REMOTE")"
  if [[ "$LOCAL_SHA" != "$REMOTE_SHA" ]]; then
    if git merge-base --is-ancestor "$REMOTE_SHA" "$LOCAL_SHA"; then
      echo "warn: local ${BRANCH} is ahead of ${REMOTE} — push before publishing if install.sh must match" >&2
    elif git merge-base --is-ancestor "$LOCAL_SHA" "$REMOTE_SHA"; then
      fail "local ${BRANCH} is behind ${REMOTE} — git pull then release"
    else
      fail "local ${BRANCH} has diverged from ${REMOTE} — reconcile before release"
    fi
  fi
fi

echo "OK: local branch ${BRANCH} matches release v${VERSION}"
