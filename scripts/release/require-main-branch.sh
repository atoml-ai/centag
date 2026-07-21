#!/usr/bin/env bash
# Gate: GitHub Release publish is allowed only from main.
#
# Usage:
#   bash scripts/release/require-main-branch.sh           # local git checkout
#   bash scripts/release/require-main-branch.sh --ci      # GitHub Actions
#
# Escape hatch (emergency only):
#   CENTAG_RELEASE_ALLOW_NON_MAIN=1
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

MODE="local"
[[ "${1:-}" == "--ci" ]] && MODE="ci"

if [[ "${CENTAG_RELEASE_ALLOW_NON_MAIN:-}" == "1" ]]; then
  echo "warn: CENTAG_RELEASE_ALLOW_NON_MAIN=1 — skipping main-branch gate" >&2
  exit 0
fi

fail() {
  echo "error: $*" >&2
  echo "hint: merge to main first, then release. Override (emergency): CENTAG_RELEASE_ALLOW_NON_MAIN=1" >&2
  exit 1
}

if [[ "$MODE" == "ci" ]]; then
  EVENT="${GITHUB_EVENT_NAME:-}"
  REF="${GITHUB_REF:-}"
  SHA="${GITHUB_SHA:-}"

  case "$EVENT" in
    workflow_dispatch)
      [[ "$REF" == "refs/heads/main" ]] || fail "workflow_dispatch release only from main (got ${REF})"
      echo "OK: CI workflow_dispatch on main"
      ;;
    push)
      # Tag push: annotated/lightweight tag must point to a commit reachable from origin/main
      [[ "${GITHUB_REF_TYPE:-}" == "tag" ]] || fail "unexpected push ref type (want tag)"
      git fetch --no-tags origin main 2>/dev/null || git fetch origin main
      git merge-base --is-ancestor "$SHA" "origin/main" \
        || fail "tag ${GITHUB_REF_NAME:-} (${SHA:0:12}) is not on origin/main — only tags from main may release"
      echo "OK: CI tag ${GITHUB_REF_NAME:-} is on origin/main"
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
[[ "$BRANCH" != "HEAD" ]] || fail "detached HEAD — checkout main to release"
[[ "$BRANCH" == "main" ]] || fail "local release upload only from main (current branch: ${BRANCH})"

# Prefer that local main tracks / contains the same tip intent as origin/main when available
if git rev-parse --verify origin/main >/dev/null 2>&1; then
  LOCAL_SHA="$(git rev-parse HEAD)"
  REMOTE_SHA="$(git rev-parse origin/main)"
  if [[ "$LOCAL_SHA" != "$REMOTE_SHA" ]]; then
    # Allow local main ahead (about to push) or warn if behind/diverged
    if git merge-base --is-ancestor "$REMOTE_SHA" "$LOCAL_SHA"; then
      echo "warn: local main is ahead of origin/main — push before publishing if CI/install.sh must match" >&2
    elif git merge-base --is-ancestor "$LOCAL_SHA" "$REMOTE_SHA"; then
      fail "local main is behind origin/main — git pull (or reset) then release"
    else
      fail "local main has diverged from origin/main — reconcile before release"
    fi
  fi
fi

echo "OK: local branch is main"
