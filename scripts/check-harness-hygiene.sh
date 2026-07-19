#!/usr/bin/env bash
# Lightweight harness hygiene for Centag layout (web/, scripts/, bin/ as build output).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail=0

require_file() {
  if [[ ! -f "$1" ]]; then
    echo "MISSING: $1"
    fail=1
  fi
}

require_file "docs/harness/AGENTS.md"
require_file "docs/harness/ARCHITECTURE.md"
require_file ".golangci.yml"
require_file "docs/versions/README.md"

# Forbidden legacy / non-product trees at repo root
for d in tooling var webui desktop; do
  if [[ -e "$d" ]]; then
    echo "UNEXPECTED PATH: $d (Centag uses web/, scripts/, bin/ for build output)"
    fail=1
  fi
done

# apps/ is reserved for optional client shells (must stay decoupled from core).
# Allowed: launcher (L1 menu + browser), proxyctl (OS PAC/CA helper for system egress).
if [[ -d apps ]]; then
  shopt -s nullglob
  for child in apps/*; do
    base="$(basename "$child")"
    case "$base" in
      launcher|proxyctl) ;;
      *)
        echo "UNEXPECTED PATH: $child (only apps/launcher and apps/proxyctl are allowed under apps/)"
        fail=1
        ;;
    esac
  done
  shopt -u nullglob
fi

# go list must not include packages under node_modules / web/dist
# Prefer the filtered list used by CI when available.
if command -v go >/dev/null 2>&1; then
  if [[ -x scripts/ci-go-packages.sh ]]; then
    list="$(bash scripts/ci-go-packages.sh 2>/dev/null || true)"
  else
    list="$(go list ./... 2>/dev/null | grep -Ev 'node_modules|/vendor/' || true)"
  fi
  bad="$(printf '%s\n' "$list" | grep -E 'node_modules|web/dist' || true)"
  if [[ -n "$bad" ]]; then
    echo "UNEXPECTED go packages:"
    echo "$bad"
    fail=1
  fi
fi

if [[ "$fail" -ne 0 ]]; then
  echo "harness-check FAILED"
  exit 1
fi

echo "harness-check OK"
