#!/usr/bin/env bash
# Table-driven checks for require-release-branch.sh (local + CI modes).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/release/require-release-branch.sh"
pass=0
fail=0
skip=0

run_case() {
  local name="$1" expect="$2"
  shift 2
  local ec=0
  local out err
  out="$(mktemp)"
  err="$(mktemp)"
  set +e
  "$@" >"$out" 2>"$err"
  ec=$?
  set -e
  if [[ "$ec" -eq "$expect" ]]; then
    pass=$((pass + 1))
    echo "PASS: $name"
  else
    fail=$((fail + 1))
    echo "FAIL: $name (exit=$ec want=$expect)" >&2
    echo "--- stdout ---" >&2
    cat "$out" >&2 || true
    echo "--- stderr ---" >&2
    cat "$err" >&2 || true
  fi
  rm -f "$out" "$err"
}

BRANCH="$(git -C "$ROOT" rev-parse --abbrev-ref HEAD)"
VERSION=""
case "$BRANCH" in
  v[0-9]*|feature/v[0-9]*|release/v[0-9]*)
    VERSION="${BRANCH##*/}"
    VERSION="${VERSION#v}"
    ;;
esac

# --- local ---
if [[ -n "$VERSION" ]]; then
  run_case "local: matching version branch" 0 \
    env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH bash "$SCRIPT" --version "$VERSION"

  run_case "local: version with v prefix" 0 \
    env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH bash "$SCRIPT" --version "v${VERSION}"

  run_case "local: wrong version rejected" 1 \
    env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH bash "$SCRIPT" --version 9.9.9
else
  echo "SKIP: local matching cases (not on a version branch: ${BRANCH})"
  skip=$((skip + 3))
fi

run_case "local: missing version rejected" 1 \
  env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH -u CENTAG_RELEASE_VERSION bash "$SCRIPT"

run_case "local: bypass env" 0 \
  env CENTAG_RELEASE_ALLOW_ANY_BRANCH=1 bash "$SCRIPT" --version 9.9.9

run_case "local: unknown arg" 1 \
  env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH bash "$SCRIPT" --version "${VERSION:-0.0.0}" --nope

# Fixed fixture version for CI name checks (independent of checkout).
FIX_VER="0.2.7"

# --- CI workflow_dispatch ---
run_case "ci dispatch: feature/vX" 0 \
  env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH \
    GITHUB_EVENT_NAME=workflow_dispatch \
    GITHUB_REF_NAME="feature/v${FIX_VER}" \
    bash "$SCRIPT" --ci --version "$FIX_VER"

run_case "ci dispatch: vX" 0 \
  env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH \
    GITHUB_EVENT_NAME=workflow_dispatch \
    GITHUB_REF_NAME="v${FIX_VER}" \
    bash "$SCRIPT" --ci --version "$FIX_VER"

run_case "ci dispatch: release/vX" 0 \
  env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH \
    GITHUB_EVENT_NAME=workflow_dispatch \
    GITHUB_REF_NAME="release/v${FIX_VER}" \
    bash "$SCRIPT" --ci --version "$FIX_VER"

run_case "ci dispatch: main rejected" 1 \
  env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH \
    GITHUB_EVENT_NAME=workflow_dispatch \
    GITHUB_REF_NAME=main \
    bash "$SCRIPT" --ci --version "$FIX_VER"

run_case "ci dispatch: wrong feature version" 1 \
  env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH \
    GITHUB_EVENT_NAME=workflow_dispatch \
    GITHUB_REF_NAME=feature/v0.2.6 \
    bash "$SCRIPT" --ci --version "$FIX_VER"

# --- CI push/tag ---
run_case "ci push: non-tag rejected" 1 \
  env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH \
    GITHUB_EVENT_NAME=push \
    GITHUB_REF_TYPE=branch \
    GITHUB_REF_NAME="feature/v${FIX_VER}" \
    GITHUB_SHA="$(git -C "$ROOT" rev-parse HEAD)" \
    bash "$SCRIPT" --ci --version "$FIX_VER"

run_case "ci push: tag version mismatch" 1 \
  env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH \
    GITHUB_EVENT_NAME=push \
    GITHUB_REF_TYPE=tag \
    GITHUB_REF_NAME=v9.9.9 \
    GITHUB_SHA="$(git -C "$ROOT" rev-parse HEAD)" \
    bash "$SCRIPT" --ci --version "$FIX_VER"

run_case "ci: unsupported event" 1 \
  env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH \
    GITHUB_EVENT_NAME=pull_request \
    bash "$SCRIPT" --ci --version "$FIX_VER"

# Tag ancestry: SHA must be an ancestor of origin/<version-branch>.
if [[ -n "$VERSION" ]] && git -C "$ROOT" rev-parse --verify "origin/${BRANCH}" >/dev/null 2>&1; then
  REMOTE_SHA="$(git -C "$ROOT" rev-parse "origin/${BRANCH}")"
  run_case "ci push: tag on version branch" 0 \
    env -u CENTAG_RELEASE_ALLOW_ANY_BRANCH \
      GITHUB_EVENT_NAME=push \
      GITHUB_REF_TYPE=tag \
      GITHUB_REF_NAME="v${VERSION}" \
      GITHUB_SHA="$REMOTE_SHA" \
      bash "$SCRIPT" --ci --version "$VERSION"
else
  echo "SKIP: ci push tag ancestry (need version branch + origin/${BRANCH:-?})"
  skip=$((skip + 1))
fi

echo
echo "require-release-branch_test: ${pass} passed, ${fail} failed, ${skip} skipped"
[[ "$fail" -eq 0 ]]
