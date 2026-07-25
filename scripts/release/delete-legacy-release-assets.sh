#!/usr/bin/env bash
# Delete legacy GitHub Release assets that should not coexist with cli/desktop naming.
#
# Usage:
#   ./scripts/release/delete-legacy-release-assets.sh --tag v0.3.0
#   ./scripts/release/delete-legacy-release-assets.sh --tag v0.3.0 --repo atoml-ai/centag
#
# Removes:
#   centag-proxyctl-*
#   centag-launcher-tray-*
set -euo pipefail

REPO="${CENTAG_RELEASE_REPO:-atoml-ai/centag}"
TAG=""

fail() { echo "error: $*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag) TAG="${2:-}"; shift 2 ;;
    --repo) REPO="${2:-}"; shift 2 ;;
    -h|--help)
      sed -n '2,12p' "$0"
      exit 0
      ;;
    *) fail "unknown arg: $1" ;;
  esac
done

[[ -n "$TAG" ]] || fail "--tag required"
command -v gh >/dev/null 2>&1 || fail "gh is required"

if ! gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then
  echo "==> no release ${TAG} on ${REPO}; skip legacy asset cleanup" >&2
  exit 0
fi

# List asset names; delete legacy proxyctl (and old personal-only wrap-era names if still present).
while IFS= read -r name; do
  [[ -n "$name" ]] || continue
  case "$name" in
    centag-proxyctl-*|centag-launcher-tray-*)
      echo "==> deleting legacy asset ${name} from ${TAG}" >&2
      gh release delete-asset "$TAG" "$name" --repo "$REPO" --yes
      ;;
  esac
done < <(gh release view "$TAG" --repo "$REPO" --json assets --jq '.assets[].name')

echo "==> legacy asset cleanup done for ${TAG}" >&2
