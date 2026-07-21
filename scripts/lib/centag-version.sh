#!/usr/bin/env bash
# Resolve Centag product version for ldflags / banners.
# Priority: version branch (feature/v0.2.7 → v0.2.7) → git tag → v0.0.0-dev
#
# Usage:
#   source scripts/lib/centag-version.sh && centag_resolve_version
#   bash scripts/lib/centag-version.sh          # print version to stdout

centag_resolve_version() {
  local branch="" ver=""
  if command -v git >/dev/null 2>&1; then
    branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
    case "$branch" in
      feature/v*|release/v*|hotfix/v*|v[0-9]*)
        ver="${branch##*/}"
        ;;
      feature/*|release/*|hotfix/*)
        ver="v${branch##*/}"
        ;;
    esac
    if [[ -z "$ver" ]]; then
      ver="$(git describe --tags --abbrev=0 2>/dev/null || true)"
    fi
  fi
  if [[ -z "$ver" ]]; then
    ver="v0.0.0-dev"
  fi
  # ensure leading v for numeric releases
  if [[ "$ver" =~ ^[0-9] ]]; then
    ver="v${ver}"
  fi
  printf '%s\n' "$ver"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  centag_resolve_version
fi
